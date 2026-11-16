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

// theadGPUInfos 是平头哥真武 PPU 的参考规格。
// 产品页和 ppu-smi 都没有给出 FP16、FP32 等峰值，这些字段留 0。
// pccl 还列出 ZW-M890L、ZW-M530、ZW-M530P，没有规格转储，不入库。
var theadVendorInfo = SGPUVendorInfo{
	Vendor:           "THEAD",
	VendorId:         "1ded",
	Country:          "CN",
	ManageTool:       "ppu-smi",
	ComputeBlockName: "CU",
}

var theadBeethoven80 = SGPUGenInfo{
	SGPUVendorInfo: theadVendorInfo,
	Arch:           "Beethoven",
	ComputeCap:     "8.0",
}

var theadBeethoven89 = SGPUGenInfo{
	SGPUVendorInfo: theadVendorInfo,
	Arch:           "Beethoven",
	ComputeCap:     "8.9",
}

var theadGPUInfos = []SGPUInfo{
	{
		// 1ded:6001。ppu-smi 把 Device Id 印成 0x60011DED，产品名 PPU-ZW810E。
		// 阿里云产品页称真武 810E：OAM，96GB HBM2e，带宽 2765 GB/s，
		// PCIe 5.0 x16，ICN 700 GB/s，TDP 400W。
		// ppu-smi：架构 Beethoven，计算能力 8.0，显存 98304 MiB，ECC 可打开，带 MIG 开关。
		// 16 卡节点上 ppu-smi 报的链路上限是 8x。
		SGPUGenInfo:   theadBeethoven80,
		DeviceId:      "6001",
		Name:          "PPU-ZW810E",
		ChipsOnDevice: 1,
		MemorySizeGB:  96, MemoryType: "HBM2e", Ecc: true,
		BandwidthGBs: 2765,
		TdpW:         400, FormFactor: "OAM", PcieGen: "5.0", PcieLanes: 16,
		Interconnect: "ICN", InterconnectBandwidthGBs: 700,
		MigSupported: true, MaxVfs: 2,
	},
	{
		// 1ded:6005。平头哥 pla 的 ppu-smi 转储：产品名 ZW-M890P，
		// 架构 Beethoven，计算能力 8.9，显存 98304 MiB，HBM 厂商 Samsung。
		// PCIe 5.0 x16，默认功耗上限 850W，ECC 打开，带 MIG 开关。
		// 阿里云把超节点互联写成 ICN Link，没有单卡带宽和显存代次。
		// 超节点文章另写单卡 144GB、ICN 800 GB/s，与这份 ppu-smi 读数不一致，不采用。
		SGPUGenInfo:   theadBeethoven89,
		DeviceId:      "6005",
		Name:          "ZW-M890P",
		ChipsOnDevice: 1,
		MemorySizeGB:  96, Ecc: true,
		TdpW: 850, PcieGen: "5.0", PcieLanes: 16,
		Interconnect: "ICN",
		MigSupported: true,
	},
}
