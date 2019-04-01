package guest

import (
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/scheduler/algorithm/predicates"
	"yunion.io/x/onecloud/pkg/scheduler/core"
)

const (
	ExpectedStatus       = "running"
	ExpectedHostStatus   = "online"
	ExpectedEnableStatus = "enable"
)

// StatusPredicate is to filter the current state of host is available,
// not available host's capacity will be set to 0 and filtered out.
type StatusPredicate struct {
	predicates.BasePredicate
}

func (p *StatusPredicate) Name() string {
	return "host_status"
}

func (p *StatusPredicate) Clone() core.FitPredicate {
	return &StatusPredicate{}
}

func (p *StatusPredicate) Execute(u *core.Unit, c core.Candidater) (bool, []core.PredicateFailureReason, error) {
	h := predicates.NewPredicateHelper(p, u, c)
	hc, err := h.HostCandidate()
	if err != nil {
		return false, nil, err
	}

	curStatus := hc.Status
	curHostStatus := hc.HostStatus
	curEnableStatus := hc.Enabled

	if curStatus != ExpectedStatus {
		h.Exclude2("status", curStatus, ExpectedStatus)
	}

	if curHostStatus != ExpectedHostStatus {
		h.Exclude2("host_status", curHostStatus, ExpectedHostStatus)
	}

	if !curEnableStatus {
		h.Exclude2("enable_status", curEnableStatus, true)
	}

	if hc.Zone.Status != ExpectedEnableStatus {
		h.Exclude2("zone_status", hc.Zone.Status, ExpectedEnableStatus)
	}

	if hc.Cloudprovider != nil {
		if hc.Cloudprovider.Status != api.CLOUD_PROVIDER_CONNECTED {
			h.Exclude2("cloud_provider_status", hc.Cloudprovider.Status, api.CLOUD_PROVIDER_CONNECTED)
		}
		if hc.Cloudprovider.HealthStatus != api.CLOUD_PROVIDER_HEALTH_NORMAL {
			h.Exclude2("cloud_provider_health_status", hc.Cloudprovider.HealthStatus, api.CLOUD_PROVIDER_HEALTH_NORMAL)
		}
	}

	return h.GetResult()
}
