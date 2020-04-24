// +build windows

/*
Copyright 2018 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hyperv

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/machine/drivers/hyperv"
	"github.com/docker/machine/libmachine/drivers"
	"github.com/pkg/errors"

	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/download"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/registry"
)

const (
	docURL                    = "https://minikube.sigs.k8s.io/docs/reference/drivers/hyperv/"
	defaultExternalSwitchName = "minikube"
)

func init() {
	if err := registry.Register(registry.DriverDef{
		Name:     driver.HyperV,
		Init:     func() drivers.Driver { return hyperv.NewDriver("", "") },
		Config:   configure,
		Status:   status,
		Priority: registry.Preferred,
	}); err != nil {
		panic(fmt.Sprintf("register: %v", err))
	}
}

func configure(cfg config.ClusterConfig, n config.Node) (interface{}, error) {
	d := hyperv.NewDriver(driver.MachineName(cfg, n), localpath.MiniPath())
	d.Boot2DockerURL = download.LocalISOResource(cfg.MinikubeISO)
	d.VSwitch = cfg.HypervVirtualSwitch
	if d.VSwitch == "" && cfg.HypervUseExternalSwitch {
		switchName, adapter, err := chooseSwitch(cfg.HypervExternalAdapter)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to choose switch for Hyper-V driver")
		}
		if cfg.HypervExternalAdapter == "" && switchName == "" {
			// create a switch on the returned adapter
			switchName = defaultExternalSwitchName
			err := createVMSwitch(switchName, adapter)
			if err != nil {
				return "", err
			}
		}
		d.VSwitch = switchName
	}
	d.MemSize = cfg.Memory
	d.CPU = cfg.CPUs
	d.DiskSize = cfg.DiskSize
	d.SSHUser = "docker"
	d.DisableDynamicMemory = true // default to disable dynamic memory as minikube is unlikely to work properly with dynamic memory
	return d, nil
}

func status() registry.State {
	path, err := exec.LookPath("powershell")
	if err != nil {
		return registry.State{Error: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "@(Get-Wmiobject Win32_ComputerSystem).HypervisorPresent")
	out, err := cmd.CombinedOutput()
	if string(out) != "True\r\n" {
		errorMessage := fmt.Errorf("%s failed:\n%s", strings.Join(cmd.Args, " "), out)
		fixMessage := "Start PowerShell as Administrator, and run: 'Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All'"

		// If timed out, prompt different error and suggestion messages
		// See https://github.com/kubernetes/minikube/issues/6579
		if ctx.Err() != nil {
			errorMessage = fmt.Errorf("%s exited unexpectedly:\n%s", strings.Join(cmd.Args, " "), ctx.Err())
			fixMessage = "If you have Hyper-V configured correctly, please try start again with `--force` specified"
		}
		return registry.State{Installed: false, Error: errorMessage, Fix: fixMessage, Doc: docURL}
	}
	return registry.State{Installed: true, Healthy: true}
}
