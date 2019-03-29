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

package command

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/coredns/coredns/plugin/pkg/log"

	"yunion.io/x/jsonutils"

	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/util/ansible"
	o "yunion.io/x/onecloud/pkg/webconsole/options"
)

type SSHtoolSol struct {
	*BaseCommand
	IP       string
	Username string
	reTry    int
	showInfo string
	keyFile  string
}

func getCommand(ctx context.Context, userCred mcclient.TokenCredential, ip string) (string, *BaseCommand, error) {
	cmd := NewBaseCommand(o.Options.SshToolPath)
	if !o.Options.EnableAutoLogin {
		return "", nil, nil
	}
	s := auth.GetAdminSession(ctx, o.Options.Region, "v2")
	key, err := modules.Sshkeypairs.GetById(s, userCred.GetProjectId(), jsonutils.Marshal(map[string]bool{"admin": true}))
	if err != nil {
		return "", nil, err
	}
	file, err := ioutil.TempFile("", fmt.Sprintf("id_rsa.%s.", ip))
	if err != nil {
		return "", nil, err
	}
	privKey, err := key.GetString("private_key")
	if err != nil {
		return "", nil, err
	}
	_, err = file.Write([]byte(privKey))
	if err != nil {
		return "", nil, err
	}
	file.Close()
	err = os.Chmod(file.Name(), 0700)
	if err != nil {
		return "", nil, err
	}
	cmd.AppendArgs("-i", file.Name())
	cmd.AppendArgs("-q")
	cmd.AppendArgs("-o", "StrictHostKeyChecking=no")
	cmd.AppendArgs("-o", "GlobalKnownHostsFile=/dev/null")
	cmd.AppendArgs("-o", "UserKnownHostsFile=/dev/null")
	cmd.AppendArgs("-o", "PasswordAuthentication=no")
	cmd.AppendArgs(fmt.Sprintf("%s@%s", ansible.PUBLIC_CLOUD_ANSIBLE_USER, ip))
	return file.Name(), cmd, nil
}

func NewSSHtoolSolCommand(ctx context.Context, userCred mcclient.TokenCredential, ip string) (*SSHtoolSol, error) {
	if conn, err := net.DialTimeout("tcp", ip+":22", time.Second*2); err != nil {
		return nil, fmt.Errorf("IPAddress %s not accessable", ip)
	} else {
		conn.Close()

		keyFile, cmd, err := getCommand(ctx, userCred, ip)
		if err != nil {
			log.Errorf("getCommand error: %v", err)
		}

		return &SSHtoolSol{
			BaseCommand: cmd,
			IP:          ip,
			Username:    "",
			reTry:       0,
			showInfo:    fmt.Sprintf("%s login: ", ip),
			keyFile:     keyFile,
		}, nil
	}
}

func (c *SSHtoolSol) GetCommand() *exec.Cmd {
	if c.BaseCommand != nil {
		cmd := c.BaseCommand.GetCommand()
		cmd.Env = append(cmd.Env, "TERM=xterm-256color")
		return cmd
	}
	return nil
}

func (c *SSHtoolSol) Cleanup() error {
	if len(c.keyFile) > 0 {
		os.Remove(c.keyFile)
		c.keyFile = ""
	}
	return nil
}

func (c *SSHtoolSol) GetProtocol() string {
	return PROTOCOL_TTY
}

func (c *SSHtoolSol) Connect() error {
	conn, err := net.DialTimeout("tcp", c.IP+":22", time.Second*2)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

func (c *SSHtoolSol) GetData(data string) (isShow bool, ouput string, command string) {
	if len(c.Username) == 0 {
		if len(data) == 0 {
			//用户名不能为空
			return true, c.showInfo, ""
		}
		c.Username = data
		return false, "Password:", ""
	}
	return true, "", fmt.Sprintf("%s -p %s %s -oGlobalKnownHostsFile=/dev/null -oUserKnownHostsFile=/dev/null -oStrictHostKeyChecking=no %s@%s", o.Options.SshpassToolPath, data, o.Options.SshToolPath, c.Username, c.IP)
}

func (c *SSHtoolSol) ShowInfo() string {
	c.Username = ""
	c.reTry++
	if c.reTry == 3 {
		c.reTry = 0
		//清屏
		time.Sleep(1 * time.Second)
		return "\033c " + c.showInfo
	}
	return c.showInfo
}
