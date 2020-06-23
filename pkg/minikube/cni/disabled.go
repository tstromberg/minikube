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

package cni

import (
	"github.com/golang/glog"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/driver"
)

// Disabled is a CNI manager than does nothing
type Disabled struct {
	cc config.ClusterConfig
}

// String returns a string representation
func (c Disabled) String() string {
	return "Disabled"
}

// Apply enables the CNI
func (c Disabled) Apply(master Runner, nodes []Runner) error {
	if driver.IsKIC(c.cc.Driver) && c.cc.KubernetesConfig.ContainerRuntime != "docker" {
		glog.Warningf("CNI is recommended for %q driver and %q runtime - expect networking issues", c.cc.Driver, c.cc.KubernetesConfig.ContainerRuntime)
	}

	if len(c.cc.Nodes) > 1 {
		glog.Warningf("CNI is recommended for multi-node clusters - expect networking issues")
	}

	return nil
}

// CIDR returns the default CIDR used by this CNI
func (c Disabled) CIDR() string {
	return ""
}
