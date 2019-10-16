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

package cluster

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/host"
	"github.com/docker/machine/libmachine/provision"
	"github.com/docker/machine/libmachine/state"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/registry"
	"k8s.io/minikube/pkg/minikube/tests"
)

type MockDownloader struct{}

func (d MockDownloader) GetISOFileURI(isoURL string) string          { return "" }
func (d MockDownloader) CacheMinikubeISOFromURL(isoURL string) error { return nil }

func createMockDriverHost(c config.MachineConfig) interface{} {
	return nil
}

func RegisterMockDriver(t *testing.T) {
	t.Helper()
	_, err := registry.Driver(constants.DriverMock)
	// Already registered
	if err == nil {
		return
	}
	err = registry.Register(registry.DriverDef{
		Name:          constants.DriverMock,
		Builtin:       true,
		ConfigCreator: createMockDriverHost,
		DriverCreator: func() drivers.Driver {
			return &tests.MockDriver{T: t}
		},
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
}

var defaultMachineConfig = config.MachineConfig{
	VMDriver:    constants.DriverMock,
	MinikubeISO: "",
	Downloader:  MockDownloader{},
	DockerEnv:   []string{"MOCK_MAKE_IT_PROVISION=true"},
}

func TestCreateHost(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)

	exists, _ := api.Exists(config.GetMachineName())
	if exists {
		t.Fatal("Machine already exists.")
	}

	_, err := createHost(api, defaultMachineConfig)
	if err != nil {
		t.Fatalf("Error creating host: %v", err)
	}
	exists, err = api.Exists(config.GetMachineName())
	if err != nil {
		t.Fatalf("exists failed for %q: %v", config.GetMachineName(), err)
	}
	if !exists {
		t.Fatalf("%q does not exist, but should.", config.GetMachineName())
	}

	h, err := api.Load(config.GetMachineName())
	if err != nil {
		t.Fatalf("Error loading machine: %v", err)
	}

	if s, _ := h.Driver.GetState(); s != state.Running {
		t.Fatalf("Machine is not running. State is: %s", s)
	}

	found := false
	for _, def := range registry.ListDrivers() {
		if h.DriverName == def.Name {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Wrong driver name: %v. It should be among drivers %v", h.DriverName, registry.ListDrivers())
	}
}

func TestStartHostExists(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)
	// Create an initial host.
	_, err := createHost(api, defaultMachineConfig)
	if err != nil {
		t.Fatalf("Error creating host: %v", err)
	}

	// Make sure the next call to Create will fail, to assert it doesn't get called again.
	api.CreateError = true
	if err := api.Create(&host.Host{}); err == nil {
		t.Fatal("api.Create did not fail, but should have.")
	}

	md := &tests.MockDetector{Provisioner: &tests.MockProvisioner{}}
	provision.SetDetector(md)

	// This should pass without calling Create because the host exists already.
	h, err := StartHost(api, defaultMachineConfig)
	if err != nil {
		t.Fatalf("Error starting host: %v", err)
	}
	if h.Name != config.GetMachineName() {
		t.Fatalf("GetMachineName()=%q, want %q", config.GetMachineName(), h.Name)
	}
	if s, _ := h.Driver.GetState(); s != state.Running {
		t.Fatalf("Machine not started.")
	}
	if !md.Provisioner.Provisioned {
		t.Fatalf("Expected provision to be called")
	}
}

func TestStartStoppedHost(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)
	// Create an initial host.
	h, err := createHost(api, defaultMachineConfig)
	if err != nil {
		t.Fatalf("Error creating host: %v", err)
	}
	d := tests.MockDriver{T: t}
	h.Driver = &d
	d.CurrentState = state.Stopped

	md := &tests.MockDetector{Provisioner: &tests.MockProvisioner{}}
	provision.SetDetector(md)
	h, err = StartHost(api, defaultMachineConfig)
	if err != nil {
		t.Fatal("Error starting host.")
	}
	if h.Name != config.GetMachineName() {
		t.Fatalf("Machine created with incorrect name: %s", h.Name)
	}

	if s, _ := h.Driver.GetState(); s != state.Running {
		t.Fatalf("Machine not started.")
	}

	if !api.SaveCalled {
		t.Fatalf("Machine must be saved after starting.")
	}

	if !md.Provisioner.Provisioned {
		t.Fatalf("Expected provision to be called")
	}
}

