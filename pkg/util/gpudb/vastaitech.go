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

package gpudb

// vastaitechGPUInfos 是瀚博加速卡的参考规格。
// 2021 年发布会称 SV100 单芯片 INT8 超过 200 TOPS，支持 FP16、BF16，没有精确峰值，这些字段留 0。
var vastaitechVendorInfo = SGPUVendorInfo{
	Vendor:     "VASTAITECH",
	VendorId:   "1ec6",
	Country:    "CN",
	ManageTool: "vasmi",
}

var vastaitechSV100 = SGPUGenInfo{
	SGPUVendorInfo: vastaitechVendorInfo,
	Arch:           "VUCA",
}

var vastaitechGPUInfos = []SGPUInfo{
	{
		// 1ec6:0100 的设备名是 SV100。
		// 同期载天 VA1 写 32GB 内存，类型未公布，PCIe 4.0 x16，半高半长单槽。
		// 发布会写 75W，现行 VA1 产品页写 70W。载天 VA10 也基于 SV100，产品页写 150W，不并入这一条。
		SGPUGenInfo: vastaitechSV100,
		DeviceId:    "0100",
		Name:        "SV100", Chip: "SV100",
		ChipsOnDevice: 1,
		MemorySizeGB:  32,
		TdpW:          70, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
	},
}
