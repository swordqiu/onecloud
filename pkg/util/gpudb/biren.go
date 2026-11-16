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

// birenGPUInfos 是壁仞加速卡的参考规格。
// 公开资料没有把 1ee0:0004 对应到单一板卡，显存和峰值留 0。
var birenVendorInfo = SGPUVendorInfo{
	Vendor:     "BIREN",
	VendorId:   "1ee0",
	Country:    "CN",
	ManageTool: "brsmi",
}

var birenGPUInfos = []SGPUInfo{
	{
		// HAMi 壁仞设备插件把厂商 ID 写成 1ee0，管理工具是 brsmi。
		// pci.ids 没有 1ee0。0004 是壁仞 GPU 的设备 ID，公开驱动没有给出板卡名。
		SGPUGenInfo:   SGPUGenInfo{SGPUVendorInfo: birenVendorInfo},
		DeviceId:      "0004",
		Name:          "Biren 0004",
		ChipsOnDevice: 1,
	},
}
