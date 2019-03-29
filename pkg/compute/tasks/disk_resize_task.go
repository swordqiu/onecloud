// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tasks

import (
	"context"
	"fmt"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"

	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/util/logclient"
)

type DiskResizeTask struct {
	SDiskBaseTask
}

func init() {
	taskman.RegisterTask(DiskResizeTask{})
}

func (self *DiskResizeTask) OnInit(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	disk := obj.(*models.SDisk)

	guestId, _ := self.Params.GetString("guest_id")
	var masterGuest *models.SGuest
	if len(guestId) > 0 {
		masterGuest = models.GuestManager.FetchGuestById(guestId)
	}

	storage := disk.GetStorage()
	host := storage.GetMasterHost()

	if masterGuest != nil {
		host = masterGuest.GetHost()
	}

	reason := "Cannot find host for disk"
	if host == nil || host.HostStatus != models.HOST_ONLINE {
		disk.SetDiskReady(ctx, self.GetUserCred(), reason)
		self.SetStageFailed(ctx, reason)
		db.OpsLog.LogEvent(disk, db.ACT_RESIZE_FAIL, reason, self.GetUserCred())
		logclient.AddActionLogWithStartable(self, disk, logclient.ACT_RESIZE, reason, self.UserCred, false)
		return
	}

	disk.SetStatus(self.GetUserCred(), models.DISK_START_RESIZE, "")
	if masterGuest != nil {
		masterGuest.SetStatus(self.GetUserCred(), models.VM_RESIZE_DISK, "")
	}
	self.StartResizeDisk(ctx, host, storage, disk, masterGuest)
}

func (self *DiskResizeTask) StartResizeDisk(ctx context.Context, host *models.SHost, storage *models.SStorage, disk *models.SDisk, guest *models.SGuest) {
	log.Infof("Resizing disk on host %s ...", host.GetName())
	self.SetStage("OnDiskResizeComplete", nil)
	sizeMb, _ := self.GetParams().Int("size")
	if err := host.GetHostDriver().RequestResizeDiskOnHost(ctx, host, storage, disk, guest, sizeMb, self); err != nil {
		log.Errorf("request_resize_disk_on_host: %v", err)
		self.OnStartResizeDiskFailed(ctx, disk, err)
		return
	}
	self.OnStartResizeDiskSucc(ctx, disk)
}

func (self *DiskResizeTask) OnStartResizeDiskSucc(ctx context.Context, disk *models.SDisk) {
	disk.SetStatus(self.GetUserCred(), models.DISK_RESIZING, "")
}

func (self *DiskResizeTask) OnStartResizeDiskFailed(ctx context.Context, disk *models.SDisk, reason error) {
	disk.SetDiskReady(ctx, self.GetUserCred(), reason.Error())
	self.SetStageFailed(ctx, reason.Error())
	db.OpsLog.LogEvent(disk, db.ACT_RESIZE_FAIL, reason.Error(), self.GetUserCred())
	logclient.AddActionLogWithStartable(self, disk, logclient.ACT_RESIZE, reason.Error(), self.UserCred, false)
}

func (self *DiskResizeTask) OnDiskResizeComplete(ctx context.Context, disk *models.SDisk, data jsonutils.JSONObject) {
	jSize, err := data.Get("disk_size")
	if err != nil {
		log.Errorf("OnDiskResizeComplete error: %s", err.Error())
		self.OnStartResizeDiskFailed(ctx, disk, err)
		return
	}
	sizeMb, err := jSize.Int()
	if err != nil {
		log.Errorf("OnDiskResizeComplete error: %s", err.Error())
		self.OnStartResizeDiskFailed(ctx, disk, err)
		return
	}
	oldStatus := disk.Status
	_, err = db.Update(disk, func() error {
		disk.Status = models.DISK_READY
		disk.DiskSize = int(sizeMb)
		return nil
	})
	if err != nil {
		log.Errorf("OnDiskResizeComplete error: %s", err.Error())
		self.OnStartResizeDiskFailed(ctx, disk, err)
		return
	}
	disk.SetDiskReady(ctx, self.GetUserCred(), "")
	notes := fmt.Sprintf("%s=>%s", oldStatus, disk.Status)
	db.OpsLog.LogEvent(disk, db.ACT_UPDATE_STATUS, notes, self.UserCred)
	self.CleanHostSchedCache(disk)
	db.OpsLog.LogEvent(disk, db.ACT_RESIZE, disk.GetShortDesc(ctx), self.UserCred)
	logclient.AddActionLogWithStartable(self, disk, logclient.ACT_RESIZE, nil, self.UserCred, true)
	self.OnDiskResized(ctx, disk)
}

func (self *DiskResizeTask) OnDiskResized(ctx context.Context, disk *models.SDisk) {
	guestId, _ := self.Params.GetString("guest_id")
	if len(guestId) > 0 {
		self.SetStage("TaskComplete", nil)
		masterGuest := models.GuestManager.FetchGuestById(guestId)
		masterGuest.StartSyncTask(ctx, self.UserCred, false, self.GetId())
	} else {
		self.TaskComplete(ctx, disk, nil)
	}
}

func (self *DiskResizeTask) TaskComplete(ctx context.Context, disk *models.SDisk, data jsonutils.JSONObject) {
	self.SetStageComplete(ctx, disk.GetShortDesc(ctx))
	self.finalReleasePendingUsage(ctx)
}

func (self *DiskResizeTask) TaskCompleteFailed(ctx context.Context, disk *models.SDisk, data jsonutils.JSONObject) {
	self.SetStageFailed(ctx, data.String())
}

func (self *DiskResizeTask) OnDiskResizeCompleteFailed(ctx context.Context, disk *models.SDisk, data jsonutils.JSONObject) {
	disk.SetDiskReady(ctx, self.GetUserCred(), data.String())
	db.OpsLog.LogEvent(disk, db.ACT_RESIZE_FAIL, disk.GetShortDesc(ctx), self.UserCred)
	logclient.AddActionLogWithStartable(self, disk, logclient.ACT_RESIZE, data.String(), self.UserCred, false)
	guestId, _ := self.Params.GetString("guest_id")
	if len(guestId) > 0 {
		masterGuest := models.GuestManager.FetchGuestById(guestId)
		masterGuest.SetStatus(self.UserCred, models.VM_RESIZE_DISK_FAILED, data.String())
	}
	self.SetStageFailed(ctx, data.String())
}
