// +build integration

/*
Copyright 2020 The Kubernetes Authors All rights reserved.

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
	"os/exec"
	"strings"
	"testing"
)

func TestPause(t *testing.T) {
	MaybeParallel(t)

	type validateFunc func(context.Context, *testing.T, string)
	profile := UniqueProfileName("pause")
	ctx, cancel := context.WithTimeout(context.Background(), Minutes(30))
	defer CleanupWithLogs(t, profile, cancel)

	// Serial tests
	t.Run("serial", func(t *testing.T) {
		tests := []struct {
			name      string
			validator validateFunc
		}{
			{"Start", validateFreshStart},
			{"SecondStartNoReset", validateStartNoReset},
			{"Pause", validatePause},
			{"Unpause", validateUnpause},
			{"PauseAgain", validatePause},
			{"DeletePaused", validateDelete},
			{"VerifyDeletedResources", validateVerifyDeleted},
		}
		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				tc.validator(ctx, t, profile)
			})
		}
	})
}

func validateFreshStart(ctx context.Context, t *testing.T, profile string) {
	args := append([]string{"start", "-p", profile, "--memory=1800", "--install-addons=false", "--wait=all"}, StartArgs()...)
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Fatalf("failed to start minikube with args: %q : %v", rr.Command(), err)
	}
}

// validateStartNoReset validates that starting a running cluster won't invoke a reset
func validateStartNoReset(ctx context.Context, t *testing.T, profile string) {
	args := []string{"start", "-p", profile, "--alsologtostderr", "-v=5"}
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Fatalf("failed to second start a running minikube with args: %q : %v", rr.Command(), err)
	}
	if !NoneDriver() {
		softLog := "The running cluster does not need a reset"
		if !strings.Contains(rr.Output(), softLog) {
			t.Errorf("expected the second start log outputs to include %q but got: %s", softLog, rr.Output())
		}
	}

}

func validatePause(ctx context.Context, t *testing.T, profile string) {
	args := []string{"pause", "-p", profile, "--alsologtostderr", "-v=5"}
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Errorf("failed to pause minikube with args: %q : %v", rr.Command(), err)
	}
}

func validateUnpause(ctx context.Context, t *testing.T, profile string) {
	args := []string{"unpause", "-p", profile, "--alsologtostderr", "-v=5"}
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Errorf("failed to unpause minikube with args: %q : %v", rr.Command(), err)
	}
}

func validateDelete(ctx context.Context, t *testing.T, profile string) {
	args := []string{"delete", "-p", profile, "--alsologtostderr", "-v=5"}
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Errorf("failed to delete minikube with args: %q : %v", rr.Command(), err)
	}
}

// make sure no left over left after deleting a profile such as containers or volumes
func validateVerifyDeleted(ctx context.Context, t *testing.T, profile string) {
	rr, err := Run(t, exec.CommandContext(ctx, Target(), "profile", "list", "--output", "json"))
	if err != nil {
		t.Errorf("failed to list profiles with json format after it was deleted. args %q: %v", rr.Command(), err)
	}

	var jsonObject map[string][]map[string]interface{}
	if err := json.Unmarshal(rr.Stdout.Bytes(), &jsonObject); err != nil {
		t.Errorf("failed to decode json from profile list: args %q: %v", rr.Command(), err)
	}
	validProfiles := jsonObject["valid"]
	profileExists := false
	for _, profileObject := range validProfiles {
		if profileObject["Name"] == profile {
			profileExists = true
			break
		}
	}
	if profileExists {
		t.Errorf("expected the deleted profile %q not to show up in profile list but it does! output: %s . args: %q", profile, rr.Stdout.String(), rr.Command())
	}

	if KicDriver() {
		bin := "docker"
		if PodmanDriver() {
			bin = "podman"
		}
		rr, err := Run(t, exec.CommandContext(ctx, bin, "ps", "-a"))
		if err == nil && strings.Contains(rr.Output(), profile) {
			t.Errorf("expected container %q not to exist in output of %s but it does output: %s.", profile, rr.Command(), rr.Output())
		}

		rr, err = Run(t, exec.CommandContext(ctx, bin, "volume", "inspect", profile))
		if err == nil {
			t.Errorf("expected to see error and volume %q to not exist after deletion but got no error and this output: %s", rr.Command(), rr.Output())
		}

	}

}
