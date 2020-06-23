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

// Package cni configures the Container Networking Interface
package cni

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/minikube/assets"
	"k8s.io/minikube/pkg/minikube/command"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/vmpath"
)

const (
	// DefaultPodCIDR is the default CIDR to use in minikube CNI's.
	DefaultPodCIDR = "10.244.0.0/16"
)

// Runner is the subset of command.Runner this package consumes
type Runner interface {
	RunCmd(cmd *exec.Cmd) (*command.RunResult, error)
	Copy(assets.CopyableFile) error
}

// Manager is a common interface for CNI
type Manager interface {
	// Enable enables the CNI
	Apply(Runner, []Runner) error

	// CIDR returns the default CIDR used by this CNI
	CIDR() string

	// String representation
	String() string
}

// tmplInputs are inputs to CNI templates
type tmplInput struct {
	ImageName    string
	PodCIDR      string
	DefaultRoute string
}

// New returns a new CNI manager
func New(cc config.ClusterConfig) (Manager, error) {
	if cc.KubernetesConfig.NetworkPlugin != "" && cc.KubernetesConfig.NetworkPlugin != "cni" {
		glog.Infof("network plugin configured as %q, returning disabled", cc.KubernetesConfig.NetworkPlugin)
		return Disabled{}, nil
	}

	glog.Infof("Creating CNI manager for %q", cc.KubernetesConfig.CNI)

	switch cc.KubernetesConfig.CNI {
	case "", "auto":
		return chooseDefault(cc), nil
	case "false":
		return Disabled{cc: cc}, nil
	case "kindnet", "true":
		return KindNet{cc: cc}, nil
	case "bridge":
		return Bridge{cc: cc}, nil
	case "flannel":
		return Flannel{cc: cc}, nil
	default:
		return NewCustom(cc, cc.KubernetesConfig.CNI)
	}
}

func chooseDefault(cc config.ClusterConfig) Manager {
	// For backwards compatibility with older profiles using --enable-default-cni
	if cc.KubernetesConfig.EnableDefaultCNI {
		glog.Infof("EnableDefaultCNI is true, recommending bridge")
		return Bridge{}
	}

	if cc.KubernetesConfig.ContainerRuntime != "docker" {
		if driver.IsKIC(cc.Driver) {
			glog.Infof("%q driver + %s runtime found, recommending kindnet", cc.Driver, cc.KubernetesConfig.ContainerRuntime)
			return KindNet{cc: cc}
		}
		glog.Infof("%q driver + %s runtime found, recommending bridge", cc.Driver, cc.KubernetesConfig.ContainerRuntime)
		return Bridge{cc: cc}
	}

	if len(cc.Nodes) > 1 {
		glog.Infof("%d nodes found, recommending kindnet", len(cc.Nodes))
		return KindNet{cc: cc}
	}

	glog.Infof("CNI unnecessary in this configuration, recommending no CNI")
	return Disabled{}
}

// manifestPath returns the path to the CNI manifest
func manifestPath() string {
	return path.Join(vmpath.GuestEphemeralDir, "cni.yaml")
}

// manifestAsset returns a copyable asset for the CNI manifest
func manifestAsset(b []byte) assets.CopyableFile {
	return assets.NewMemoryAssetTarget(b, manifestPath(), "0644")
}

// kubectlPath returns the path to the kubelet
func kubectlPath(cc config.ClusterConfig) string {
	return path.Join(vmpath.GuestPersistentDir, "binaries", cc.KubernetesConfig.KubernetesVersion, "kubectl")
}

// applyManifest applies a CNI manifest
func applyManifest(cc config.ClusterConfig, r Runner, f assets.CopyableFile) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	kubectl := kubectlPath(cc)
	glog.Infof("applying CNI manifest using %s ...", kubectl)

	if err := r.Copy(f); err != nil {
		return errors.Wrapf(err, "copy")
	}

	cmd := exec.CommandContext(ctx, "sudo", kubectl, "apply", fmt.Sprintf("--kubeconfig=%s", path.Join(vmpath.GuestPersistentDir, "kubeconfig")), "-f", manifestPath())
	if rr, err := r.RunCmd(cmd); err != nil {
		return errors.Wrapf(err, "cmd: %s output: %s", rr.Command(), rr.Output())
	}

	return nil
}

// applyNetConf applies a netconf file across nodes
func applyNetConf(rs []Runner, f assets.CopyableFile) error {
	for _, r := range rs {
		if err := r.Copy(f); err != nil {
			return errors.Wrapf(err, "copy")
		}
	}

	return nil
}
