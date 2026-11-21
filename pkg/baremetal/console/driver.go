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
	"yunion.io/x/onecloud/pkg/util/redfish/bmconsole"
)

// ConsoleJNLPInput is the BMC identity used to fetch a console JNLP.
type ConsoleJNLPInput struct {
	Host         string
	Username     string
	Password     string
	SerialNumber string
	Model        string
}

// IConsoleJNLPDriver fetches a vendor BMC Java console JNLP.
type IConsoleJNLPDriver interface {
	GetName() string
	Match(manufacture string) bool
	GetConsoleJNLP(ctx context.Context, input *ConsoleJNLPInput) (string, error)
}

// IConsoleHTML5Driver fetches a URL that opens the vendor BMC HTML5 console.
type IConsoleHTML5Driver interface {
	GetHTML5Console(ctx context.Context, input *ConsoleJNLPInput) (string, error)
}

var consoleDrivers []IConsoleJNLPDriver

func RegisterDriver(drv IConsoleJNLPDriver) {
	consoleDrivers = append(consoleDrivers, drv)
}

// GetDriver returns the console driver that handles manufacture.
func GetDriver(manufacture string) IConsoleJNLPDriver {
	for _, drv := range consoleDrivers {
		if drv.Match(manufacture) {
			return drv
		}
	}
	return nil
}

func matchOem(manufacture string, names ...string) bool {
	oem := types.ManufactureOemName(manufacture)
	for _, name := range names {
		if oem == name {
			return true
		}
	}
	return false
}

func newBMCConsole(input *ConsoleJNLPInput) *bmconsole.SBMCConsole {
	return bmconsole.NewBMCConsole(input.Host, input.Username, input.Password, false)
}
