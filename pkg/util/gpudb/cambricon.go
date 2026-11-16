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

// cambriconGPUInfos 是寒武纪思元加速卡的参考规格。
// MLU 是自研 ASIC，软件栈是 BANG / CNRT，不兼容 CUDA。
// 370 的峰值按产品页原表填写。INT8 是推理主指标。没有单独的向量峰值。
var cambriconVendorInfo = SGPUVendorInfo{
	Vendor:     "Cambricon",
	VendorId:   "cabc",
	Country:    "CN",
	ManageTool: "cnmon",
}

var cambriconMLUarch03 = SGPUGenInfo{
	SGPUVendorInfo:      cambriconVendorInfo,
	Arch:                "MLUarch03",
	ComputeCap:          "MLU370",
	MatrixPeakPrecision: "int8",
}

var cambriconGPUInfos = []SGPUInfo{
	{
		// cabc:0370，子系统 0053。官网 S4/S8 共用一张表：
		// INT8 192、FP16/BF16 72、FP32 18，24GB/48GB LPDDR5，307.2 GB/s，75W，PCIe 4.0 x16。
		// S4 同为 24GB、1 颗芯片，不另列。cnmon 把 S4 报成 23365 MiB。
		SGPUGenInfo: cambriconMLUarch03,
		DeviceId:    "0370",
		Name:        "MLU370-S8", Chip: "思元370",
		ChipsOnDevice: 1,
		MemorySizeGB:  48, MemoryType: "LPDDR5",
		BandwidthGBs: 307.2,
		Fp32Tflops:   18, Fp16Tflops: 72, Bf16Tflops: 72, Int8Tops: 192,
		PeakMatrixTops: 192,
		TdpW:           75, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
	},
	{
		// 子系统 0057。官网：INT8 256、FP16/BF16 96、FP32 24，
		// 24GB LPDDR5，307.2 GB/s，150W，PCIe 4.0 x16，全高全长单槽。
		SGPUGenInfo: cambriconMLUarch03,
		DeviceId:    "0370",
		Name:        "MLU370-X4", Chip: "思元370",
		ChipsOnDevice: 1,
		MemorySizeGB:  24, MemoryType: "LPDDR5",
		BandwidthGBs: 307.2,
		Fp32Tflops:   24, Fp16Tflops: 96, Bf16Tflops: 96, Int8Tops: 256,
		PeakMatrixTops: 256,
		TdpW:           150, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
	},
	{
		// 子系统 0054。双芯思元 370，一张卡在 lspci 里是两个 0370。
		// 官网峰值仍是 INT8 256、FP16/BF16 96、FP32 24，没有按双芯翻倍。
		// 48GB LPDDR5，614.4 GB/s，250W，MLU-Link 聚合 200 GB/s。
		SGPUGenInfo: cambriconMLUarch03,
		DeviceId:    "0370",
		Name:        "MLU370-X8", Chip: "思元370",
		ChipsOnDevice: 2,
		MemorySizeGB:  48, MemoryType: "LPDDR5",
		BandwidthGBs: 614.4,
		Fp32Tflops:   24, Fp16Tflops: 96, Bf16Tflops: 96, Int8Tops: 256,
		PeakMatrixTops: 256,
		TdpW:           250, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
		Interconnect: "MLU-Link", InterconnectBandwidthGBs: 200,
	},
}
