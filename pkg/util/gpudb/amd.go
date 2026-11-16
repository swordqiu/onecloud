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

// amdGPUInfos 是 AMD Instinct 型号的参考规格。
// Fp32Tflops、Fp64Tflops 记 vector，Matrix 峰值写在对应条目的注释里。
var amdVendorInfo = SGPUVendorInfo{
	Vendor:              "AMD",
	VendorId:            "1002",
	Country:             "US",
	ManageTool:          "rocm-smi",
	ComputeBlockName:    "CU",
	SchedulingWidth:     64,
	VectorPeakPrecision: "fp32",
}

var (
	amdCDNA1 = SGPUGenInfo{
		SGPUVendorInfo: amdVendorInfo,
		Arch:           "CDNA", ComputeCap: "gfx908",
		TensorSparsity: 1, MatrixPeakPrecision: "fp16",
	}
	amdCDNA2 = SGPUGenInfo{
		SGPUVendorInfo: amdVendorInfo,
		Arch:           "CDNA2", ComputeCap: "gfx90a",
		TensorSparsity: 1, MatrixPeakPrecision: "int8",
	}
	amdCDNA3 = SGPUGenInfo{
		SGPUVendorInfo: amdVendorInfo,
		Arch:           "CDNA3", ComputeCap: "gfx942",
		TensorSparsity: 2, MatrixPeakPrecision: "fp8",
	}
	amdCDNA4 = SGPUGenInfo{
		SGPUVendorInfo: amdVendorInfo,
		Arch:           "CDNA4", ComputeCap: "gfx950",
		TensorSparsity: 2, MatrixPeakPrecision: "fp4",
	}
)

