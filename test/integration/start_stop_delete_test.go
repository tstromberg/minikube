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
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/machine/libmachine/state"
	"github.com/google/go-cmp/cmp"
	"k8s.io/minikube/pkg/minikube/bootstrapper/images"
	"k8s.io/minikube/pkg/minikube/constants"
)

func TestStartStop(t *testing.T) {
	MaybeParallel(t)

	t.Run("group", func(t *testing.T) {
		tests := []struct {
			name    string
			version string
			args    []string
		}{
			{"old-docker", constants.OldestKubernetesVersion, []string{
				// default is the network created by libvirt, if we change the name minikube won't boot
				// because the given network doesn't exist
				"--kvm-network=default",
				"--kvm-qemu-uri=qemu:///system",
				"--disable-driver-mounts",
				"--keep-context=false",
				"--container-runtime=docker",
			}},
			{"newest-cni", constants.NewestKubernetesVersion, []string{
				"--feature-gates",
				"ServerSideApply=true",
				"--network-plugin=cni",
				"--extra-config=kubelet.network-plugin=cni",
				"--extra-config=kubeadm.pod-network-cidr=192.168.111.111/16",
			}},
			{"containerd", constants.DefaultKubernetesVersion, []string{
				"--container-runtime=containerd",
				"--docker-opt",
				"containerd=/var/run/containerd/containerd.sock",
				"--apiserver-port=8444",
			}},
			{"crio", "v1.15.0", []string{
				"--container-runtime=crio",
				"--disable-driver-mounts",
				"--extra-config=kubeadm.ignore-preflight-errors=SystemVerification",
			}},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				MaybeParallel(t)
				WaitForStartSlot(t)

				if !strings.Contains(tc.name, "docker") && NoneDriver() {
					t.Skipf("skipping %s - incompatible with none driver", t.Name())
				}

				profile := UniqueProfileName(tc.name)
				ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
				defer CleanupWithLogs(t, profile, cancel)

				startArgs := append([]string{"start", "-p", profile, "--alsologtostderr", "-v=3", "--wait=true"}, tc.args...)
				startArgs = append(startArgs, StartArgs()...)
				startArgs = append(startArgs, fmt.Sprintf("--kubernetes-version=%s", tc.version))
				rr, err := Run(t, exec.CommandContext(ctx, Target(), startArgs...))
				if err != nil {
					// Fatal so that we may collect logs before stop/delete steps
					t.Fatalf("%s failed: %v", rr.Args, err)
				}

				// SADNESS: 0/1 nodes are available: 1 node(s) had taints that the pod didn't tolerate.
				if strings.Contains(tc.name, "cni") {
					t.Logf("WARNING: cni mode requires additional setup before pods can schedule :(")
				} else {
					// schedule a pod to assert persistence
					rr, err = Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "create", "-f", filepath.Join(*testdataDir, "busybox.yaml")))
					if err != nil {
						t.Fatalf("%s failed: %v", rr.Args, err)
					}

					names, err := PodWait(ctx, t, profile, "default", "integration-test=busybox", 4*time.Minute)
					if err != nil {
						t.Fatalf("wait: %v", err)
					}

					// Use this pod to confirm that the runtime resource limits are sane
					rr, err = Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "exec", names[0], "--", "/bin/sh", "-c", "ulimit -n"))
					if err != nil {
						t.Fatalf("ulimit: %v", err)
					}

					got, err := strconv.ParseInt(strings.TrimSpace(rr.Stdout.String()), 10, 64)
					if err != nil {
						t.Errorf("ParseInt(%q): %v", rr.Stdout.String(), err)
					}

					// Arbitrary value set by some container runtimes. If higher, apps like MySQL may make bad decisions.
					expected := int64(1048576)
					if got != expected {
						t.Errorf("'ulimit -n' returned %d, expected %d", got, expected)
					}
				}

				rr, err = Run(t, exec.CommandContext(ctx, Target(), "stop", "-p", profile, "--alsologtostderr", "-v=3"))
				if err != nil {
					t.Errorf("%s failed: %v", rr.Args, err)
				}

				got := Status(ctx, t, Target(), profile)
				if got != state.Stopped.String() {
					t.Errorf("status = %q; want = %q", got, state.Stopped)
				}

				WaitForStartSlot(t)
				rr, err = Run(t, exec.CommandContext(ctx, Target(), startArgs...))
				if err != nil {
					// Explicit fatal so that failures don't move directly to deletion
					t.Fatalf("%s failed: %v", rr.Args, err)
				}

				// Make sure that kubeadm did not need to pull in additional images
				if !NoneDriver() {
					rr, err = Run(t, exec.CommandContext(ctx, Target(), "ssh", "-p", profile, "sudo crictl images -o json"))
					if err != nil {
						t.Errorf("%s failed: %v", rr.Args, err)
					}
					jv := map[string][]struct {
						Tags []string `json:"repoTags"`
					}{}
					err = json.Unmarshal(rr.Stdout.Bytes(), &jv)
					if err != nil {
						t.Errorf("images unmarshal: %v", err)
					}
					gotImages := []string{}
					for _, img := range jv["images"] {
						for _, i := range img.Tags {
							// Ignore non-Kubernetes images
							if !strings.Contains(i, "ingress") && !strings.Contains(i, "busybox") {
								// Remove docker.io for naming consistency between container runtimes
								gotImages = append(gotImages, strings.TrimPrefix(i, "docker.io/"))
							}
						}
					}
					want, err := images.Kubeadm("", tc.version)
					if err != nil {
						t.Errorf("kubeadm images: %v", tc.version)
					}
					sort.Strings(want)
					sort.Strings(gotImages)
					if diff := cmp.Diff(want, gotImages); diff != "" {
						t.Errorf("%s images mismatch (-want +got):\n%s", tc.version, diff)
					}
				}

				if strings.Contains(tc.name, "cni") {
					t.Logf("WARNING: cni mode requires additional setup before pods can schedule :(")
				} else if _, err := PodWait(ctx, t, profile, "default", "integration-test=busybox", 2*time.Minute); err != nil {
					t.Fatalf("wait: %v", err)
				}

				got = Status(ctx, t, Target(), profile)
				if got != state.Running.String() {
					t.Errorf("status = %q; want = %q", got, state.Running)
				}

				if *cleanup {
					// Normally handled by cleanuprofile, but not fatal there
					rr, err = Run(t, exec.CommandContext(ctx, Target(), "delete", "-p", profile))
					if err != nil {
						t.Errorf("%s failed: %v", rr.Args, err)
					}
				}
			})
		}
	})
}
