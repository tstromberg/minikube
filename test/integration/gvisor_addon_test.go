// +build integration

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

package integration

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGvisorAddon(t *testing.T) {
	if NoneDriver() {
		t.Skip("Can't run containerd backend with none driver")
	}
	MaybeSlowParallel(t)

	profile := UniqueProfileName("gvisor")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer func() {
		if t.Failed() {
			rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "logs", "gvisor", "-n", "kube-system"))
			if err != nil {
				t.Logf("failed to get gvisor post-mortem logs: %v", err)
			}
			t.Logf("gvisor post-mortem: %s:\n%s\n", rr.Command(), rr.Output())
		}
		CleanupWithLogs(t, profile, cancel)
	}()

	startArgs := append([]string{"start", "-p", profile, "--container-runtime=containerd", "--docker-opt", "containerd=/var/run/containerd/containerd.sock", "--wait=false"}, StartArgs()...)
	rr, err := Run(t, exec.CommandContext(ctx, Target(), startArgs...))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Args, err)
	}

	// TODO: Re-examine if we should be pulling in an image which users don't normally invoke
	rr, err = Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "cache", "add", "gcr.io/k8s-minikube/gvisor-addon:latest"))
	if err != nil {
		t.Errorf("%s failed: %v", rr.Args, err)
	}

	// NOTE: addons are global, but the addon must assert that the runtime is containerd
	rr, err = Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "addons", "enable", "gvisor"))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Args, err)
	}

	// Because addons are persistent across profiles :(
	defer func() {
		rr, err := Run(t, exec.Command(Target(), "-p", profile, "addons", "disable", "gvisor"))
		if err != nil {
			t.Logf("%s failed: %v", rr.Args, err)
		}
	}()

	if _, err := PodWait(ctx, t, profile, "kube-system", "kubernetes.io/minikube-addons=gvisor", 2*time.Minute); err != nil {
		t.Fatalf("waiting for gvisor controller to be up: %v", err)
	}

	// Create an untrusted workload
	rr, err = Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "replace", "--force", "-f", filepath.Join(*testdataDir, "nginx-untrusted.yaml")))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Args, err)
	}
	// Create gvisor workload
	rr, err = Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "replace", "--force", "-f", filepath.Join(*testdataDir, "nginx-gvisor.yaml")))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Args, err)
	}

	if _, err := PodWait(ctx, t, profile, "default", "run=nginx,untrusted=true", 2*time.Minute); err != nil {
		t.Errorf("nginx: %v", err)
	}
	if _, err := PodWait(ctx, t, profile, "default", "run=nginx,runtime=gvisor", 2*time.Minute); err != nil {
		t.Errorf("nginx: %v", err)
	}

	// Ensure that workloads survive a restart
	rr, err = Run(t, exec.CommandContext(ctx, Target(), "stop", "-p", profile))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Args, err)
	}

	rr, err = Run(t, exec.CommandContext(ctx, Target(), startArgs...))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Args, err)
	}
	if _, err := PodWait(ctx, t, profile, "kube-system", "kubernetes.io/minikube-addons=gvisor", 2*time.Minute); err != nil {
		t.Errorf("waiting for gvisor controller to be up: %v", err)
	}
	if _, err := PodWait(ctx, t, profile, "default", "run=nginx,untrusted=true", 2*time.Minute); err != nil {
		t.Errorf("nginx: %v", err)
	}
	if _, err := PodWait(ctx, t, profile, "default", "run=nginx,runtime=gvisor", 2*time.Minute); err != nil {
		t.Errorf("nginx: %v", err)
	}
}
