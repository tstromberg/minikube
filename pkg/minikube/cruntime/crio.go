/*
Copyright 2019 The Kubernetes Authors All rights reserved.

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

package cruntime

import (
	"fmt"

	"github.com/golang/glog"
)

// CRIO contains CRIO runtime state
type CRIO struct {
	Socket string
	Runner CommandRunner
}

// Name is a human readable name for CRIO
func (r *CRIO) Name() string {
	return "CRIO"
}

// SocketPath returns the path to the socket file for CRIO
func (r *CRIO) SocketPath() string {
	if r.Socket != "" {
		return r.Socket
	}
	return "/var/run/crio/crio.sock"
}

// Available returns an error if it is not possible to use this runtime on a host
func (r *CRIO) Available() error {
	return r.Runner.Run("command -v crio")
}

// Active returns if CRIO is active on the host
func (r *CRIO) Active() bool {
	err := r.Runner.Run("systemctl is-active --quiet service crio")
	return err == nil
}

// Enable idempotently enables CRIO on a host
func (r *CRIO) Enable() error {
	if err := disableOthers(r, r.Runner); err != nil {
		glog.Warningf("disableOthers: %v", err)
	}
	if err := populateCRIConfig(r.Runner, r.SocketPath()); err != nil {
		return err
	}
	if err := enableIPForwarding(r.Runner); err != nil {
		return err
	}
	return r.Runner.Run("sudo systemctl restart crio")
}

// Disable idempotently disables CRIO on a host
func (r *CRIO) Disable() error {
	return r.Runner.Run("sudo systemctl stop crio")
}

// LoadImage loads an image into this runtime
func (r *CRIO) LoadImage(path string) error {
	// This should use ctr via pullImageCRI once we sort out why api.v1.CRIPluginService is unimplemented.
	return r.Runner.Run(fmt.Sprintf("sudo podman load -i %s", path))

}

// KubeletOptions returns kubelet options for a runtime.
func (r *CRIO) KubeletOptions() map[string]string {
	return map[string]string{
		"container-runtime":          "remote",
		"container-runtime-endpoint": r.SocketPath(),
		"image-service-endpoint":     r.SocketPath(),
		"runtime-request-timeout":    "15m",
	}
}

// ListContainers returns a list of managed by this container runtime
func (r *CRIO) ListContainers(filter string) ([]string, error) {
	return listCRIContainers(r.Runner, filter)
}

// KillContainers removes containers based on ID
func (r *CRIO) KillContainers(ids []string) error {
	return killCRIContainers(r.Runner, ids)
}

// StopContainers stops containers based on ID
func (r *CRIO) StopContainers(ids []string) error {
	return stopCRIContainers(r.Runner, ids)
}
