/*
Copyright 2020 The Kubernetes Authors All rights reserved.

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

package machine

import (
	"io/ioutil"
	"os/exec"

	"github.com/docker/machine/libmachine/provision"
	"github.com/golang/glog"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"k8s.io/minikube/pkg/minikube/command"
	"k8s.io/minikube/pkg/minikube/out"
)

type hostInfo struct {
	Memory   int64
	CPUs     int
	DiskSize int64
}

func megs(bytes uint64) int64 {
	return int64(bytes / 1024 / 1024)
}

func getHostInfo() (*hostInfo, error) {
	i, err := cpu.Info()
	if err != nil {
		glog.Warningf("Unable to get CPU info: %v", err)
		return nil, err
	}
	v, err := mem.VirtualMemory()
	if err != nil {
		glog.Warningf("Unable to get mem info: %v", err)
		return nil, err
	}
	d, err := disk.Usage("/")
	if err != nil {
		glog.Warningf("Unable to get disk info: %v", err)
		return nil, err
	}

	var info hostInfo
	info.CPUs = len(i)
	info.Memory = megs(v.Total)
	info.DiskSize = megs(d.Total)
	return &info, nil
}

// showLocalOsRelease shows systemd information about the current linux distribution, on the local host
func showLocalOsRelease() {
	osReleaseOut, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		glog.Errorf("ReadFile: %v", err)
		return
	}

	osReleaseInfo, err := provision.NewOsRelease(osReleaseOut)
	if err != nil {
		glog.Errorf("NewOsRelease: %v", err)
		return
	}

	out.T(out.Provisioner, "OS release is {{.pretty_name}}", out.V{"pretty_name": osReleaseInfo.PrettyName})
}

// logRemoteOsRelease shows systemd information about the current linux distribution, on the remote VM
func logRemoteOsRelease(r command.Runner) {
	rr, err := r.RunCmd(exec.Command("cat", "/etc/os-release"))
	if err != nil {
		glog.Infof("remote release failed: %v", err)
	}

	osReleaseInfo, err := provision.NewOsRelease(rr.Stdout.Bytes())
	if err != nil {
		glog.Errorf("NewOsRelease: %v", err)
		return
	}

	glog.Infof("Remote host: %s", osReleaseInfo.PrettyName)
}
