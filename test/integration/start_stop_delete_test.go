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
	"net"
	"testing"
	"time"

	"github.com/docker/machine/libmachine/state"
	"k8s.io/minikube/pkg/minikube/constants"
)

func TestStartStop(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
	}{
		{
			name:    "default",
			runtime: "",
		},
		{
			name:    "start stop with containerd runtime",
			runtime: constants.ContainerdRuntime,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			mk := NewMinikubeRunner()
			if test.runtime != "" && mk.VMDriver == "none" {
				t.Skipf("skipping, can't use %s with none driver", test.runtime)
			}

			mk.MustRun(ctx, "config set WantReportErrorPrompt false")
			mk.Run(ctx, "delete")
			mk.MustBeInState(state.None)

			runner.SetRuntime(test.runtime)
			mk.Run(ctx, mk.StartArgs())
			mk.MustBeInState(state.Running)

			ip := mk.MustRun(ctx, "ip").TrimRight("\n")
			if net.ParseIP(ip) == nil {
				t.Fatalf(util.Msg(ctx, fmt.Sprintf("invalid IP address: %s", ip), Logs{minikube: mk})
			}
			mk.MustRun(ctx, "stop")
			mk.MustBeInState(ctx, state.Stopped)
			mk.MustRun(mk.StartArgs())
			mk.MustBeInState(ctx, state.Running)
			mk.MustRun(ctx, "delete")
			mk.MustBeInState(ctx, state.None)
		})
	}
}
