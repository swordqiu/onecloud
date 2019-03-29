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

package models

import (
	"context"
	"time"

	"yunion.io/x/jsonutils"

	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/mcclient"
)

type SActionlogManager struct {
	db.SOpsLogManager
}

type SActionlog struct {
	db.SOpsLog

	StartTime time.Time `nullable:"false" list:"user" create:"optional"`                          // = Column(DateTime, nullable=False)
	Success   bool      `list:"user" create:"required"`                                           // = Column(Boolean, default=True)
	Service   string    `width:"32" charset:"utf8" nullable:"true" list:"user" create:"optional"` //= Column(VARCHAR(32, charset='utf8'), nullable=False)
}

var ActonLog *SActionlogManager

func init() {
	ActonLog = &SActionlogManager{db.SOpsLogManager{db.NewModelBaseManager(SActionlog{}, "action_tbl", "action", "actions")}}
}

func (action *SActionlog) CustomizeCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerProjId string, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	now := time.Now().UTC()
	action.OpsTime = now
	if action.StartTime.IsZero() {
		action.StartTime = now
	}
	return nil
}
