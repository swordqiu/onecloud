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

// mthreadsGPUInfos 是摩尔线程智算卡的参考规格。
// S4000、S5000 的产品页都没有给出 FP16、FP32 等峰值，这些字段留 0。
var mthreadsVendorInfo = SGPUVendorInfo{
	Vendor:     "MTHREADS",
	VendorId:   "1ed5",
	Country:    "CN",
	ManageTool: "mthreads-gmi",
}

var mthreadsQuyuan = SGPUGenInfo{
	SGPUVendorInfo: mthreadsVendorInfo,
	Arch:           "曲院",
	ComputeCap:     "2.2",
}

var mthreadsPingHu = SGPUGenInfo{
	SGPUVendorInfo: mthreadsVendorInfo,
	Arch:           "平湖",
	ComputeCap:     "3.1",
}

var mthreadsGPUInfos = []SGPUInfo{
	{
		// 1ed5:0323。DeviceHunt 把 0323、0327、0328 都标成 MTT S4000。
		// 摩尔线程 gpu-compute-driver-bench 把 0323、0327 标成 mp_22，
		// torchada 写 MTT S4000 的计算能力是 2.2（mp_22）。文档中心的架构名是曲院。
		// 官网：48GB、768 GB/s、PCIe 5.0 x16、MTLink 240 GB/s、最大功耗 450W，支持 SR-IOV。
		// mthreads-gmi 报 49152 MiB，ECC 为 N/A。规格表只写各精度“支持”，没有峰值。
		SGPUGenInfo:   mthreadsQuyuan,
		DeviceId:      "0323",
		Name:          "MTT S4000",
		ChipsOnDevice: 1,
		MemorySizeGB:  48,
		BandwidthGBs:  768,
		TdpW:          450, FormFactor: "PCIe", PcieGen: "5.0", PcieLanes: 16,
		Interconnect: "MTLink", InterconnectBandwidthGBs: 240,
		MigSupported: true,
	},
	{
		// 1ed5:0400。摩尔线程 gpu-compute-driver-bench 把 0400 标成 mp_31，
		// torchada 写 MTT S5000 的计算能力是 3.1（mp_31）。芯片 PH100。
		// mthreads-gmi 报 81920 MiB，PCIe x16，ECC 为 On-die EDC，MPC Capable。
		// 官网：OAM 模组，有风冷和液冷两档；8 卡平台用 MTLink 互联。
		// pci.ids 还没有 0400。产品页写支持 FP8 到 FP64，没有给出峰值。
		SGPUGenInfo: mthreadsPingHu,
		DeviceId:    "0400",
		Name:        "MTT S5000", Chip: "PH100",
		ChipsOnDevice: 1,
		MemorySizeGB:  80,
		Ecc:           true,
		FormFactor:    "OAM", PcieLanes: 16,
		Interconnect: "MTLink",
		MigSupported: true,
	},
}
