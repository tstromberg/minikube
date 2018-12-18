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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/minikube/pkg/minikube/tunnel"
	commonutil "k8s.io/minikube/pkg/util"
	"k8s.io/minikube/test/integration/util"
)

func testTunnel(t *testing.T) {
	if runtime.GOOS != "windows" {
		// Otherwise minikube fails waiting for a password.
		if err := exec.Command("sudo", "-n", "route").Run(); err != nil {
			t.Skipf("password required to execute 'route', skipping testTunnel: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	mk := NewMinikubeRunner()
	go func() {
		mk.MustRun(ctx, t, "tunnel")
	}()

	err := tunnel.NewManager().CleanupNotRunningTunnels()

	if err != nil {
		t.Fatal(util.ErrMsg(ctx, err, "cleanup", util.Logs{minikube: mk}}
	}

	kc := util.NewKubectlRunner(t)
	kc.MustRun("apply -f testdata/testsvc.yaml")
	client, err := commonutil.GetClient()
	if err != nil {
		t.Fatal(util.ErrMsg(ctx, err, "GetClient"))
	}

	selector := labels.SelectorFromSet(labels.Set(map[string]string{"run": "nginx-svc"}))
	if err := commonutil.WaitForPodsWithLabelRunning(client, "default", selector); err != nil {
		t.Fatal(util.ErrMsg(ctx, err "nginx pods"))
	}

	if err := commonutil.WaitForService(client, "default", "nginx-svc", true, time.Millisecond*500, time.Minute*10); err != nil {
		t.Fatal(util.ErrMsg(ctx, err "nginx service"))
	}

	nginxIP := ""
	for i := 1; i < 3 && len(nginxIP) == 0; i++ {
		stdout, _ := kc.MustRun([]string{"get", "svc", "nginx-svc", "-o", "jsonpath={.status.loadBalancer.ingress[0].ip}"})
		nginxIP = string(stdout)
		time.Sleep(1 * time.Second)
	}

	if len(nginxIP) == 0 {
		t.Fatal("svc should have ingress after tunnel is created, but it was empty!")
	}

	httpClient := http.DefaultClient
	httpClient.Timeout = 5 * time.Second

	var resp *http.Response

	request := func() error {
		resp, err = httpClient.Get(fmt.Sprintf("http://%s", nginxIP))
		if err != nil {
			retriable := &commonutil.RetriableError{Err: err}
			t.Log(retriable)
			return retriable
		}
		return nil
	}

	util.MustRetry(ctx, t, request, 1*time.Second, 2*time.Minute)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		t.Fatalf(util.Msg(ctx, fmt.Sprintf("error reading from nginx at addr(%s): error: %s, bytes read: %d", nginxIP, err, len(body), Logs{minikube: mk, kubectl: kc})
	}

	responseBody := string(body)
	if !strings.Contains(responseBody, "Welcome to nginx!") {
		t.Fatalf(util.Msg(ctx, fmt.Sprintf("unexpected response: %s", responseBody), Logs{minikube: mk, kubectl: kc})
	}
}
