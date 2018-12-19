// +build iso

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
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestISO(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	mk := NewMinikubeRunner()
	mk.Run(ctx, "delete")
	mk.MustRun(ctx, mk.StartArgs())

	t.Run("permissions", testMountPermissions)
	t.Run("binaries", testInstalledBinaries)
	t.Run("persistence", testMountPersistence)
}

func testMountPermissions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mk := NewMinikubeRunner()
	// test mount permissions
	mountPoints := []string{"/Users", "/hosthome"}
	perms := "drwxr-xr-x"
	foundMount := false

	for _, dir := range mountPoints {
		stdout, stderr, err := mk.SSH(fmt.Sprintf("ls -l %s", dir))
		if err != nil {
			continue
		}
		foundMount = true
		if !strings.Contains(output, perms) {
			t.Fatalf("Incorrect permissions. Expected %s, got %s.", perms, output)
		}
	}
	if !foundMount {
		t.Fatalf("No shared mount found. Checked %s", mountPoints)
	}
}

func testInstalledBinaries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	minikubeRunner := NewMinikubeRunner(t)

	packages := []string{
		"git",
		"rsync",
		"curl",
		"wget",
		"socat",
		"iptables",
		"VBoxControl",
		"VBoxService",
	}

	for _, pkg := range packages {
		if stdout, stderr, err := minikubeRunner.SSH(fmt.Sprintf("which %s", pkg)); err != nil {
			t.Errorf("Error finding package: %s. Error: %v. Output: %s", pkg, err, output)
		}
	}

}

func testMountPersistence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	minikubeRunner := NewMinikubeRunner(t)

	for _, dir := range []string{
		"/data",
		"/var/lib/docker",
		"/var/lib/cni",
		"/var/lib/kubelet",
		"/var/lib/minikube",
		"/var/lib/rkt",
		"/var/lib/boot2docker",
	} {
		output, err := minikubeRunner.SSH(fmt.Sprintf("df %s | tail -n 1 | awk '{print $1}'", dir))
		if err != nil {
			t.Errorf("Error checking device for %s. Error: %v", dir, err)
		}
		if !strings.Contains(output, "/dev/sda1") {
			t.Errorf("Path %s is not mounted persistently. %s", dir, output)
		}
	}
}
