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

package console

import (
	"testing"

	"yunion.io/x/onecloud/pkg/cloudcommon/types"
)

func TestGetDriver(t *testing.T) {
	cases := []struct {
		manufacture string
		name        string
	}{
		{"hp", types.OEM_NAME_HP},
		{"HP", types.OEM_NAME_HP},
		{"hpe", types.OEM_NAME_HP},
		{"HPE", types.OEM_NAME_HP},
		{"dell", types.OEM_NAME_DELL},
		{"Dell Inc.", types.OEM_NAME_DELL},
		{"supermicro", types.OEM_NAME_SUPERMICRO},
		{"Supermicro", types.OEM_NAME_SUPERMICRO},
		{"lenovo", types.OEM_NAME_LENOVO},
		{"Lenovo", types.OEM_NAME_LENOVO},
		{"huawei", types.OEM_NAME_HUAWEI},
		{"Huawei Technologies Co., Ltd.", types.OEM_NAME_HUAWEI},
		{"h3c", types.OEM_NAME_H3C},
		{"New H3C Technologies", types.OEM_NAME_H3C},
		{"sugon", types.OEM_NAME_SUGON},
		{"Sugon", types.OEM_NAME_SUGON},
		{"xfusion", types.OEM_NAME_XFUSION},
		{"xFusion Digital Technologies Co., Ltd.", types.OEM_NAME_XFUSION},
		{"zte", types.OEM_NAME_ZTE},
		{"ZTE Corporation", types.OEM_NAME_ZTE},
	}
	for _, c := range cases {
		drv := GetDriver(c.manufacture)
		if drv == nil {
			t.Errorf("manufacture %q: driver is nil, want %s", c.manufacture, c.name)
			continue
		}
		if drv.GetName() != c.name {
			t.Errorf("manufacture %q: got driver %s, want %s", c.manufacture, drv.GetName(), c.name)
		}
	}

	for _, manufacture := range []string{"", "inspur"} {
		if drv := GetDriver(manufacture); drv != nil {
			t.Errorf("manufacture %q: got driver %s, want nil", manufacture, drv.GetName())
		}
	}
}
