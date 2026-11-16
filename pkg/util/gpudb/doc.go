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

// Package gpudb 是 GPU/NPU 参考规格目录。
//
// GetGPUInfo 的前两个参数是 PCI VendorId 和 DeviceId，4 位十六进制，不含 0x，例如 10de、20b2。
// 不接受营销名。memSizeGB 为 0 时取该 ID 下显存最大的一条；chipsOnDevice 为 0 时取该显存下芯片数最少的一条。
// GetGPUInfo2 的参数是 vendorId:deviceId，例如 10de:20b2，容量和芯片数都按 0 处理。
//
// 数值 0、空字符串、布尔 false 表示未给出或不适用。
// SchedulingExplicitTile 的 false 除外，它表示硬件隐藏延迟。
// MemorySizeGB 是规格标称容量，单位 GB，按 1GB = 1024MiB 换算，80GB 记 80。
// 同一 VendorId + DeviceId 可以有多条，靠 ChipsOnDevice 和 MemorySizeGB 区分，四项合在一起全局唯一。
// JSON 保留零值，不使用 omitempty。
package gpudb // import "yunion.io/x/onecloud/pkg/util/gpudb"
