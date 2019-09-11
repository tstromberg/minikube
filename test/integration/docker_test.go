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
	"strings"
	"testing"
	"time"
)

func TestDockerFlags(t *testing.T) {
	if NoneDriver() {
		t.Skip("skipping: none driver does not support ssh or bundle docker")
	}
	MaybeParallel(t)

	profile := UniqueProfileName("docker-flags")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer Cleanup(t, profile, cancel)

	args := append([]string{"start", "-p", profile, "--wait=false", "--docker-env=FOO=BAR", "--docker-env=BAZ=BAT", "--docker-opt=debug", "--docker-opt=icc=true"}, StartArgs()...)
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Errorf("%s failed: %v", rr.Args, err)
	}

	rr, err = Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "ssh", "systemctl show docker --property=Environment --no-pager"))
	if err != nil {
		t.Errorf("%s failed: %v", rr.Args, err)
	}

	for _, envVar := range []string{"FOO=BAR", "BAZ=BAT"} {
		if !strings.Contains(rr.Stdout.String(), envVar) {
			t.Errorf("env var %s missing: %s.", envVar, rr.Stdout)
		}
	}

	rr, err = Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "ssh", "systemctl show docker --property=ExecStart --no-pager"))
	if err != nil {
		t.Errorf("%s failed: %v", rr.Args, err)
	}
	for _, opt := range []string{"--debug", "--icc=true"} {
		if !strings.Contains(rr.Stdout.String(), opt) {
			t.Fatalf("%s = %q, want *%s*", rr.Command(), rr.Stdout, opt)
		}
	}
}
