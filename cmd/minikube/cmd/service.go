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
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"k8s.io/minikube/pkg/drivers/kic/oci"
	"k8s.io/minikube/pkg/minikube/config"
	pkg_config "k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/out"
	"k8s.io/minikube/pkg/minikube/service"
	"k8s.io/minikube/pkg/minikube/tunnel/kic"
)

const defaultServiceFormatTemplate = "http://{{.IP}}:{{.Port}}"

var (
	namespace          string
	https              bool
	serviceURLMode     bool
	serviceURLFormat   string
	serviceURLTemplate *template.Template
	wait               int
	interval           int
)

// serviceCmd represents the service command
var serviceCmd = &cobra.Command{
	Use:   "service [flags] SERVICE",
	Short: "Gets the kubernetes URL(s) for the specified service in your local cluster",
	Long:  `Gets the kubernetes URL(s) for the specified service in your local cluster. In the case of multiple URLs they will be printed one at a time.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		t, err := template.New("serviceURL").Parse(serviceURLFormat)
		if err != nil {
			exit.WithError("The value passed to --format is invalid", err)
		}
		serviceURLTemplate = t

		RootCmd.PersistentPreRun(cmd, args)
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 || len(args) > 1 {
			exit.UsageT("You must specify a service name")
		}

		svc := args[0]
		api, err := machine.NewAPIClient()
		if err != nil {
			exit.WithError("Error getting client", err)
		}
		defer api.Close()

		profileName := viper.GetString(pkg_config.ProfileName)
		cfg, err := config.Load(profileName)
		if err != nil {
			exit.WithError("Error getting config", err)
		}
		cp, err := config.PrimaryControlPlane(cfg)
		if err != nil {
			exit.WithError("Error getting control plane", err)
		}
		machineName := driver.MachineName(*cfg, cp)
		if !machine.IsHostRunning(api, machineName) {
			os.Exit(1)
		}

		if runtime.GOOS == "darwin" && cfg.Driver == oci.Docker {
			startKicServiceTunnel(svc, cfg.Name)
			return
		}

		urls, err := service.WaitForService(api, namespace, svc, serviceURLTemplate, serviceURLMode, https, wait, interval)
		if err != nil {
			exit.WithError("Error opening service", err)
		}

		openURLs(svc, urls)
	},
}

func init() {
	serviceCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "The service namespace")
	serviceCmd.Flags().BoolVar(&serviceURLMode, "url", false, "Display the kubernetes service URL in the CLI instead of opening it in the default browser")
	serviceCmd.Flags().BoolVar(&https, "https", false, "Open the service URL with https instead of http")
	serviceCmd.Flags().IntVar(&wait, "wait", service.DefaultWait, "Amount of time to wait for a service in seconds")
	serviceCmd.Flags().IntVar(&interval, "interval", service.DefaultInterval, "The initial time interval for each check that wait performs in seconds")

	serviceCmd.PersistentFlags().StringVar(&serviceURLFormat, "format", defaultServiceFormatTemplate, "Format to output service URL in. This format will be applied to each url individually and they will be printed one at a time.")

}

func startKicServiceTunnel(svc, configName string) {
	ctrlC := make(chan os.Signal, 1)
	signal.Notify(ctrlC, os.Interrupt)

	clientset, err := service.K8s.GetClientset(1 * time.Second)
	if err != nil {
		exit.WithError("error creating clientset", err)
	}

	port, err := oci.HostPortBinding(oci.Docker, configName, 22)
	if err != nil {
		exit.WithError("error getting ssh port", err)
	}
	sshPort := strconv.Itoa(port)
	sshKey := filepath.Join(localpath.MiniPath(), "machines", configName, "id_rsa")

	serviceTunnel := kic.NewServiceTunnel(sshPort, sshKey, clientset.CoreV1())
	urls, err := serviceTunnel.Start(svc, namespace)
	if err != nil {
		exit.WithError("error starting tunnel", err)
	}

	// wait for tunnel to come up
	time.Sleep(1 * time.Second)

	data := [][]string{{namespace, svc, "", strings.Join(urls, "\n")}}
	service.PrintServiceList(os.Stdout, data)

	openURLs(svc, urls)
	out.T(out.Warning, "Because you are using docker driver on Mac, the terminal needs to be open to run it.")

	<-ctrlC

	err = serviceTunnel.Stop()
	if err != nil {
		exit.WithError("error stopping tunnel", err)
	}
}

func openURLs(svc string, urls []string) {
	for _, u := range urls {
		_, err := url.Parse(u)
		if err != nil {
			glog.Warningf("failed to parse url %q: %v (will not open)", u, err)
			out.String(fmt.Sprintf("%s\n", u))
			continue
		}

		if serviceURLMode {
			out.String(fmt.Sprintf("%s\n", u))
			continue
		}

		out.T(out.Celebrate, "Opening service {{.namespace_name}}/{{.service_name}} in default browser...", out.V{"namespace_name": namespace, "service_name": svc})
		if err := browser.OpenURL(u); err != nil {
			exit.WithError(fmt.Sprintf("open url failed: %s", u), err)
		}
	}
}
