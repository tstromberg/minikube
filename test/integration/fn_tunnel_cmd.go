/*
Copyright 2018 The Kubernetes Authors All rights reserved.

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
	"io/ioutil"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/pkg/errors"

	"k8s.io/minikube/pkg/kapi"
	"k8s.io/minikube/pkg/minikube/tunnel"
	"k8s.io/minikube/pkg/util/retry"
)

func validateTunnelCmd(ctx context.Context, t *testing.T, profile string) {
	ctx, cancel := context.WithTimeout(ctx, Minutes(20))
	defer cancel()

	if !KicDriver() && runtime.GOOS != "windows" {
		if err := exec.Command("sudo", "-n", "ifconfig").Run(); err != nil {
			t.Skipf("password required to execute 'route', skipping testTunnel: %v", err)
		}
	}

	client, err := kapi.Client(profile)
	if err != nil {
		t.Fatalf("failed to get kubernetes client for %q: %v", profile, err)
	}

	// Pre-Cleanup
	if err := tunnel.NewManager().CleanupNotRunningTunnels(); err != nil {
		t.Errorf("CleanupNotRunningTunnels: %v", err)
	}

	// Start the tunnel
	args := []string{"-p", profile, "tunnel", "--alsologtostderr"}
	ss, err := Start(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Errorf("failed to start a tunnel: args %q: %v", args, err)
	}
	defer ss.Stop(t)

	// Start the "nginx" pod.
	rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "apply", "-f", filepath.Join(*testdataDir, "testsvc.yaml")))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Command(), err)
	}
	if _, err := PodWait(ctx, t, profile, "default", "run=nginx-svc", Minutes(4)); err != nil {
		t.Fatalf("wait: %v", err)
	}

	if err := kapi.WaitForService(client, "default", "nginx-svc", true, 1*time.Second, Minutes(2)); err != nil {
		t.Fatal(errors.Wrap(err, "Error waiting for nginx service to be up"))
	}

	// Wait until the nginx-svc has a loadbalancer ingress IP
	hostname := ""
	err = wait.PollImmediate(5*time.Second, Minutes(3), func() (bool, error) {
		rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "get", "svc", "nginx-svc", "-o", "jsonpath={.status.loadBalancer.ingress[0].ip}"))
		if err != nil {
			return false, err
		}
		if len(rr.Stdout.String()) > 0 {
			hostname = rr.Stdout.String()
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Errorf("nginx-svc svc.status.loadBalancer.ingress never got an IP")

		rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "get", "svc", "nginx-svc"))
		if err != nil {
			t.Errorf("%s failed: %v", rr.Command(), err)
		}
		t.Logf("failed to kubectl get svc nginx-svc:\n%s", rr.Stdout)
	}

	got := []byte{}
	url := fmt.Sprintf("http://%s", hostname)

	fetch := func() error {
		h := &http.Client{Timeout: time.Second * 10}
		resp, err := h.Get(url)
		if err != nil {
			return &retry.RetriableError{Err: err}
		}
		if resp.Body == nil {
			return &retry.RetriableError{Err: fmt.Errorf("no body")}
		}
		defer resp.Body.Close()
		got, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return &retry.RetriableError{Err: err}
		}
		return nil
	}
	if err = retry.Expo(fetch, 3*time.Second, Minutes(2), 13); err != nil {
		t.Errorf("failed to hit nginx at %q: %v", url, err)
	}

	want := "Welcome to nginx!"
	if strings.Contains(string(got), want) {
		t.Logf("tunnel at %s is working!", url)
	} else {
		t.Errorf("expected body to contain %q, but got *%q*", want, got)
	}

	// Not all platforms support DNS forwarding
	if runtime.GOOS != "darwin" {
		return
	}

	// use FQDN to avoid extra DNS query lookup
	url = "http://nginx-svc.default.svc.cluster.local."
	if err = retry.Expo(fetch, 3*time.Second, Seconds(30), 10); err != nil {
		t.Errorf("failed to hit nginx with DNS forwarded %q: %v", url, err)
		// debug more information for: https://github.com/kubernetes/minikube/issues/7809
		clusterLogs(t, profile)
	}

	want = "Welcome to nginx!"
	if strings.Contains(string(got), want) {
		t.Logf("tunnel at %s is working!", url)
	} else {
		t.Errorf("expected body to contain %q, but got *%q*", want, got)
	}

}
