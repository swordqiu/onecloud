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

// SGPUVendorInfo 是同一厂商共用的字段，由 SGPUGenInfo 嵌入。
//
// SchedulingExplicitTile 除外：false 表示硬件用 warp/wavefront 隐藏延迟，不是“未给出”。
type SGPUVendorInfo struct {
	// Vendor、VendorId 来自官方与 PCI ID 库。VendorId 为 4 位小写十六进制，不含 0x，例如 "10de"。
	Vendor   string `json:"vendor"`
	VendorId string `json:"vendor_id"`
	// Country 是厂商所在国家，ISO 3166-1 alpha-2。NVIDIA、AMD、Intel 为 "US"，华为、寒武纪为 "CN"。
	Country string `json:"country"`
	// ManageTool 是管理工具命令名，不含路径。NVIDIA 为 "nvidia-smi"，AMD Instinct 为 "rocm-smi"，昇腾为 "npu-smi"。
	ManageTool string `json:"manage_tool"`
	// ComputeBlockName 是最小调度单元的厂商叫法，如 "SM"、"CU"、"AI Core"、"Xe Core"。
	ComputeBlockName string `json:"compute_block_name"`
	// SchedulingWidth 是一次调度的线程数。NVIDIA 的 warp 为 32，CDNA 的 wavefront 为 64。
	// 昇腾没有 warp，按 block/task 调度，记 0。同一厂商如果有两套宽度，不要共用这一份。
	SchedulingWidth int `json:"scheduling_width"`
	// SchedulingExplicitTile 为 true 时，要显式排块、搬运和同步，例如昇腾。
	// false 表示硬件用 warp/wavefront 隐藏延迟。这个 false 是有效取值。
	SchedulingExplicitTile bool `json:"scheduling_explicit_tile"`
	// VectorPeakPrecision 是该厂商向量峰值的精度。NVIDIA、AMD 为 "fp32"，昇腾可能是 "fp16"。
	VectorPeakPrecision string `json:"vector_peak_precision"`
}

// SGPUGenInfo 是同一代 GPU 共用的字段，并嵌入所属厂商。
// 代际按架构和计算能力划分，例如 Ampere 的 GA10x（8.6）和 GA100（8.0）是两份。
//
// 算力口径：
//   - 官方若只公布带结构化稀疏的数字，入库前先除以稀疏倍率。TensorSparsity 记录该代支持的倍率
//     （不支持为 1，NVIDIA 2:1 结构化稀疏为 2），稀疏峰值 = 稠密值 × TensorSparsity。
//     B300 的 NVFP4 例外：稀疏峰值是 18 PFLOPS，不是稠密 14 PFLOPS 的 2 倍。其余精度仍按这个倍率。
//   - MatrixPeakPrecision 是该代 AI 推理主指标的精度。
//     Blackwell 用 fp4，Hopper、Ada、MI300 用 fp8，Ampere 和 Turing 用 int8，Volta 用 fp16。
//     没有矩阵单元时留空。fp 类的单位是 TFLOPS，int8 的单位是 TOPS。
type SGPUGenInfo struct {
	SGPUVendorInfo

	// Arch、ComputeCap 来自 NVIDIA Compute Capability 表 / CANN 文档。
	// Arch 如 "Hopper"、"Blackwell"、"CDNA3"、"Da Vinci"。
	// ComputeCap 保持字符串：NVIDIA 为 "9.0"，AMD 为 "gfx942"，昇腾为对应 SoC 版本。
	Arch       string `json:"arch"`
	ComputeCap string `json:"compute_cap"`
	// TensorSparsity 是结构化稀疏倍率。1 表示只计稠密；2 表示支持 2:1 稀疏。
	TensorSparsity float64 `json:"tensor_sparsity"`
	// MatrixPeakPrecision 是 PeakMatrixTops 的精度，如 "fp4"、"fp8"、"fp16"、"int8"。
	MatrixPeakPrecision string `json:"matrix_peak_precision"`
}

