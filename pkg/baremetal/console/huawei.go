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
	"context"

	"yunion.io/x/onecloud/pkg/cloudcommon/types"
	"yunion.io/x/onecloud/pkg/httperrors"
)

type sHuaweiConsoleDriver struct{}

func init() {
	RegisterDriver(&sHuaweiConsoleDriver{})
}

func (d *sHuaweiConsoleDriver) GetName() string {
	return types.OEM_NAME_HUAWEI
}

func (d *sHuaweiConsoleDriver) Match(manufacture string) bool {
	return matchOem(manufacture, types.OEM_NAME_HUAWEI)
}

func (d *sHuaweiConsoleDriver) GetConsoleJNLP(ctx context.Context, input *ConsoleJNLPInput) (string, error) {
	return runIbmcRedfishConsole(ctx, d.GetName(), input, probeJNLP)
}

func (d *sHuaweiConsoleDriver) GetHTML5Console(ctx context.Context, input *ConsoleJNLPInput) (string, error) {
	return "", httperrors.NewNotImplementedError("iBMC ExportKvmStartupFile only returns a JNLP file")
}
