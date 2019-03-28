package modules

var (
	CloudproviderregionManager JointResourceManager
)

func init() {
	CloudproviderregionManager = NewJointComputeManager("cloudproviderregion",
		"cloudproviderregions",
		[]string{"Cloudaccount_ID", "Cloudaccount",
			"Cloudprovider_ID", "CloudProvider",
			"Cloudregion_ID", "CloudRegion",
			"Enabled", "Sync_Status",
			"Last_Sync", "Last_Sync_End_At", "Auto_Sync",
			"last_deep_sync_at",
			"Sync_Results"},
		[]string{},
		&Cloudproviders,
		&Cloudregions)

	registerCompute(&CloudproviderregionManager)
}
