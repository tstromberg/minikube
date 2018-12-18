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
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	pkgutil "k8s.io/minikube/pkg/util"
	"k8s.io/minikube/test/integration/util"
)

func waitForNginx(t *testing.T) error {
	client, err := commonutil.GetClient()

	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}

	selector := labels.SelectorFromSet(labels.Set(map[string]string{"run": "nginx"}))
	if err := commonutil.WaitForPodsWithLabel(client, "default", selector); err != nil {
		return errors.Wrap(err, "waiting for nginx pods")
	}

	if err := commonutil.WaitForService(client, "default", "nginx", true, time.Millisecond*500, time.Minute*10); err != nil {
		t.Errorf("Error waiting for nginx service to be up")
	}
	return nil
}

func waitForIngressController(t *testing.T) error {
	client, err := commonutil.GetClient()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}

	if err := commonutil.WaitForDeploymentToStabilize(client, "kube-system", "nginx-ingress-controller", time.Minute*10); err != nil {
		return errors.Wrap(err, "waiting for ingress-controller deployment to stabilize")
	}

	selector := labels.SelectorFromSet(labels.Set(map[string]string{"app.kubernetes.io/name": "nginx-ingress-controller"}))
	if err := commonutil.WaitForPodsWithLabel(client, "kube-system", selector); err != nil {
		return errors.Wrap(err, "waiting for ingress-controller pods")
	}

	return nil
}

func waitForIngressDefaultBackend(t *testing.T) error {
	client, err := commonutil.GetClient()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes client")
	}

	if err := commonutil.WaitForDeploymentToStabilize(client, "kube-system", "default-http-backend", time.Minute*10); err != nil {
		return errors.Wrap(err, "waiting for default-http-backend deployment to stabilize")
	}

	if err := commonutil.WaitForService(client, "kube-system", "default-http-backend", true, time.Millisecond*500, time.Minute*10); err != nil {
		return errors.Wrap(err, "waiting for default-http-backend service to be up")
	}

	if err := commonutil.WaitForServiceEndpointsNum(client, "kube-system", "default-http-backend", 1, time.Second*3, time.Minute*10); err != nil {
		return errors.Wrap(err, "waiting for one default-http-backend endpoint to be up")
	}

	return nil
}

func testAddons(t *testing.T) {
	t.Parallel()
	client, err := pkgutil.GetClient()
	if err != nil {
		t.Fatalf("Could not get kubernetes client: %v", err)
	}
	selector := labels.SelectorFromSet(labels.Set(map[string]string{"component": "kube-addon-manager"}))
	if err := pkgutil.WaitForPodsWithLabel(client, "kube-system", selector); err != nil {
		t.Errorf("Error waiting for addon manager to be up")
	}
}

func readLineWithTimeout(b *bufio.Reader, timeout time.Duration) (string, error) {
	s := make(chan string)
	e := make(chan error)
	go func() {
		read, err := b.ReadString('\n')
		if err != nil {
			e <- err
		} else {
			s <- read
		}
		close(s)
		close(e)
	}()

	select {
	case line := <-s:
		return line, nil
	case err := <-e:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout after %s", timeout)
	}
}

func testDashboard(t *testing.T) {
	t.Parallel()
	mk := NewMinikubeRunner()
	cmd, out := minikubeRunner.RunDaemon(ctx, "dashboard --url")
	defer func() {
		err := cmd.Process.Kill()
		if err != nil {
			t.Logf("Failed to kill dashboard command: %v", err)
		}
	}()

	s, err := readLineWithTimeout(out, 180*time.Second)
	if err != nil {
		t.Fatalf("failed to read url: %v", err)
	}

	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		t.Fatalf("failed to parse %q: %v", s, err)
	}

	if u.Scheme != "http" {
		t.Errorf("got Scheme %s, expected http", u.Scheme)
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("failed SplitHostPort: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("got host %s, expected 127.0.0.1", host)
	}

	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatalf("failed get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Unable to read http response body: %v", err)
		}
		t.Errorf("%s returned status code %d, expected %d.\nbody:\n%s", u, resp.StatusCode, http.StatusOK, body)
	}
}

func testIngressController(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	mk := NewMinikubeRunner()
	mk.MustRun(ctx, t, "addons enable ingress")
	if err := waitForIngressController(t); err != nil {
		t.Fatalf("waiting for ingress-controller to be up: %v", err)
	}

	if err := waitForIngressDefaultBackend(t); err != nil {
		t.Fatalf("waiting for default-http-backend to be up: %v", err)
	}

	kc := util.NewKubectlRunner()
	kc.MustRun(ctx, t, "create -f testdata/nginx-ing.yaml")
	kc.MustRun(ctx, t, "create -f testdata/nginx-pod-svc.yaml")
	if err := util.waitForNginx(t); err != nil {
		t.Fatalf("waiting for nginx to be up: %v", err)
	}

	checkIngress := func() error {
		want := "Welcome to nginx!"
		got, stderr, err := mk.Run("ssh -- curl http://127.0.0.1:80 -H 'Host: nginx.example.com'")
		if !strings.Contains(got, want) {
			return fmt.Errorf("got: %q, want: %q", got, want)
		}
		return nil
	}

	defer func() {
		for _, p := range []string{podPath, ingressPath} {
			if stdout, stderr, err := kc.Run(ctx, fmt.Sprintf("delete -f %s")); err != nil {
				t.Logf("delete -f %s failed: %v\noutput: %s\n", p, err, out)
			}
		}
	}()	
	util.MustRetry(ctx,t, checkIngress, 3*time.Second, 5)
	mk.MustRun("addons disable ingress")
}

func testServicesList(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	mk := NewMinikubeRunner(t)
	stdout := mk.MustRun(ctx, "service list")
	if !strings.Contains(stdout, "kubernetes") {
		t.Fatalf(util.Msg(fmt.Sprintf("kubernetes service missing from output %s", output)))
	}
}
