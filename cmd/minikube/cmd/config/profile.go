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

package config

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	pkgConfig "k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/console"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/exit"
	pkgutil "k8s.io/minikube/pkg/util"
)

// ProfileCmd represents the profile command
var ProfileCmd = &cobra.Command{
	Use:   "profile [MINIKUBE_PROFILE_NAME].  You can return to the default minikube profile by running `minikube profile default`",
	Short: "Profile gets or sets the current minikube profile",
	Long:  "profile sets the current minikube profile, or gets the current profile if no arguments are provided.  This is used to run and manage multiple minikube instance.  You can return to the default minikube profile by running `minikube profile default`",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			profile := viper.GetString(pkgConfig.MachineProfile)
			console.OutT(console.Empty, profile)
			os.Exit(0)
		}

		if len(args) > 1 {
			exit.UsageT("usage: minikube profile [MINIKUBE_PROFILE_NAME]")
		}

		profile := args[0]
		if profile == "default" {
			profile = "minikube"
		}
		err := Set(pkgConfig.MachineProfile, profile)
		if err != nil {
			exit.WithError("Setting profile failed", err)
		}
		cc, err := pkgConfig.Load()
		// might err when loading older version of cfg file that doesn't have KeepContext field
		if err != nil && !os.IsNotExist(err) {
			console.ErrT(console.Sad, `Error loading profile config: {{.error}}`, console.Arg{"error": err})
		}
		if err == nil {
			if cc.MachineConfig.KeepContext {
				console.SuccessT("Skipped switching kubectl context for {{.profile_name}} , because --keep-context", console.Arg{"profile_name": profile})
				console.SuccessT("To connect to this cluster, use: kubectl --context={{.profile_name}}", console.Arg{"profile_name": profile})
			} else {
				err := pkgutil.SetCurrentContext(constants.KubeconfigPath, profile)
				if err != nil {
					console.ErrT(console.Sad, `Error while setting kubectl current context :  {{.error}}`, console.Arg{"error": err})
				}
			}
		}
		console.SuccessT("minikube profile was successfully set to {{.profile_name}}", console.Arg{"profile_name": profile})
	},
}
