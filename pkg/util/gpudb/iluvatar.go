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

// iluvatarGPUInfos 是天数智芯加速卡的参考规格。
// 官方没有公布这些型号的 FP32、FP16、INT8 峰值，这些字段留 0。
var iluvatarVendorInfo = SGPUVendorInfo{
	Vendor:     "ILUVATAR",
	VendorId:   "1e3e",
	Country:    "CN",
	ManageTool: "ixsmi",
}

var iluvatarGPUInfos = []SGPUInfo{
	{
		// 1e3e:0002 是智铠100（MR-V100）。DeepSpark 把 0002 标为智铠加速卡，
		// 天数安装手册里该 DeviceId 的显存 BAR 为 32G。
		// 官网：32GB HBM2E，150W，全高全长单槽，PCIe Gen4 x16。
		SGPUGenInfo: SGPUGenInfo{SGPUVendorInfo: iluvatarVendorInfo},
		DeviceId:    "0002",
		Name:        "MR-V100", Chip: "MR-V100",
		ChipsOnDevice: 1,
		MemorySizeGB:  32, MemoryType: "HBM2e",
		TdpW: 150, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
	},
	{
		// 1e3e:000f 是天垓 150S（BI-V150S）。
		// 天数知识库写 TDP 450W，ixsmi 样例的功耗上限也是 450W。
		// 产品页写整卡 64GB HBM2e；ixsmi 对每个 BI-V150S 设备报 32768 MiB。
		// 产品资料写 PCIe Gen4 x16。
		SGPUGenInfo: SGPUGenInfo{SGPUVendorInfo: iluvatarVendorInfo},
		DeviceId:    "000f",
		Name:        "BI-V150S", Chip: "BI-V150S",
		ChipsOnDevice: 1,
		MemorySizeGB:  32, MemoryType: "HBM2e",
		TdpW: 450, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
	},
}