// SGPUInfo 记录一个 GPU 型号的规格。厂商和代际字段通过 SGPUGenInfo 共用。
//
// 数值字段为 0、字符串为空、布尔为 false，表示未给出或不适用，见对应条目的注释。
// SchedulingExplicitTile 的 false 除外，它表示硬件隐藏延迟，是有效取值。
//
// 算力口径：
//   - Fp32Tflops、Fp64Tflops 是 CUDA Core / Stream Processor 的稠密峰值，不含 Tensor / Matrix Core。
//   - Tf32Tflops、Fp16Tflops、Bf16Tflops、Fp8Tflops、Fp6Tflops、Fp4Tflops、Int8Tops 是 Tensor / Matrix 稠密峰值。
type SGPUInfo struct {
	SGPUGenInfo

	// DeviceId 来自 lspci / 官方 PCI ID 库，4 位小写十六进制，不含 0x，例如 "20b2"。
	// 同一 VendorId + DeviceId 可以有多条，靠 ChipsOnDevice 和 MemorySizeGB 区分。
	// 这四项合在一起全局唯一。不记录子系统 ID。
	DeviceId string `json:"device_id"`

	// Name 来自 TechPowerUp / 官方，例如 "A100-SXM4-80GB"。
	// Chip 是芯片代号，例如 "GA100"、"GH100"。同一代常有多颗芯片，如 Ampere 的 GA102 和 GA104，所以不放进代际。
	Name string `json:"name"`
	Chip string `json:"chip"`
	// ChipsOnDevice 是一块卡上的 GPU 芯片数量。单芯片为 1。
	// 同一封装里的 die、chiplet 不拆开计。规格没写明时为 0。
	ChipsOnDevice uint `json:"chips_on_device"`

	// 微架构数量都是整卡合计。0 表示该架构没有这一级，或规格没有单独公布。
	// ComputeBlock 是最小调度单元的数量，叫法见 ComputeBlockName。
	ComputeBlock int `json:"compute_block"`
	// VectorLanes 是向量 ALU 总数：CUDA Core、Stream Processor、昇腾 Vector Unit、Intel EU。
	VectorLanes int `json:"vector_lanes"`
	// MatrixEngines 是矩阵单元总数：Tensor Core、Matrix Core、昇腾 Cube、Intel XMX。
	MatrixEngines int `json:"matrix_engines"`
	// ScalarUnit 是独立标量单元总数，负责控制流和地址计算。
	// NVIDIA 不单列标量单元，记 0；AMD 每个 CU 一个，数量等于 ComputeBlock。
	ScalarUnit int `json:"scalar_unit"`

	// 显存来自官方 datasheet 与规格库。
	// MemorySizeGB 是规格标称容量，单位 GB，按 1GB = 1024MiB 换算，80GB 记 80。
	// 管理工具报出的可用容量略小时，具体 MiB 写在该条目的注释里。
	// MemoryType 如 "HBM3"、"HBM3e"、"GDDR6X"；MemoryBusBit 为位宽，例如 5120。
	MemorySizeGB uint   `json:"memory_size_gb"`
	MemoryType   string `json:"memory_type"`
	MemoryBusBit int    `json:"memory_bus_bit"`
	// Ecc 为 true 表示规格支持 ECC。
	Ecc bool `json:"ecc"`

	// BandwidthGBs 是显存带宽，单位 GB/s，来自官方 datasheet。
	BandwidthGBs float64 `json:"bandwidth_gb_s"`

	// Fp32Tflops 是稠密 FP32 峰值，单位 TFLOPS。
	Fp32Tflops float64 `json:"fp32_tflops"`
	// Fp64Tflops 是稠密 FP64 峰值，单位 TFLOPS。HPC 卡与推理卡在这里差一个数量级以上。
	Fp64Tflops float64 `json:"fp64_tflops"`

	// 以下为 Tensor / Matrix 稠密峰值。TF32 是 Ampere 之后训练常用口径，FP16 与 BF16 在部分型号上并不相等。
	Tf32Tflops float64 `json:"tf32_tflops"`
	Fp16Tflops float64 `json:"fp16_tflops"`
	Bf16Tflops float64 `json:"bf16_tflops"`
	// Blackwell、MI300 及之后，FP8 / FP6 / FP4 与 FP16 不再成固定比例，必须按官方数字单独填写。单位 TFLOPS。
	Fp8Tflops float64 `json:"fp8_tflops"`
	Fp6Tflops float64 `json:"fp6_tflops"`
	Fp4Tflops float64 `json:"fp4_tflops"`
	// Int8Tops 是 INT8 推理峰值，单位 TOPS。
	Int8Tops float64 `json:"int8_tops"`
	// PeakVectorTflops 是非矩阵单元的向量峰值，单位 TFLOPS，精度见 VectorPeakPrecision。
	// 现有 NVIDIA、AMD 型号取 FP32，数值与 Fp32Tflops 相同。
	PeakVectorTflops float64 `json:"peak_vector_tflops"`
	// PeakMatrixTops 是该型号的 AI 推理主指标，取一档稠密矩阵峰值，精度见 MatrixPeakPrecision。
	PeakMatrixTops float64 `json:"peak_matrix_tops"`

	// TdpW 是官方热设计功耗，单位瓦。
	TdpW int `json:"tdp_w"`

	// FormFactor 如 "PCIe"、"SXM"、"OAM"。PcieGen 如 "5.0"；无 PCIe 主机接口时留空，PcieLanes 为 0。
	FormFactor string `json:"form_factor"`
	PcieGen    string `json:"pcie_gen"`
	PcieLanes  int    `json:"pcie_lanes"`

	// 卡间互联。Interconnect 如 "NVLink 4.0"、"HCCS"、"Infinity Fabric"。
	// InterconnectBandwidthGBs 取官方公布的单卡聚合带宽，单位 GB/s；没有卡间互联时为 0。
	Interconnect             string  `json:"interconnect"`
	InterconnectBandwidthGBs float64 `json:"interconnect_bandwidth_gb_s"`

	// MigSupported 为 true 表示该型号支持 MIG 或同等实例切分。
	MigSupported bool `json:"mig_supported"`
	// MaxVfs 是单卡最多能同时划出的 VF 或 MIG 实例数。
	// 不支持切分，或规格没有给出个数时为 0。
	MaxVfs int `json:"max_vfs"`

	// ReleaseDate 来自规格库，格式 YYYY-MM-DD；只知年月时用 YYYY-MM。
	ReleaseDate string `json:"release_date"`
}