func TestStartHost(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)

	md := &tests.MockDetector{Provisioner: &tests.MockProvisioner{}}
	provision.SetDetector(md)

	h, err := StartHost(api, defaultMachineConfig)
	if err != nil {
		t.Fatal("Error starting host.")
	}
	if h.Name != config.GetMachineName() {
		t.Fatalf("GetMachineName()=%q, want %q", config.GetMachineName(), h.Name)
	}
	if exists, _ := api.Exists(h.Name); !exists {
		t.Fatal("Machine not saved.")
	}
	if s, _ := h.Driver.GetState(); s != state.Running {
		t.Fatalf("Machine not started.")
	}

	// Provision regenerates Docker certs. This happens automatically during create,
	// so we should only call it again if the host already exists.
	if md.Provisioner.Provisioned {
		t.Fatalf("Did not expect Provision to be called")
	}
}

func TestStartHostConfig(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)

	md := &tests.MockDetector{Provisioner: &tests.MockProvisioner{}}
	provision.SetDetector(md)

	config := config.MachineConfig{
		VMDriver:   constants.DriverMock,
		DockerEnv:  []string{"FOO=BAR"},
		DockerOpt:  []string{"param=value"},
		Downloader: MockDownloader{},
	}

	h, err := StartHost(api, config)
	if err != nil {
		t.Fatal("Error starting host.")
	}

	for i := range h.HostOptions.EngineOptions.Env {
		if h.HostOptions.EngineOptions.Env[i] != config.DockerEnv[i] {
			t.Fatal("Docker env variables were not set!")
		}
	}

	for i := range h.HostOptions.EngineOptions.ArbitraryFlags {
		if h.HostOptions.EngineOptions.ArbitraryFlags[i] != config.DockerOpt[i] {
			t.Fatal("Docker flags were not set!")
		}
	}

}

func TestStopHostError(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)
	if err := StopHost(api); err == nil {
		t.Fatal("An error should be thrown when stopping non-existing machine.")
	}
}

func TestStopHost(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)
	h, err := createHost(api, defaultMachineConfig)
	if err != nil {
		t.Errorf("createHost failed: %v", err)
	}

	if err := StopHost(api); err != nil {
		t.Fatal("An error should be thrown when stopping non-existing machine.")
	}
	if s, _ := h.Driver.GetState(); s != state.Stopped {
		t.Fatalf("Machine not stopped. Currently in state: %s", s)
	}
}

func TestDeleteHost(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)
	if _, err := createHost(api, defaultMachineConfig); err != nil {
		t.Errorf("createHost failed: %v", err)
	}

	if err := DeleteHost(api); err != nil {
		t.Fatalf("Unexpected error deleting host: %v", err)
	}
}

func TestDeleteHostErrorDeletingVM(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)
	h, err := createHost(api, defaultMachineConfig)
	if err != nil {
		t.Errorf("createHost failed: %v", err)
	}

	d := &tests.MockDriver{RemoveError: true, T: t}
	h.Driver = d

	if err := DeleteHost(api); err == nil {
		t.Fatal("Expected error deleting host.")
	}
}

func TestDeleteHostErrorDeletingFiles(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)
	api.RemoveError = true
	if _, err := createHost(api, defaultMachineConfig); err != nil {
		t.Errorf("createHost failed: %v", err)
	}

	if err := DeleteHost(api); err == nil {
		t.Fatal("Expected error deleting host.")
	}
}

