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

// enflameGPUInfos 是燧原云燧加速卡的参考规格。
// GCU 是自研架构，不直接跑 CUDA。产品页没有公布 FP16、INT8 峰值。
var enflameVendorInfo = SGPUVendorInfo{
	Vendor:     "Enflame",
	VendorId:   "1e36",
	Country:    "CN",
	ManageTool: "efsmi",
}

var enflameGCU320 = SGPUGenInfo{
	SGPUVendorInfo: enflameVendorInfo,
	Arch:           "GCU",
	ComputeCap:     "邃思320",
}

var enflameGCU400 = SGPUGenInfo{
	SGPUVendorInfo: enflameVendorInfo,
	Arch:           "GCU",
	ComputeCap:     "邃思400",
}

var enflameGPUInfos = []SGPUInfo{
	{
		// 1e36:c035。DeviceHunt 把 c035 标成 S60，芯片是邃思 320，面向推理。
		// 产品手册表 3.1.1：48GB、672 GB/s、14 Gbps、PCIe Gen5 x16、ECC、最大 300W、
		// FHFL 双槽位、SR-IOV 4VF。概述写 GDDR6。算力峰值手册没有给出，留 0。
		// 384-bit 由 672 GB/s 和 14 Gbps 反推，手册没有写位宽。
		// Ecc 表示规格支持 ECC。规格容量是 48GB。
		// ECC 关闭时 efsmi 可见 49120 MiB，打开时是 42976 MiB。
		SGPUGenInfo: enflameGCU320,
		DeviceId:    "c035",
		Name:        "S60", Chip: "邃思320",
		ChipsOnDevice: 1,
		MemorySizeGB:  48, MemoryType: "GDDR6", MemoryBusBit: 384, Ecc: true,
		BandwidthGBs: 672,
		TdpW:         300, FormFactor: "PCIe", PcieGen: "5.0", PcieLanes: 16,
		MigSupported: true, MaxVfs: 4,
	},
	{
		// 1e36:c042。DeviceHunt 把 c042 标成 L600，芯片是邃思 400，训推一体。
		// 没有核对到官方规格表，显存和峰值留 0。
		SGPUGenInfo: enflameGCU400,
		DeviceId:    "c042",
		Name:        "L600", Chip: "邃思400",
		ChipsOnDevice: 1,
	},
}
