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

// kunlunxinGPUInfos 是昆仑芯加速卡的参考规格。
// 官方资料没有公布 P800 的 FP16、INT8、FP32 峰值，这些字段留 0。
var kunlunxinVendorInfo = SGPUVendorInfo{
	Vendor:     "KUNLUNXIN",
	VendorId:   "1d22",
	Country:    "CN",
	ManageTool: "xpu-smi",
}

var kunlunxinKL3 = SGPUGenInfo{
	SGPUVendorInfo: kunlunxinVendorInfo,
	Arch:           "XPU-P", ComputeCap: "KL3",
}

var kunlunxinGPUInfos = []SGPUInfo{
	{
		// xpu-smi 中 DeviceId 0x3688、厂商 0x1d22，产品名 P800 OAM，架构 KL3。
		// 显存 98304 MiB，功耗上限 400W。
		// PCIe 按 xpu-smi 样例的最大代际记 Gen4 x16。腾讯云部署文档附录写接口为 PCIe Gen5 x16，这里不改。
		SGPUGenInfo: kunlunxinKL3,
		DeviceId:    "3688",
		Name:        "P800 OAM", Chip: "P800",
		ChipsOnDevice: 1,
		MemorySizeGB:  96,
		TdpW:          400, FormFactor: "OAM", PcieGen: "4.0", PcieLanes: 16,
	},
}
