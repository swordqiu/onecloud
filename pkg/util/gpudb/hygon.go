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

// hygonGPUInfos 是海光 DCU 的参考规格。
// 海光没有公布 K100_AI 的 FP16、INT8、FP32 峰值和显存带宽，这些字段留 0。
var hygonVendorInfo = SGPUVendorInfo{
	Vendor:           "HYGON",
	VendorId:         "1d94",
	Country:          "CN",
	ManageTool:       "hy-smi",
	ComputeBlockName: "CU",
}

var hygonDCU = SGPUGenInfo{
	SGPUVendorInfo: hygonVendorInfo,
	Arch:           "DCU",
	ComputeCap:     "gfx928",
}

var hygonGPUInfos = []SGPUInfo{
	{
		// 1d94:6210 的设备名是 KONGMING，子系统名为 K100_AI。
		// pci.ids 把 K100_AI 标在 6211，没有 6210 这一行。
		// lspci 的显存 BAR 为 64G；DTK 运行时报告 120 CU、65520 MiB，ISA 为 gfx928。
		// 同一台 K100_AI 上 hy-smi 的功耗上限是 400W。
		// 第三方页面把显存写成 HBM2e、带宽 896 GB/s，FP16 写成 192 或 196 TFLOPS，这里不采用。
		SGPUGenInfo: hygonDCU,
		DeviceId:    "6210",
		Name:        "K100_AI", Chip: "KONGMING",
		ChipsOnDevice: 1,
		ComputeBlock:  120,
		MemorySizeGB:  64,
		TdpW:          400, FormFactor: "PCIe",
	},
}
