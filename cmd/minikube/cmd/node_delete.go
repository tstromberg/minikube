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

package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/mustload"
	"k8s.io/minikube/pkg/minikube/node"
	"k8s.io/minikube/pkg/minikube/out"
)

var nodeDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes a node from a cluster.",
	Long:  "Deletes a node from a cluster.",
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) == 0 {
			exit.UsageT("Usage: minikube node delete [name]")
		}
		name := args[0]

		co := mustload.Healthy(ClusterFlagValue())
		out.T(out.DeletingHost, "Deleting node {{.name}} from cluster {{.cluster}}", out.V{"name": name, "cluster": co.Config.Name})

		n, err := node.Delete(*co.Config, name)
		if err != nil {
			exit.WithError("GUEST_NODE_DELETE", "deleting node", err)
		}

		if driver.IsKIC(co.Config.Driver) {
			machineName := driver.MachineName(*co.Config, *n)
			deletePossibleKicLeftOver(machineName, co.Config.Driver)
		}

		out.T(out.Deleted, "Node {{.name}} was successfully deleted.", out.V{"name": name})
	},
}

func init() {
	nodeCmd.AddCommand(nodeDeleteCmd)
}