func TestGetHostStatus(t *testing.T) {
	RegisterMockDriver(t)
	api := tests.NewMockAPI(t)

	checkState := func(expected string) {
		s, err := GetHostStatus(api)
		if err != nil {
			t.Fatalf("Unexpected error getting status: %v", err)
		}
		if s != expected {
			t.Fatalf("Expected status: %s, got %s", s, expected)
		}
	}

	checkState(state.None.String())

	if _, err := createHost(api, defaultMachineConfig); err != nil {
		t.Errorf("createHost failed: %v", err)
	}

	checkState(state.Running.String())

	if err := StopHost(api); err != nil {
		t.Errorf("StopHost failed: %v", err)
	}
	checkState(state.Stopped.String())
}

func TestGetHostDockerEnv(t *testing.T) {
	RegisterMockDriver(t)
	tempDir := tests.MakeTempDir()
	defer os.RemoveAll(tempDir)

	api := tests.NewMockAPI(t)
	h, err := createHost(api, defaultMachineConfig)
	if err != nil {
		t.Fatalf("Error creating host: %v", err)
	}
	d := &tests.MockDriver{
		BaseDriver: drivers.BaseDriver{
			IPAddress: "127.0.0.1",
		},
		T: t,
	}
	h.Driver = d

	envMap, err := GetHostDockerEnv(api)
	if err != nil {
		t.Fatalf("Unexpected error getting env: %v", err)
	}

	dockerEnvKeys := [...]string{
		"DOCKER_TLS_VERIFY",
		"DOCKER_HOST",
		"DOCKER_CERT_PATH",
	}
	for _, dockerEnvKey := range dockerEnvKeys {
		if _, hasKey := envMap[dockerEnvKey]; !hasKey {
			t.Fatalf("Expected envMap[\"%s\"] key to be defined", dockerEnvKey)
		}
	}
}

func TestGetHostDockerEnvIPv6(t *testing.T) {
	tempDir := tests.MakeTempDir()
	defer os.RemoveAll(tempDir)

	api := tests.NewMockAPI(t)
	h, err := createHost(api, defaultMachineConfig)
	if err != nil {
		t.Fatalf("Error creating host: %v", err)
	}
	d := &tests.MockDriver{
		BaseDriver: drivers.BaseDriver{
			IPAddress: "fe80::215:5dff:fe00:a903",
		},
		T: t,
	}
	h.Driver = d

	envMap, err := GetHostDockerEnv(api)
	if err != nil {
		t.Fatalf("Unexpected error getting env: %v", err)
	}

	expected := "tcp://[fe80::215:5dff:fe00:a903]:2376"
	v := envMap["DOCKER_HOST"]
	if v != expected {
		t.Fatalf("Expected DOCKER_HOST to be defined as %s but was %s", expected, v)
	}
}

func TestCreateSSHShell(t *testing.T) {
	api := tests.NewMockAPI(t)

	s, _ := tests.NewSSHServer(t)
	port, err := s.Start()
	if err != nil {
		t.Fatalf("Error starting ssh server: %v", err)
	}

	d := &tests.MockDriver{
		Port:         port,
		CurrentState: state.Running,
		BaseDriver: drivers.BaseDriver{
			IPAddress:  "127.0.0.1",
			SSHKeyPath: "",
		},
		T: t,
	}
	api.Hosts[config.GetMachineName()] = &host.Host{Driver: d}

	cliArgs := []string{"exit"}
	if err := CreateSSHShell(api, cliArgs); err != nil {
		t.Fatalf("Error running ssh command: %v", err)
	}

	if !s.IsSessionRequested() {
		t.Fatalf("Expected ssh session to be run")
	}
}

func TestGuestClockDelta(t *testing.T) {
	local := time.Now()
	h := tests.NewMockHost()
	// Truncate remote clock so that it is between 0 and 1 second behind
	h.CommandOutput["date +%s.%N"] = fmt.Sprintf("%d.0000", local.Unix())
	got, err := guestClockDelta(h, local)
	if err != nil {
		t.Fatalf("guestClock: %v", err)
	}
	if got > (0 * time.Second) {
		t.Errorf("unexpected positive delta (remote should be behind): %s", got)
	}
	if got < (-1 * time.Second) {
		t.Errorf("unexpectedly negative delta (remote too far behind): %s", got)
	}
}
