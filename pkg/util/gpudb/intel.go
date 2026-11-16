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

// intelGPUInfos 是英特尔数据中心 GPU 的参考规格。
//
// Flex 170 的向量 FP32 用 oneAPI 的 8192 FLOPs/clk 乘规格页 2050 MHz。
// 产品简介里的 16 TFLOPS FP32 是 XMX 脉动阵列，不写入 Fp32Tflops。
// Xe-HPG 的 subgroup 可以是 8、16、32，SchedulingWidth 留 0。
var intelVendorInfo = SGPUVendorInfo{
	Vendor:              "Intel",
	VendorId:            "8086",
	Country:             "US",
	ManageTool:          "xpu-smi",
	ComputeBlockName:    "Xe Core",
	VectorPeakPrecision: "fp32",
}

var intelXeHPG = SGPUGenInfo{
	SGPUVendorInfo:      intelVendorInfo,
	Arch:                "Xe-HPG",
	ComputeCap:          "DG2",
	TensorSparsity:      1,
	MatrixPeakPrecision: "int8",
}

var intelGPUInfos = []SGPUInfo{
	{
		// 8086:56c0 是 Data Center GPU Flex 170，开源参考名 ATS-M150，芯片 ACM-G10。
		// 规格页：32 Xe-core、512 EU、512 XMX，16GB GDDR6，256-bit，576 GB/s，
		// 显卡 2050 MHz、XMX 1950 MHz，TBP 150W，PCIe 4.0 x16。发布于 2022 年第三季度。
		// 产品简介：全高四分之三长单宽被动卡，SR-IOV 31 个实例，
		// 脉动阵列 16 TFLOPS FP32 / 250 TOPS INT8。
		// oneAPI：单精度 MAD 8192 FLOPs/clk，FP16 DP4AS 65536 FLOPs/clk，无 FP64。
		// 向量 FP32 = 8192 × 2.050 GHz。FP16 = 65536 × 1.950 GHz，联想指南写 128。
		// 按该 FP16 的 2 倍，INT8 约为 256 TOPS；入库用产品简介的 250。
		// 系列页 “up to 500 TOPS” 不写入 INT8。联想指南写 ECC，英特尔规格页未单列。
		SGPUGenInfo: intelXeHPG,
		DeviceId:    "56c0",
		Name:        "Data Center GPU Flex 170", Chip: "ACM-G10",
		ChipsOnDevice: 1,
		ComputeBlock:  32, VectorLanes: 512, MatrixEngines: 512,
		MemorySizeGB: 16, MemoryType: "GDDR6", MemoryBusBit: 256,
		BandwidthGBs: 576,
		Fp32Tflops:   16.79, Fp16Tflops: 127.8, Int8Tops: 250,
		PeakVectorTflops: 16.79, PeakMatrixTops: 250,
		TdpW: 150, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
		MigSupported: true, MaxVfs: 31,
		ReleaseDate: "2022-08",
	},
}
