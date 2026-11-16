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

// huaweiGPUInfos 是华为昇腾推理卡、视频解析卡和训练卡的参考规格。
//
// 达芬奇 AI Core 同时含 Cube、Vector 和 Scalar，三个数量都按 AI Core 个数记。
// 产品页公布的 FP16 是 Cube 峰值，写入 Fp16Tflops，不计入 PeakVectorTflops。
// 310P 的推理主指标是 INT8。910、910B 的主指标是 FP16，INT8 为 FP16 的 2 倍。
var huaweiVendorInfo = SGPUVendorInfo{
	Vendor:                 "Ascend",
	VendorId:               "19e5",
	Country:                "CN",
	ManageTool:             "npu-smi",
	ComputeBlockName:       "AI Core",
	SchedulingExplicitTile: true,
	VectorPeakPrecision:    "fp16",
}

var (
	huaweiDaVinci310P = SGPUGenInfo{
		SGPUVendorInfo: huaweiVendorInfo,
		Arch:           "Da Vinci", ComputeCap: "Ascend310P",
		TensorSparsity: 1, MatrixPeakPrecision: "int8",
	}
	huaweiDaVinci910 = SGPUGenInfo{
		SGPUVendorInfo: huaweiVendorInfo,
		Arch:           "Da Vinci", ComputeCap: "Ascend910",
		TensorSparsity: 1, MatrixPeakPrecision: "fp16",
	}
	huaweiDaVinci910B = SGPUGenInfo{
		SGPUVendorInfo: huaweiVendorInfo,
		Arch:           "Da Vinci", ComputeCap: "Ascend910B",
		TensorSparsity: 1, MatrixPeakPrecision: "fp16",
	}
)

var huaweiGPUInfos = []SGPUInfo{
	{
		// 310P 为 8 个 AI Core。产品页写的是 8 核 1.9 GHz CPU。
		// 用户指南内存规格写支持 ECC。
		// Atlas 300V 同为 24GB、1 颗芯片，不另列。
		SGPUGenInfo: huaweiDaVinci310P,
		DeviceId:    "d500",
		Name:        "Atlas 300I Pro", Chip: "Ascend 310P",
		ChipsOnDevice: 1,
		ComputeBlock:  8,
		VectorLanes:   8, MatrixEngines: 8, ScalarUnit: 8,
		PeakMatrixTops: 140,
		MemorySizeGB:   24, MemoryType: "LPDDR4X", Ecc: true,
		BandwidthGBs: 204.8,
		Fp16Tflops:   70, Int8Tops: 140,
		TdpW: 72, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
	},
	{
		// 两颗 Ascend 310P。另有 48GB 版，算力和带宽相同。
		SGPUGenInfo: huaweiDaVinci310P,
		DeviceId:    "d500",
		Name:        "Atlas 300I Duo", Chip: "Ascend 310P",
		ChipsOnDevice: 2,
		ComputeBlock:  16,
		VectorLanes:   16, MatrixEngines: 16, ScalarUnit: 16,
		PeakMatrixTops: 280,
		MemorySizeGB:   96, MemoryType: "LPDDR4X", Ecc: true,
		BandwidthGBs: 408,
		Fp16Tflops:   140, Int8Tops: 280,
		TdpW: 150, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
	},
	{
		// 产品页没有单列 ECC，这里不标。
		SGPUGenInfo: huaweiDaVinci310P,
		DeviceId:    "d500",
		Name:        "Atlas 300V Pro", Chip: "Ascend 310P",
		ChipsOnDevice: 1,
		ComputeBlock:  8,
		VectorLanes:   8, MatrixEngines: 8, ScalarUnit: 8,
		PeakMatrixTops: 140,
		MemorySizeGB:   48, MemoryType: "LPDDR4X",
		BandwidthGBs: 204.8,
		Fp16Tflops:   70, Int8Tops: 140,
		TdpW: 72, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
	},
	{
		// 用户指南写 1 颗 910，20 个 AI Core。DeviceId d802，按 910B 归档。
		// 64GB 版带宽 1600 GB/s，算力相同。功耗也可配 300W。产品彩页另给 FP32 75。
		// Atlas 300T A2 同为 32GB、1 颗芯片，不另列。
		SGPUGenInfo: huaweiDaVinci910B,
		DeviceId:    "d802",
		Name:        "Atlas 300I A2 32GB", Chip: "Ascend 910B",
		ChipsOnDevice: 1,
		ComputeBlock:  20,
		VectorLanes:   20, MatrixEngines: 20, ScalarUnit: 20,
		PeakMatrixTops: 280,
		MemorySizeGB:   32, MemoryType: "HBM", Ecc: true,
		BandwidthGBs: 800,
		Fp32Tflops:   75, Fp16Tflops: 280, Int8Tops: 560,
		TdpW: 350, FormFactor: "PCIe", PcieGen: "5.0", PcieLanes: 16,
	},
	{
		// 昇腾社区 Atlas 300I A2 规格表的 64GB 档：带宽 1.6 TB/s、FP16 280 TFLOPS、INT8 560 TOPS、最大功耗 350W、PCIe 5.0。
		// 同表 32GB 档算力相同。产品彩页另给 FP32 75。表里的 8 core × 2.0 GHz 是 CPU，不是 AI Core。
		// npu-smi 把芯片报成 910B4-1，不是这块卡的型号。该 d802 卡 lspci 的 SR-IOV Total VFs 是 12。
		SGPUGenInfo: huaweiDaVinci910B,
		DeviceId:    "d802",
		Name:        "Atlas 300I A2 64GB", Chip: "Ascend 910B",
		ChipsOnDevice: 1,
		ComputeBlock:  20,
		VectorLanes:   20, MatrixEngines: 20, ScalarUnit: 20,
		PeakMatrixTops: 280,
		MemorySizeGB:   64, MemoryType: "HBM", Ecc: true,
		BandwidthGBs: 1600,
		Fp32Tflops:   75, Fp16Tflops: 280, Int8Tops: 560,
		TdpW: 350, FormFactor: "PCIe", PcieGen: "5.0", PcieLanes: 16,
		MigSupported: true, MaxVfs: 12,
	},
	{
		// 用户指南写 FP16 280 TFLOPS，内存支持 ECC。INT8 按本代 2 倍记 560。
		// 另有 16GB DDR4。HBM 带宽用户指南未单列。
		// Atlas 300T 同为 32GB、1 颗芯片，不另列。
		SGPUGenInfo: huaweiDaVinci910,
		DeviceId:    "d801",
		Name:        "Atlas 300T Pro", Chip: "Ascend 910",
		ChipsOnDevice: 1,
		ComputeBlock:  30,
		VectorLanes:   30, MatrixEngines: 30, ScalarUnit: 30,
		PeakMatrixTops: 280,
		MemorySizeGB:   32, MemoryType: "HBM", Ecc: true,
		Fp16Tflops: 280, Int8Tops: 560,
		TdpW: 300, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
		Interconnect: "RoCE", InterconnectBandwidthGBs: 12.5,
		MigSupported: true,
	},
}
