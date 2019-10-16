/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package constants

import (
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/minikube/pkg/minikube/localpath"
)

const (
	// DefaultKubernetesVersion is the default kubernetes version
	DefaultKubernetesVersion = "v1.16.1"
	// NewestKubernetesVersion is the newest Kubernetes version to test against
	NewestKubernetesVersion = "v1.16.1"
	// OldestKubernetesVersion is the oldest Kubernetes version to test against
	OldestKubernetesVersion = "v1.11.10"

	// APIServerPort is the default API server port
	APIServerPort = 8443
	// APIServerName is the default API server name
	APIServerName = "minikubeCA"
	// ClusterDNSDomain is the default DNS domain
	ClusterDNSDomain = "cluster.local"

	// DefaultMachineName is the default name for the VM
	DefaultMachineName = "minikube"
	// DefaultNodeName is the default name for the kubeadm node within the VM
	DefaultNodeName = "minikube"

	// MountProcessFileName is the filename of the mount process
	MountProcessFileName = ".mount-process"

	// IsMinikubeChildProcess is the name of "is minikube child process" variable
	IsMinikubeChildProcess = "IS_MINIKUBE_CHILD_PROCESS"

	// DriverMock is a mock driver.
	DriverMock = "mock-driver"
	// DriverNone is the none driver.
	DriverNone = "none"
	// DriverKvm2 is the kvm2 driver option name for in linux
	DriverKvm2 = "kvm2"
	// DriverVirtualbox is the virtualbox driver option name
	DriverVirtualbox = "virtualbox"
	// DriverHyperkit is the hyperkit driver option name for mac os
	DriverHyperkit = "hyperkit"
	// DriverVmware is the vmware driver option name
	DriverVmware = "vmware"
	// DriverVmwareFusion is the vmware fusion driver option
	DriverVmwareFusion = "vmwarefusion"
	// DriverHyperv is the hyperv driver option for windows
	DriverHyperv = "hyperv"
	// DriverParallels is the parallels driver option name
	DriverParallels = "parallels"

	// GvisorFilesPath is the path to the gvisor files saved by go-bindata
	GvisorFilesPath = "/tmp/gvisor"
	// GvisorConfigTomlTargetName is the go-bindata target name for the gvisor config.toml
	GvisorConfigTomlTargetName = "gvisor-config.toml"
)

var (
	// DefaultMinipath is the default Minikube path (under the home directory)
	DefaultMinipath = filepath.Join(homedir.HomeDir(), ".minikube")

	// KubeconfigPath is the path to the Kubernetes client config
	KubeconfigPath = clientcmd.RecommendedHomeFile
	// KubeconfigEnvVar is the env var to check for the Kubernetes client config
	KubeconfigEnvVar = clientcmd.RecommendedConfigPathEnvVar

	// ImageRepositories contains all known image repositories
	ImageRepositories = map[string][]string{
		"global": {""},
		"cn":     {"registry.cn-hangzhou.aliyuncs.com/google_containers"},
	}

	// KubeadmBinaries are Kubernetes release binaries required for kubeadm
	KubeadmBinaries = []string{"kubelet", "kubeadm"}
	// ImageCacheDir is the path to the image cache directory
	ImageCacheDir = localpath.MakeMiniPath("cache", "images")
)
