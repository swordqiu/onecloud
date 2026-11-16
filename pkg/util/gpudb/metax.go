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

// metaxGPUInfos 是沐曦通用 GPU 的参考规格。
// 架构是自研 GPGPU，软件栈 MXMACA。官网产品页没有给出峰值，这些字段留 0。
var metaxVendorInfo = SGPUVendorInfo{
	Vendor:     "MetaX",
	VendorId:   "9999",
	Country:    "CN",
	ManageTool: "mx-smi",
}

var metaxMXC500 = SGPUGenInfo{
	SGPUVendorInfo: metaxVendorInfo,
	Arch:           "GPGPU",
	ComputeCap:     "MXC500",
}

var metaxGPUInfos = []SGPUInfo{
	{
		// 9999:4001。驱动安装指南用这个 ID 作为曦云 C500 的 lspci 示例。
		// 同代支持 SR-IOV 和 sGPU。
		SGPUGenInfo:   metaxMXC500,
		DeviceId:      "4001",
		Name:          "曦云C500",
		ChipsOnDevice: 1,
		Interconnect:  "MetaXLink",
		MigSupported:  true,
	},
}
