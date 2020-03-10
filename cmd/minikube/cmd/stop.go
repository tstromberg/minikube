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

package cmd

import (
	"time"

	"github.com/docker/machine/libmachine"
	"github.com/docker/machine/libmachine/mcnerror"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/minikube/pkg/minikube/config"
	pkg_config "k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/kubeconfig"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/out"
	"k8s.io/minikube/pkg/util/retry"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops a running local kubernetes cluster",
	Long: `Stops a local kubernetes cluster running in Virtualbox. This command stops the VM
itself, leaving all files intact. The cluster can be started again with the "start" command.`,
	Run: runStop,
}

// runStop handles the executes the flow of "minikube stop"
func runStop(cmd *cobra.Command, args []string) {
	profile := viper.GetString(pkg_config.ProfileName)
	api, err := machine.NewAPIClient()
	if err != nil {
		exit.WithError("Error getting client", err)
	}
	defer api.Close()

	cc, err := config.Load(profile)
	if err != nil {
		exit.WithError("Error getting cluster config", err)
	}

	for _, n := range cc.Nodes {
		nonexistent := stop(api, *cc, n)

		if !nonexistent {
			out.T(out.Stopped, `Node "{{.node_name}}" stopped.`, out.V{"node_name": n.Name})
		}
	}

	if err := killMountProcess(); err != nil {
		out.T(out.Warning, "Unable to kill mount process: {{.error}}", out.V{"error": err})
	}

	err = kubeconfig.UnsetCurrentContext(profile, kubeconfig.PathFromEnv())
	if err != nil {
		exit.WithError("update config", err)
	}
}

func stop(api libmachine.API, cluster config.ClusterConfig, n config.Node) bool {
	nonexistent := false
	stop := func() (err error) {
		machineName := driver.MachineName(cluster, n)
		err = machine.StopHost(api, machineName)
		if err == nil {
			return nil
		}
		glog.Warningf("stop host returned error: %v", err)

		switch err := errors.Cause(err).(type) {
		case mcnerror.ErrHostDoesNotExist:
			out.T(out.Meh, `"{{.profile_name}}" does not exist, nothing to stop`, out.V{"profile_name": cluster})
			nonexistent = true
			return nil
		default:
			return err
		}
	}

	if err := retry.Expo(stop, 5*time.Second, 3*time.Minute, 5); err != nil {
		exit.WithError("Unable to stop VM", err)
	}

	return nonexistent
}