var amdGPUInfos = []SGPUInfo{
	{
		// FP32 Matrix 为 46.1 TFLOPS。INT8、BF16 为 92.3。官方带宽写 1.2 TB/s。
		// 3 条 Infinity Fabric，每条 92 GB/s。另有 DeviceId 738e。
		SGPUGenInfo: amdCDNA1,
		DeviceId:    "738c",
		Name:        "Instinct MI100", Chip: "Arcturus",
		ChipsOnDevice: 1,
		ComputeBlock:  120,
		VectorLanes:   7680, MatrixEngines: 480, ScalarUnit: 120,
		PeakVectorTflops: 23.1,
		PeakMatrixTops:   184.6,
		MemorySizeGB:     32, MemoryType: "HBM2", MemoryBusBit: 4096, Ecc: true,
		BandwidthGBs: 1228.8,
		Fp32Tflops:   23.1, Fp64Tflops: 11.5,
		Fp16Tflops: 184.6, Bf16Tflops: 92.3, Int8Tops: 92.3,
		TdpW: 300, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
		Interconnect: "Infinity Fabric", InterconnectBandwidthGBs: 276,
		ReleaseDate: "2020-11-16",
	},
	{
		// FP64/FP32 Matrix 另为 45.3 TFLOPS，这里记的是 vector。
		SGPUGenInfo: amdCDNA2,
		DeviceId:    "740f",
		Name:        "Instinct MI210", Chip: "Aldebaran",
		ChipsOnDevice: 1,
		ComputeBlock:  104,
		VectorLanes:   6656, MatrixEngines: 416, ScalarUnit: 104,
		PeakVectorTflops: 22.6,
		PeakMatrixTops:   181,
		MemorySizeGB:     64, MemoryType: "HBM2e", MemoryBusBit: 4096, Ecc: true,
		BandwidthGBs: 1638.4,
		Fp32Tflops:   22.6, Fp64Tflops: 22.6,
		Fp16Tflops: 181, Bf16Tflops: 181, Int8Tops: 181,
		TdpW: 300, FormFactor: "PCIe", PcieGen: "4.0", PcieLanes: 16,
		Interconnect: "Infinity Fabric", InterconnectBandwidthGBs: 300,
		ReleaseDate: "2022-03-22",
	},
	{
		// FP64/FP32 Matrix 另为 90.5 TFLOPS，这里记的是 vector。TDP 500W，峰值功耗 560W。
		SGPUGenInfo: amdCDNA2,
		DeviceId:    "740c",
		Name:        "Instinct MI250", Chip: "Aldebaran",
		ChipsOnDevice: 1,
		ComputeBlock:  208,
		VectorLanes:   13312, MatrixEngines: 832, ScalarUnit: 208,
		PeakVectorTflops: 45.3,
		PeakMatrixTops:   362.1,
		MemorySizeGB:     128, MemoryType: "HBM2e", MemoryBusBit: 8192, Ecc: true,
		BandwidthGBs: 3276.8,
		Fp32Tflops:   45.3, Fp64Tflops: 45.3,
		Fp16Tflops: 362.1, Bf16Tflops: 362.1, Int8Tops: 362.1,
		TdpW: 500, FormFactor: "OAM", PcieGen: "4.0", PcieLanes: 16,
		Interconnect: "Infinity Fabric", InterconnectBandwidthGBs: 800,
		ReleaseDate: "2021-11-08",
	},
	{
		// FP64/FP32 Matrix 另为 95.7 TFLOPS，这里记的是 vector。TDP 500W，峰值功耗 560W。
		// 官方带宽写 3.2 TB/s。8 条 Infinity Fabric，每条 100 GB/s。
		SGPUGenInfo: amdCDNA2,
		DeviceId:    "7408",
		Name:        "Instinct MI250X", Chip: "Aldebaran",
		ChipsOnDevice: 1,
		ComputeBlock:  220,
		VectorLanes:   14080, MatrixEngines: 880, ScalarUnit: 220,
		PeakVectorTflops: 47.9,
		PeakMatrixTops:   383,
		MemorySizeGB:     128, MemoryType: "HBM2e", MemoryBusBit: 8192, Ecc: true,
		BandwidthGBs: 3276.8,
		Fp32Tflops:   47.9, Fp64Tflops: 47.9,
		Fp16Tflops: 383, Bf16Tflops: 383, Int8Tops: 383,
		TdpW: 500, FormFactor: "OAM", PcieGen: "4.0", PcieLanes: 16,
		Interconnect: "Infinity Fabric", InterconnectBandwidthGBs: 800,
		ReleaseDate: "2021-11-08",
	},
	{
		// MI300 家族里的独立 GPU。MI300A 是 APU，见单独条目。
		// FP64 Matrix 为 163.4 TFLOPS，Fp64Tflops 记的是 vector 81.7。
		SGPUGenInfo: amdCDNA3,
		DeviceId:    "74a1",
		Name:        "Instinct MI300X", Chip: "Aqua Vanjaram",
		ChipsOnDevice: 1,
		ComputeBlock:  304,
		VectorLanes:   19456, MatrixEngines: 1216, ScalarUnit: 304,
		PeakVectorTflops: 163.4,
		PeakMatrixTops:   2614.9,
		MemorySizeGB:     192, MemoryType: "HBM3", MemoryBusBit: 8192, Ecc: true,
		BandwidthGBs: 5300,
		Fp32Tflops:   163.4, Fp64Tflops: 81.7,
		Tf32Tflops: 653.7, Fp16Tflops: 1307.4, Bf16Tflops: 1307.4,
		Fp8Tflops: 2614.9, Int8Tops: 2614.9,
		TdpW: 750, FormFactor: "OAM", PcieGen: "5.0", PcieLanes: 16,
		Interconnect: "Infinity Fabric", InterconnectBandwidthGBs: 896,
		MigSupported: true, MaxVfs: 8,
		ReleaseDate: "2023-12-06",
	},
	{
		// 算力与 MI300X 相同。显存换成 256GB HBM3E、6 TB/s，功耗 1000W。
		// 8 条 Infinity Fabric，每条 128 GB/s。
		SGPUGenInfo: amdCDNA3,
		DeviceId:    "74a5",
		Name:        "Instinct MI325X", Chip: "Aqua Vanjaram",
		ChipsOnDevice: 1,
		ComputeBlock:  304,
		VectorLanes:   19456, MatrixEngines: 1216, ScalarUnit: 304,
		PeakVectorTflops: 163.4,
		PeakMatrixTops:   2614.9,
		MemorySizeGB:     256, MemoryType: "HBM3E", MemoryBusBit: 8192, Ecc: true,
		BandwidthGBs: 6000,
		Fp32Tflops:   163.4, Fp64Tflops: 81.7,
		Tf32Tflops: 653.7, Fp16Tflops: 1307.4, Bf16Tflops: 1307.4,
		Fp8Tflops: 2614.9, Int8Tops: 2614.9,
		TdpW: 1000, FormFactor: "OAM", PcieGen: "5.0", PcieLanes: 16,
		Interconnect: "Infinity Fabric", InterconnectBandwidthGBs: 1024,
		MigSupported: true,
		ReleaseDate:  "2024-10-10",
	},
	{
		// APU，SH5 插槽，另有 24 核 Zen 4。TDP 550W，峰值功耗 760W。
		// FP64 Matrix 为 122.6 TFLOPS，Fp64Tflops 记的是 vector 61.3。
		// 8 条 Infinity Fabric，每条 128 GB/s。官方支持 SR-IOV。
		SGPUGenInfo: amdCDNA3,
		DeviceId:    "74a0",
		Name:        "Instinct MI300A", Chip: "Aqua Vanjaram",
		ChipsOnDevice: 1,
		ComputeBlock:  228,
		VectorLanes:   14592, MatrixEngines: 912, ScalarUnit: 228,
		PeakVectorTflops: 122.6,
		PeakMatrixTops:   1960,
		MemorySizeGB:     128, MemoryType: "HBM3", MemoryBusBit: 8192, Ecc: true,
		BandwidthGBs: 5300,
		Fp32Tflops:   122.6, Fp64Tflops: 61.3,
		Tf32Tflops: 490.3, Fp16Tflops: 980.6, Bf16Tflops: 980.6,
		Fp8Tflops: 1960, Int8Tops: 1960,
		TdpW: 550, FormFactor: "SH5", PcieGen: "5.0", PcieLanes: 16,
		Interconnect: "Infinity Fabric", InterconnectBandwidthGBs: 1024,
		MigSupported: true,
		ReleaseDate:  "2023-12-06",
	},
	{
		// 与 MI355X 同为 CDNA4、288GB。引擎时钟 2200 MHz，MXFP4、MXFP6 为 9.2 PFLOPS。
		// OCP-FP8 稠密 4.6 PFLOPS、稀疏 9.2。TF32 没有硬件峰值。
		// 7 条 Infinity Fabric，每条 144 GB/s。
		SGPUGenInfo: amdCDNA4,
		DeviceId:    "75a0",
		Name:        "Instinct MI350X", Chip: "Aqua Vanjaram",
		ChipsOnDevice: 1,
		ComputeBlock:  256,
		VectorLanes:   16384, MatrixEngines: 1024, ScalarUnit: 256,
		PeakVectorTflops: 144.2,
		PeakMatrixTops:   9200,
		MemorySizeGB:     288, MemoryType: "HBM3E", MemoryBusBit: 8192, Ecc: true,
		BandwidthGBs: 8000,
		Fp32Tflops:   144.2, Fp64Tflops: 72.1,
		Fp16Tflops: 2300, Bf16Tflops: 2300,
		Fp8Tflops: 4600, Fp6Tflops: 9200, Fp4Tflops: 9200, Int8Tops: 4600,
		TdpW: 1000, FormFactor: "OAM", PcieGen: "5.0", PcieLanes: 16,
		Interconnect: "Infinity Fabric", InterconnectBandwidthGBs: 1008,
		MigSupported: true, MaxVfs: 8,
		ReleaseDate: "2025-06-12",
	},
	{
		// PCIe 卡，128 CU，是 MI350X 的一半。功耗最高 600W，可配到 450W。
		// MXFP4、MXFP6 为 4.6 PFLOPS。没有卡间 Infinity Fabric。
		SGPUGenInfo: amdCDNA4,
		DeviceId:    "75a8",
		Name:        "Instinct MI350P", Chip: "Aqua Vanjaram",
		ChipsOnDevice: 1,
		ComputeBlock:  128,
		VectorLanes:   8192, MatrixEngines: 512, ScalarUnit: 128,
		PeakVectorTflops: 72,
		PeakMatrixTops:   4600,
		MemorySizeGB:     144, MemoryType: "HBM3E", MemoryBusBit: 4096, Ecc: true,
		BandwidthGBs: 4000,
		Fp32Tflops:   72, Fp64Tflops: 36,
		Fp16Tflops: 1150, Bf16Tflops: 1150,
		Fp8Tflops: 2300, Fp6Tflops: 4600, Fp4Tflops: 4600, Int8Tops: 2300,
		TdpW: 600, FormFactor: "PCIe", PcieGen: "5.0", PcieLanes: 16,
		MigSupported: true,
	},
	{
		// CDNA4。MXFP4、MXFP6 官方峰值都是 10.1 PFLOPS，没有另给稀疏倍率。
		// OCP-FP8 稠密 5 PFLOPS、稀疏 10.1。TF32 没有硬件峰值。
		// FP32、FP64 的 Matrix 与 vector 同为 157.3 和 78.6。
		// 7 条 Infinity Fabric，每条 144 GB/s。
		SGPUGenInfo: amdCDNA4,
		DeviceId:    "75a3",
		Name:        "Instinct MI355X", Chip: "Aqua Vanjaram",
		ChipsOnDevice: 1,
		ComputeBlock:  256,
		VectorLanes:   16384, MatrixEngines: 1024, ScalarUnit: 256,
		PeakVectorTflops: 157.3,
		PeakMatrixTops:   10100,
		MemorySizeGB:     288, MemoryType: "HBM3E", MemoryBusBit: 8192, Ecc: true,
		BandwidthGBs: 8000,
		Fp32Tflops:   157.3, Fp64Tflops: 78.6,
		Fp16Tflops: 2500, Bf16Tflops: 2500,
		Fp8Tflops: 5000, Fp6Tflops: 10100, Fp4Tflops: 10100, Int8Tops: 5000,
		TdpW: 1400, FormFactor: "OAM", PcieGen: "5.0", PcieLanes: 16,
		Interconnect: "Infinity Fabric", InterconnectBandwidthGBs: 1008,
		MigSupported: true, MaxVfs: 8,
		ReleaseDate: "2025-06-12",
	},
}
