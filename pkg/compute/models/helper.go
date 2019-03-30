package models

import (
	"context"
	"database/sql"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/utils"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/consts"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/cloudcommon/policy"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
)

func RunBatchCreateTask(
	ctx context.Context,
	items []db.IModel,
	userCred mcclient.TokenCredential,
	data jsonutils.JSONObject,
	pendingUsage SQuota,
	taskName string,
	parentTaskId string,
) {
	taskItems := make([]db.IStandaloneModel, len(items))
	for i, t := range items {
		taskItems[i] = t.(db.IStandaloneModel)
	}
	params := data.(*jsonutils.JSONDict)
	task, err := taskman.TaskManager.NewParallelTask(ctx, taskName, taskItems, userCred, params, parentTaskId, "", &pendingUsage)
	if err != nil {
		log.Errorf("%s newTask error %s", taskName, err)
	} else {
		task.ScheduleRun(nil)
	}
}

func ValidateScheduleCreateData(ctx context.Context, userCred mcclient.TokenCredential, input *api.ServerCreateInput, hypervisor string) (*api.ServerCreateInput, error) {
	var err error

	if input.Baremetal {
		hypervisor = HYPERVISOR_BAREMETAL
	}

	// base validate_create_data
	if (input.PreferHost != "") && hypervisor != HYPERVISOR_CONTAINER {

		if !userCred.IsAdminAllow(consts.GetServiceType(), GuestManager.KeywordPlural(), policy.PolicyActionPerform, "assign-host") {
			return nil, httperrors.NewNotSufficientPrivilegeError("Only system admin can specify preferred host")
		}
		bmName := input.PreferHost
		bmObj, err := HostManager.FetchByIdOrName(nil, bmName)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, httperrors.NewResourceNotFoundError("Host %s not found", bmName)
			} else {
				return nil, httperrors.NewGeneralError(err)
			}
		}
		baremetal := bmObj.(*SHost)
		if !baremetal.Enabled {
			return nil, httperrors.NewInvalidStatusError("Baremetal %s not enabled", bmName)
		}

		if len(hypervisor) > 0 && hypervisor != HOSTTYPE_HYPERVISOR[baremetal.HostType] {
			return nil, httperrors.NewInputParameterError("cannot run hypervisor %s on specified host with type %s", hypervisor, baremetal.HostType)
		}

		if len(hypervisor) == 0 {
			hypervisor = HOSTTYPE_HYPERVISOR[baremetal.HostType]
		}

		if len(hypervisor) == 0 {
			hypervisor = HYPERVISOR_DEFAULT
		}

		_, err = GetDriver(hypervisor).ValidateCreateHostData(ctx, userCred, bmName, baremetal, input)
		if err != nil {
			return nil, err
		}

		input.PreferHost = baremetal.Id
		zone := baremetal.GetZone()
		input.PreferZone = zone.Id
		region := zone.GetRegion()
		input.PreferRegion = region.Id
	} else {
		schedtags := make(map[string]string)
		for _, tag := range input.Schedtags {
			schedtags[tag.Id] = tag.Strategy
		}
		if len(schedtags) > 0 {
			schedtags, err = SchedtagManager.ValidateSchedtags(userCred, schedtags)
			if err != nil {
				return nil, httperrors.NewInputParameterError("invalid aggregate_strategy: %s", err)
			}
			tags := make([]*api.SchedtagConfig, 0)
			for name, strategy := range schedtags {
				tags = append(tags, &api.SchedtagConfig{Id: name, Strategy: strategy})
			}
			input.Schedtags = tags
		}

		if input.PreferWire != "" {
			wireStr := input.PreferWire
			wireObj, err := WireManager.FetchById(wireStr)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, httperrors.NewResourceNotFoundError("Wire %s not found", wireStr)
				} else {
					return nil, httperrors.NewGeneralError(err)
				}
			}
			wire := wireObj.(*SWire)
			input.PreferWire = wire.Id
			zone := wire.GetZone()
			input.PreferZone = zone.Id
			region := zone.GetRegion()
			input.PreferRegion = region.Id
		} else if input.PreferZone != "" {
			zoneStr := input.PreferZone
			zoneObj, err := ZoneManager.FetchById(zoneStr)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, httperrors.NewResourceNotFoundError("Zone %s not found", zoneStr)
				} else {
					return nil, httperrors.NewGeneralError(err)
				}
			}
			zone := zoneObj.(*SZone)
			input.PreferZone = zone.Id
			region := zone.GetRegion()
			input.PreferRegion = region.Id
		} else if input.PreferRegion != "" {
			regionStr := input.PreferRegion
			regionObj, err := CloudregionManager.FetchById(regionStr)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, httperrors.NewResourceNotFoundError("Region %s not found", regionStr)
				} else {
					return nil, httperrors.NewGeneralError(err)
				}
			}
			region := regionObj.(*SCloudregion)
			input.PreferRegion = region.Id
		}
	}

	// default hypervisor
	if len(hypervisor) == 0 {
		hypervisor = HYPERVISOR_KVM
	}

	if !utils.IsInStringArray(hypervisor, HYPERVISORS) {
		return nil, httperrors.NewInputParameterError("Hypervisor %s not supported", hypervisor)
	}

	input.Hypervisor = hypervisor
	return input, nil
}
