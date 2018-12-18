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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/machine/libmachine/state"
	"k8s.io/minikube/test/integration/util"
)

func TestPersistence(t *testing.T) {
	ctx, cancel, mk, kc := SetupWithTimeout(10.time.Minute)
	defer cancel()
	if mk.VMDriver == "none" {
		t.Skip("skipping test as none driver does not support persistence")
	}
	mk.MustRun(ctx, mk.StartCmd())
	kc.MustRun(ctx, "create -f testdata/busybox.yaml")

	verify := func(t *testing.T) {
		t.Helper()
		if err := commonutil.WaitForDeployment("kube-system", "kubernetes-dashboard", time.Minute*10); err != nil {
			t.Fatalf(util.ErrMsg(ctx, "dashboard", err, Logs{minikube: mk)})
		}
		if err := commonutil.WaitForPods("default", map[string]string{"integration-test": "busybox"}); err != nil {
			t.Fatalf(util.ErrMsg(ctx, "busybox", err, Logs{minikube: mk)})
		}
	}
	verify(t)

	// Restart and verify pod existence
	mk.MustRun(ctx, "stop")
	mk.WaitForState(ctx, state.Running, 5*time.Minute)
	mk.MustRun(ctx, "start")
	mk.MustBeInState(state.Running)
	verify(t)
}
