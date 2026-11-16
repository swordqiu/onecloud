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

import (
	"fmt"
	"sort"
	"strings"

	"yunion.io/x/pkg/errors"
)

// GPUInfos 按 NVIDIA、AMD、华为、昆仑芯、天数智芯、瀚博、英特尔、海光、壁仞、摩尔线程、平头哥、寒武纪、燧原、沐曦的顺序合并各厂商规格。
// 具体型号在 nvidia.go、amd.go、huawei.go、kunlunxin.go、iluvatar.go、vastaitech.go、intel.go、hygon.go、biren.go、mthreads.go、thead.go、cambricon.go、enflame.go、metax.go。
//
// 没写出的数值为 0、字符串为空、布尔为 false，表示未给出或不适用，见对应条目的注释。
// 同一营销名有多档容量或板型时，只收最常见的一档，区别写在 Name 里。
// DeviceId 用 pci.ids 里该型号的主 ID。同一 VendorId + DeviceId 可以有多条。
//
// 目录只读。GetGPUInfo 返回副本。
var GPUInfos []*SGPUInfo
var gpuInfosMap map[string]map[string][]*SGPUInfo

var gpuInfoList [][]SGPUInfo = [][]SGPUInfo{
	nvidiaGPUInfos,
	amdGPUInfos,
	huaweiGPUInfos,
	kunlunxinGPUInfos,
	iluvatarGPUInfos,
	vastaitechGPUInfos,
	intelGPUInfos,
	hygonGPUInfos,
	birenGPUInfos,
	mthreadsGPUInfos,
	theadGPUInfos,
	cambriconGPUInfos,
	enflameGPUInfos,
	metaxGPUInfos,
}

func init() {
	gpuInfosMap = make(map[string]map[string][]*SGPUInfo)
	for _, gpuInfo := range gpuInfoList {
		for i := range gpuInfo {
			rec := new(SGPUInfo)
			*rec = gpuInfo[i]
			indexGPUInfo(rec)
			GPUInfos = append(GPUInfos, rec)
		}
	}
	sortGPUInfosMap()
}

// sortGPUInfosMap 把同一 VendorId + DeviceId 的记录按 MemorySizeGB 降序、ChipsOnDevice 升序排列。
func sortGPUInfosMap() {
	for _, devices := range gpuInfosMap {
		for _, list := range devices {
			sort.Slice(list, func(i, j int) bool {
				if list[i].MemorySizeGB != list[j].MemorySizeGB {
					return list[i].MemorySizeGB > list[j].MemorySizeGB
				}
				return list[i].ChipsOnDevice < list[j].ChipsOnDevice
			})
		}
	}
}

func indexGPUInfo(gpu *SGPUInfo) {
	vendorId := strings.ToLower(strings.TrimSpace(gpu.VendorId))
	if vendorId == "" {
		return
	}
	devices := gpuInfosMap[vendorId]
	if devices == nil {
		devices = make(map[string][]*SGPUInfo)
		gpuInfosMap[vendorId] = devices
	}
	deviceId := strings.ToLower(gpu.DeviceId)
	for _, old := range devices[deviceId] {
		if old.ChipsOnDevice == gpu.ChipsOnDevice && old.MemorySizeGB == gpu.MemorySizeGB {
			panic(fmt.Sprintf("gpudb: duplicate %s:%s chips=%d memory=%dGB (%s, %s)",
				vendorId, deviceId, gpu.ChipsOnDevice, gpu.MemorySizeGB, old.Name, gpu.Name))
		}
	}
	devices[deviceId] = append(devices[deviceId], gpu)
}

// GetGPUInfo 按 PCI VendorId 和 DeviceId 查询。vendorId 是 4 位十六进制，不含 0x，大小写不敏感。
// 不接受营销名。memSizeGB 为 0 时，在该 ID 的记录里取 MemorySizeGB 最大的一档。
// chipsOnDevice 为 0 时，在上述显存档里取 ChipsOnDevice 最小的一条。返回的是副本。
func GetGPUInfo(vendorId string, deviceId string, memSizeGB uint, chipsOnDevice uint) *SGPUInfo {
	vendorId = strings.ToLower(strings.TrimSpace(vendorId))
	deviceId = strings.ToLower(deviceId)
	devices, ok := gpuInfosMap[vendorId]
	if !ok {
		return nil
	}
	list := devices[deviceId]
	if len(list) == 0 {
		return nil
	} else if len(list) == 1 {
		copied := *list[0]
		return &copied
	}
	if memSizeGB >= list[0].MemorySizeGB || memSizeGB < list[len(list)-1].MemorySizeGB {
		memSizeGB = list[0].MemorySizeGB
	}
	var matched []*SGPUInfo
	for i := range list {
		if (i == 0 && memSizeGB >= list[i].MemorySizeGB) || (i > 0 && memSizeGB < list[i-1].MemorySizeGB && memSizeGB >= list[i].MemorySizeGB) {
			matched = append(matched, list[i])
		}
	}
	if len(matched) == 0 {
		return nil
	} else if len(matched) == 1 {
		copied := *matched[0]
		return &copied
	}
	if chipsOnDevice <= matched[0].ChipsOnDevice || chipsOnDevice > matched[len(matched)-1].ChipsOnDevice {
		chipsOnDevice = matched[0].ChipsOnDevice
	}
	for i := range matched {
		if (i == 0 && chipsOnDevice <= matched[i].ChipsOnDevice) || (i > 0 && chipsOnDevice > matched[i-1].ChipsOnDevice && chipsOnDevice <= matched[i].ChipsOnDevice) {
			copied := *matched[i]
			return &copied
		}
	}
	return nil
}

// GetGPUInfo2 解析 vendorId:deviceId，例如 10de:20b2、19e5:d500。容量和芯片数都按 0 处理。
func GetGPUInfo2(vendorDeviceId string, memSizeGB uint, chipsOnDevice uint) (*SGPUInfo, error) {
	vendorId, deviceId, ok := strings.Cut(vendorDeviceId, ":")
	if !ok {
		return nil, errors.Wrapf(errors.ErrInvalidFormat, "invalid vendorDeviceId: %s", vendorDeviceId)
	}
	return GetGPUInfo(vendorId, deviceId, memSizeGB, chipsOnDevice), nil
}
