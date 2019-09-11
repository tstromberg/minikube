/*
Copyright 2019 The Kubernetes Authors All rights reserved.

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

package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/minikube/pkg/minikube/constants"
)

func Test_extractVMDriverVersion(t *testing.T) {
	v := extractVMDriverVersion("")
	if len(v) != 0 {
		t.Error("Expected empty string")
	}

	v = extractVMDriverVersion("random text")
	if len(v) != 0 {
		t.Error("Expected empty string")
	}

	expectedVersion := "1.2.3"

	v = extractVMDriverVersion("version: v1.2.3")
	if expectedVersion != v {
		t.Errorf("Expected version: %s, got: %s", expectedVersion, v)
	}

	v = extractVMDriverVersion("version: 1.2.3")
	if expectedVersion != v {
		t.Errorf("Expected version: %s, got: %s", expectedVersion, v)
	}
}

func TestGenerateCfgFromFlagsHTTPProxyHandling(t *testing.T) {
	viper.SetDefault(memory, constants.DefaultMemorySize)
	viper.SetDefault(humanReadableDiskSize, constants.DefaultDiskSize)
	originalEnv := os.Getenv("HTTP_PROXY")
	defer func() {
		err := os.Setenv("HTTP_PROXY", originalEnv)
		if err != nil {
			t.Fatalf("Error reverting env HTTP_PROXY to it's original value. Got err: %s", err)
		}
	}()
	k8sVersion := constants.NewestKubernetesVersion
	var tests = []struct {
		description  string
		proxy        string
		proxyIgnored bool
	}{

		{
			description:  "http_proxy=127.0.0.1:3128",
			proxy:        "127.0.0.1:3128",
			proxyIgnored: true,
		},
		{
			description:  "http_proxy=localhost:3128",
			proxy:        "localhost:3128",
			proxyIgnored: true,
		},
		{
			description: "http_proxy=1.2.3.4:3128",
			proxy:       "1.2.3.4:3128",
		},
		{
			description: "no http_proxy",
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			cmd := &cobra.Command{}
			if err := os.Setenv("HTTP_PROXY", test.proxy); err != nil {
				t.Fatalf("Unexpected error setting HTTP_PROXY: %v", err)
			}
			config, err := generateCfgFromFlags(cmd, k8sVersion)
			if err != nil {
				t.Fatalf("Got unexpected error %v during config generation", err)
			}
			// ignored proxy should not be in config
			for _, v := range config.MachineConfig.DockerEnv {
				if v == test.proxy && test.proxyIgnored {
					t.Fatalf("Value %v not expected in dockerEnv but occurred", v)
				}
			}
		})
	}
}
