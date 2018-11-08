package options

type LoadbalancerBackendGroupCreateOptions struct {
	NAME         string
	Loadbalancer string
}

type LoadbalancerBackendGroupGetOptions struct {
	ID string
}

type LoadbalancerBackendGroupUpdateOptions struct {
	ID   string
	Name string
}

type LoadbalancerBackendGroupDeleteOptions struct {
	ID string
}

type LoadbalancerBackendGroupListOptions struct {
	BaseListOptions
	Loadbalancer string
}
