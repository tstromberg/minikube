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

package node

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/blang/semver"
	"github.com/docker/machine/libmachine"
	"github.com/docker/machine/libmachine/host"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	cmdcfg "k8s.io/minikube/cmd/minikube/cmd/config"
	"k8s.io/minikube/pkg/drivers/kic/oci"
	"k8s.io/minikube/pkg/minikube/bootstrapper"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/cruntime"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/kubeconfig"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/out"
	"k8s.io/minikube/pkg/util/lock"
)

var (
	// DockerEnv contains the environment variables
	DockerEnv []string
	// DockerOpt contains the option parameters
	DockerOpt []string
	// ExtraOptions contains extra options (if any)
	ExtraOptions config.ExtraOptionSlice
	// AddonList contains the list of addons
	AddonList []string
)

// configureRuntimes does what needs to happen to get a runtime going.
func configureRuntimes(runner cruntime.CommandRunner, drvName string, k8s config.KubernetesConfig, kv semver.Version) cruntime.Manager {
	co := cruntime.Config{
		Type:   viper.GetString(containerRuntime),
		Runner: runner, ImageRepository: k8s.ImageRepository,
		KubernetesVersion: kv,
	}
	cr, err := cruntime.New(co)
	if err != nil {
		exit.WithError("Failed runtime", err)
	}

	disableOthers := true
	if driver.BareMetal(drvName) {
		disableOthers = false
	}

	// Preload is overly invasive for bare metal, and caching is not meaningful. KIC handled elsewhere.
	if driver.IsVM(drvName) {
		if err := cr.Preload(k8s); err != nil {
			switch err.(type) {
			case *cruntime.ErrISOFeature:
				out.T(out.Tip, "Existing disk is missing new features ({{.error}}). To upgrade, run 'minikube delete'", out.V{"error": err})
			default:
				glog.Warningf("%s preload failed: %v, falling back to caching images", cr.Name(), err)
			}

			if err := machine.CacheImagesForBootstrapper(k8s.ImageRepository, k8s.KubernetesVersion, viper.GetString(cmdcfg.Bootstrapper)); err != nil {
				exit.WithError("Failed to cache images", err)
			}
		}
	}

	err = cr.Enable(disableOthers)
	if err != nil {
		exit.WithError("Failed to enable container runtime", err)
	}

	return cr
}

func showVersionInfo(k8sVersion string, cr cruntime.Manager) {
	version, _ := cr.Version()
	out.T(cr.Style(), "Preparing Kubernetes {{.k8sVersion}} on {{.runtime}} {{.runtimeVersion}} ...", out.V{"k8sVersion": k8sVersion, "runtime": cr.Name(), "runtimeVersion": version})
	for _, v := range DockerOpt {
		out.T(out.Option, "opt {{.docker_option}}", out.V{"docker_option": v})
	}
	for _, v := range DockerEnv {
		out.T(out.Option, "env {{.docker_env}}", out.V{"docker_env": v})
	}
}

// setupKubeAdm adds any requested files into the VM before Kubernetes is started
func setupKubeAdm(mAPI libmachine.API, cfg config.ClusterConfig, node config.Node) bootstrapper.Bootstrapper {
	bs, err := cluster.Bootstrapper(mAPI, viper.GetString(cmdcfg.Bootstrapper), cfg, node)
	if err != nil {
		exit.WithError("Failed to get bootstrapper", err)
	}
	for _, eo := range ExtraOptions {
		out.T(out.Option, "{{.extra_option_component_name}}.{{.key}}={{.value}}", out.V{"extra_option_component_name": eo.Component, "key": eo.Key, "value": eo.Value})
	}
	// Loads cached images, generates config files, download binaries
	if err := bs.UpdateCluster(cfg); err != nil {
		exit.WithError("Failed to update cluster", err)
	}

	if err := bs.SetupCerts(cfg.KubernetesConfig, node); err != nil {
		exit.WithError("Failed to setup certs", err)
	}
	return bs
}

func setupKubeconfig(h *host.Host, cc *config.ClusterConfig, n *config.Node, clusterName string) (*kubeconfig.Settings, error) {
	addr, err := apiServerURL(*h, *cc, *n)
	if err != nil {
		exit.WithError("Failed to get api server URL", err)
	}

	kcs := &kubeconfig.Settings{
		ClusterName:          clusterName,
		ClusterServerAddress: addr,
		ClientCertificate:    filepath.Join(localpath.Profile(cc.Name), "client.crt"),
		ClientKey:            filepath.Join(localpath.Profile(cc.Name), "client.key"),
		CertificateAuthority: localpath.MakeMiniPath("ca.crt"),
		KeepContext:          viper.GetBool(keepContext),
		EmbedCerts:           viper.GetBool(embedCerts),
	}

	kcs.SetPath(kubeconfig.PathFromEnv())
	if err := kubeconfig.Update(kcs); err != nil {
		return kcs, err
	}
	return kcs, nil
}

// apiServerURL returns a URL to end user can reach to the api server
func apiServerURL(h host.Host, cc config.ClusterConfig, n config.Node) (string, error) {
	hostname := ""
	port := n.Port
	var err error
	if driver.IsKIC(h.DriverName) {
		// for kic drivers we use 127.0.0.1 instead of node IP,
		// because of Docker on MacOs limitations for reaching to container's IP.
		hostname = oci.DefaultBindIPV4
		port, err = oci.ForwardedPort(h.DriverName, h.Name, port)
		if err != nil {
			return "", errors.Wrap(err, "host port binding")
		}
	} else {
		hostname, err = h.Driver.GetIP()
		if err != nil {
			return "", errors.Wrap(err, "get ip")
		}
	}

	if cc.KubernetesConfig.APIServerName != constants.APIServerName {
		hostname = cc.KubernetesConfig.APIServerName
	}
	return fmt.Sprintf("https://" + net.JoinHostPort(hostname, strconv.Itoa(port))), nil
}

// configureMounts configures any requested filesystem mounts
func configureMounts() {
	if !viper.GetBool(createMount) {
		return
	}

	out.T(out.Mounting, "Creating mount {{.name}} ...", out.V{"name": viper.GetString(mountString)})
	path := os.Args[0]
	mountDebugVal := 0
	if glog.V(8) {
		mountDebugVal = 1
	}
	mountCmd := exec.Command(path, "mount", fmt.Sprintf("--v=%d", mountDebugVal), viper.GetString(mountString))
	mountCmd.Env = append(os.Environ(), constants.IsMinikubeChildProcess+"=true")
	if glog.V(8) {
		mountCmd.Stdout = os.Stdout
		mountCmd.Stderr = os.Stderr
	}
	if err := mountCmd.Start(); err != nil {
		exit.WithError("Error starting mount", err)
	}
	if err := lock.WriteFile(filepath.Join(localpath.MiniPath(), constants.MountProcessFileName), []byte(strconv.Itoa(mountCmd.Process.Pid)), 0644); err != nil {
		exit.WithError("Error writing mount pid", err)
	}
}
