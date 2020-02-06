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

package kubeadm

import (
	"bytes"
	"os/exec"
	"path"

	"fmt"
	"net"

	// WARNING: Do not use path/filepath in this package unless you want bizarre Windows paths

	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/docker/machine/libmachine"
	"github.com/docker/machine/libmachine/state"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	kconst "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/minikube/pkg/drivers/kic"
	"k8s.io/minikube/pkg/drivers/kic/oci"
	"k8s.io/minikube/pkg/kapi"
	"k8s.io/minikube/pkg/minikube/bootstrapper"
	"k8s.io/minikube/pkg/minikube/bootstrapper/bsutil"
	"k8s.io/minikube/pkg/minikube/bootstrapper/bsutil/kverify"
	"k8s.io/minikube/pkg/minikube/bootstrapper/images"
	"k8s.io/minikube/pkg/minikube/command"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/cruntime"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/out"
	"k8s.io/minikube/pkg/minikube/vmpath"
	"k8s.io/minikube/pkg/util/retry"
)

// Bootstrapper is a bootstrapper using kubeadm
type Bootstrapper struct {
	c           command.Runner
	k8sClient   *kubernetes.Clientset // kubernetes client used to verify pods inside cluster
	contextName string
}

// NewBootstrapper creates a new kubeadm.Bootstrapper
func NewBootstrapper(api libmachine.API) (*Bootstrapper, error) {
	name := viper.GetString(config.MachineProfile)
	h, err := api.Load(name)
	if err != nil {
		return nil, errors.Wrap(err, "getting api client")
	}
	runner, err := machine.CommandRunner(h)
	if err != nil {
		return nil, errors.Wrap(err, "command runner")
	}
	return &Bootstrapper{c: runner, contextName: name, k8sClient: nil}, nil
}

// GetKubeletStatus returns the kubelet status
func (k *Bootstrapper) GetKubeletStatus() (string, error) {
	rr, err := k.c.RunCmd(exec.Command("sudo", "systemctl", "is-active", "kubelet"))
	if err != nil {
		// Do not return now, as we still have parsing to do!
		glog.Warningf("%s returned error: %v", rr.Command(), err)
	}
	s := strings.TrimSpace(rr.Stdout.String())
	glog.Infof("kubelet is-active: %s", s)
	switch s {
	case "active":
		return state.Running.String(), nil
	case "inactive":
		return state.Stopped.String(), nil
	case "activating":
		return state.Starting.String(), nil
	}
	return state.Error.String(), nil
}

// GetAPIServerStatus returns the api-server status
func (k *Bootstrapper) GetAPIServerStatus(ip net.IP, port int) (string, error) {
	s, err := kverify.APIServerStatus(k.c, ip, port)
	if err != nil {
		return state.Error.String(), err
	}
	return s.String(), nil
}

// LogCommands returns a map of log type to a command which will display that log.
func (k *Bootstrapper) LogCommands(o bootstrapper.LogOptions) map[string]string {
	var kubelet strings.Builder
	kubelet.WriteString("sudo journalctl -u kubelet")
	if o.Lines > 0 {
		kubelet.WriteString(fmt.Sprintf(" -n %d", o.Lines))
	}
	if o.Follow {
		kubelet.WriteString(" -f")
	}

	var dmesg strings.Builder
	dmesg.WriteString("sudo dmesg -PH -L=never --level warn,err,crit,alert,emerg")
	if o.Follow {
		dmesg.WriteString(" --follow")
	}
	if o.Lines > 0 {
		dmesg.WriteString(fmt.Sprintf(" | tail -n %d", o.Lines))
	}
	return map[string]string{
		"kubelet": kubelet.String(),
		"dmesg":   dmesg.String(),
	}
}

// createCompatSymlinks creates compatibility symlinks to transition running services to new directory structures
func (k *Bootstrapper) createCompatSymlinks() error {
	legacyEtcd := "/data/minikube"

	if _, err := k.c.RunCmd(exec.Command("sudo", "test", "-d", legacyEtcd)); err != nil {
		glog.Infof("%s skipping compat symlinks: %v", legacyEtcd, err)
		return nil
	}
	glog.Infof("Found %s, creating compatibility symlinks ...", legacyEtcd)

	c := exec.Command("sudo", "ln", "-s", legacyEtcd, bsutil.EtcdDataDir())
	if rr, err := k.c.RunCmd(c); err != nil {
		return errors.Wrapf(err, "create symlink failed: %s", rr.Command())
	}
	return nil
}

// StartCluster starts the cluster
func (k *Bootstrapper) StartCluster(cfg config.MachineConfig) error {
	err := bsutil.ExistingConfig(k.c)
	if err == nil { // if there is an existing cluster don't reconfigure it
		return k.restartCluster(cfg)
	}
	glog.Infof("existence check: %v", err)

	start := time.Now()
	glog.Infof("StartCluster: %+v", cfg)
	defer func() {
		glog.Infof("StartCluster complete in %s", time.Since(start))
	}()

	version, err := bsutil.ParseKubernetesVersion(cfg.KubernetesConfig.KubernetesVersion)
	if err != nil {
		return errors.Wrap(err, "parsing kubernetes version")
	}

	extraFlags := bsutil.CreateFlagsFromExtraArgs(cfg.KubernetesConfig.ExtraOptions)
	r, err := cruntime.New(cruntime.Config{Type: cfg.KubernetesConfig.ContainerRuntime})
	if err != nil {
		return err
	}

	cp, err := config.PrimaryControlPlane(cfg)
	if err != nil {
		return err
	}

	ignore := []string{
		fmt.Sprintf("DirAvailable-%s", strings.Replace(vmpath.GuestManifestsDir, "/", "-", -1)),
		fmt.Sprintf("DirAvailable-%s", strings.Replace(vmpath.GuestPersistentDir, "/", "-", -1)),
		fmt.Sprintf("DirAvailable-%s", strings.Replace(bsutil.EtcdDataDir(), "/", "-", -1)),
		"FileAvailable--etc-kubernetes-manifests-kube-scheduler.yaml",
		"FileAvailable--etc-kubernetes-manifests-kube-apiserver.yaml",
		"FileAvailable--etc-kubernetes-manifests-kube-controller-manager.yaml",
		"FileAvailable--etc-kubernetes-manifests-etcd.yaml",
		"Port-10250", // For "none" users who already have a kubelet online
		"Swap",       // For "none" users who have swap configured
		"SystemVerification",
	}
	ignore = append(ignore, bsutil.SkipAdditionalPreflights[r.Name()]...)

	// Allow older kubeadm versions to function with newer Docker releases.
	// For kic on linux example error: "modprobe: FATAL: Module configs not found in directory /lib/modules/5.2.17-1rodete3-amd64"
	if version.LT(semver.MustParse("1.13.0")) || driver.IsKIC(cfg.VMDriver) {
		glog.Infof("Older Kubernetes release detected (%s), disabling SystemVerification check.", version)
		ignore = append(ignore, "SystemVerification")
	}

	if driver.IsKIC(cfg.VMDriver) { // to bypass this error: /proc/sys/net/bridge/bridge-nf-call-iptables does not exist
		ignore = append(ignore, "FileContent--proc-sys-net-bridge-bridge-nf-call-iptables")
	}

	if driver.BareMetal(cfg.VMDriver) {
		if err := stopConflictingProcesses(k.c, r); err != nil {
			glog.Errorf("unable to stop processes: %v", err)
		}
	}

	c := exec.Command("/bin/bash", "-c", fmt.Sprintf("%s init --config %s %s --ignore-preflight-errors=%s", bsutil.InvokeKubeadm(cfg.KubernetesConfig.KubernetesVersion), bsutil.KubeadmYamlPath, extraFlags, strings.Join(ignore, ",")))
	rr, err := k.c.RunCmd(c)
	if err != nil {
		return errors.Wrapf(err, "init failed. output: %q", rr.Output())
	}

	if driver.IsKIC(cfg.VMDriver) {
		if err := k.applyKicOverlay(cfg); err != nil {
			return errors.Wrap(err, "applying kic overlay network")
		}
	}

	if !driver.IsKIC(cfg.VMDriver) { // TODO: skip for both after verifications https://github.com/kubernetes/minikube/issues/6239
		glog.Infof("Configuring cluster permissions ...")
		elevate := func() error {
			client, err := k.client(cp.IP, cp.Port)
			if err != nil {
				return err
			}
			return bsutil.ElevateKubeSystemPrivileges(client)
		}

		if err := retry.Expo(elevate, time.Millisecond*500, 120*time.Second); err != nil {
			return errors.Wrap(err, "timed out waiting to elevate kube-system RBAC privileges")
		}
	}

	if err := bsutil.AdjustResourceLimits(k.c); err != nil {
		glog.Warningf("unable to adjust resource limits: %v", err)
	}

	return nil
}

// client sets and returns a Kubernetes client to use to speak to a kubeadm launched apiserver
func (k *Bootstrapper) client(ip string, port int) (*kubernetes.Clientset, error) {
	if k.k8sClient != nil {
		return k.k8sClient, nil
	}

	cc, err := kapi.ClientConfig(k.contextName)
	if err != nil {
		return nil, errors.Wrap(err, "client config")
	}

	endpoint := fmt.Sprintf("https://%s", net.JoinHostPort(ip, strconv.Itoa(port)))
	if cc.Host != endpoint {
		glog.Errorf("Overriding stale ClientConfig host %s with %s", cc.Host, endpoint)
		cc.Host = endpoint
	}
	c, err := kubernetes.NewForConfig(cc)
	if err == nil {
		k.k8sClient = c
	}
	return c, err
}

// WaitForCluster blocks until the cluster appears to be healthy
func (k *Bootstrapper) WaitForCluster(cfg config.MachineConfig, timeout time.Duration) error {
	start := time.Now()
	out.T(out.Waiting, "Waiting for cluster to come online ...")
	cp, err := config.PrimaryControlPlane(cfg)
	if err != nil {
		return err
	}
	if err := kverify.APIServerProcess(k.c, start, timeout); err != nil {
		return err
	}

	ip := cp.IP
	port := cp.Port
	if driver.IsKIC(cfg.VMDriver) {
		ip = kic.DefaultBindIPV4
		port, err = oci.HostPortBinding(cfg.VMDriver, cfg.Name, port)
		if err != nil {
			return errors.Wrapf(err, "get host-bind port %d for container %s", port, cfg.Name)
		}
	}
	if err := kverify.APIServerIsRunning(start, ip, port, timeout); err != nil {
		return err
	}

	c, err := k.client(ip, port)
	if err != nil {
		return errors.Wrap(err, "get k8s client")
	}

	return kverify.SystemPods(c, start, timeout)
}

// restartCluster restarts the Kubernetes cluster configured by kubeadm
func (k *Bootstrapper) restartCluster(cfg config.MachineConfig) error {
	glog.Infof("restartCluster start")

	start := time.Now()
	defer func() {
		glog.Infof("restartCluster took %s", time.Since(start))
	}()

	version, err := bsutil.ParseKubernetesVersion(cfg.KubernetesConfig.KubernetesVersion)
	if err != nil {
		return errors.Wrap(err, "parsing kubernetes version")
	}

	phase := "alpha"
	controlPlane := "controlplane"
	if version.GTE(semver.MustParse("1.13.0")) {
		phase = "init"
		controlPlane = "control-plane"
	}

	if err := k.createCompatSymlinks(); err != nil {
		glog.Errorf("failed to create compat symlinks: %v", err)
	}

	baseCmd := fmt.Sprintf("%s %s", bsutil.InvokeKubeadm(cfg.KubernetesConfig.KubernetesVersion), phase)
	cmds := []string{
		fmt.Sprintf("%s phase certs all --config %s", baseCmd, bsutil.KubeadmYamlPath),
		fmt.Sprintf("%s phase kubeconfig all --config %s", baseCmd, bsutil.KubeadmYamlPath),
		fmt.Sprintf("%s phase %s all --config %s", baseCmd, controlPlane, bsutil.KubeadmYamlPath),
		fmt.Sprintf("%s phase etcd local --config %s", baseCmd, bsutil.KubeadmYamlPath),
	}

	// Run commands one at a time so that it is easier to root cause failures.
	for _, c := range cmds {
		rr, err := k.c.RunCmd(exec.Command("/bin/bash", "-c", c))
		if err != nil {
			return errors.Wrapf(err, "running cmd: %s", rr.Command())
		}
	}

	// We must ensure that the apiserver is healthy before proceeding
	if err := kverify.APIServerProcess(k.c, time.Now(), kconst.DefaultControlPlaneTimeout); err != nil {
		return errors.Wrap(err, "apiserver healthz")
	}

	for _, n := range cfg.Nodes {
		ip := n.IP
		port := n.Port
		if driver.IsKIC(cfg.VMDriver) {
			ip = kic.DefaultBindIPV4
			port, err = oci.HostPortBinding(cfg.VMDriver, cfg.Name, port)
			if err != nil {
				return errors.Wrapf(err, "get host-bind port %d for container %s", port, cfg.Name)
			}
		}
		client, err := k.client(ip, port)
		if err != nil {
			return errors.Wrap(err, "getting k8s client")
		}

		if err := kverify.SystemPods(client, time.Now(), kconst.DefaultControlPlaneTimeout); err != nil {
			return errors.Wrap(err, "system pods")
		}

		// Explicitly re-enable kubeadm addons (proxy, coredns) so that they will check for IP or configuration changes.
		if rr, err := k.c.RunCmd(exec.Command("/bin/bash", "-c", fmt.Sprintf("%s phase addon all --config %s", baseCmd, bsutil.KubeadmYamlPath))); err != nil {
			return errors.Wrapf(err, fmt.Sprintf("addon phase cmd:%q", rr.Command()))
		}

		if err := bsutil.AdjustResourceLimits(k.c); err != nil {
			glog.Warningf("unable to adjust resource limits: %v", err)
		}
	}
	return nil
}

// DeleteCluster removes the components that were started earlier
func (k *Bootstrapper) DeleteCluster(k8s config.KubernetesConfig) error {
	version, err := bsutil.ParseKubernetesVersion(k8s.KubernetesVersion)
	if err != nil {
		return errors.Wrap(err, "parsing kubernetes version")
	}

	cmd := fmt.Sprintf("%s reset --force", bsutil.InvokeKubeadm(k8s.KubernetesVersion))
	if version.LT(semver.MustParse("1.11.0")) {
		cmd = fmt.Sprintf("%s reset", bsutil.InvokeKubeadm(k8s.KubernetesVersion))
	}

	if rr, err := k.c.RunCmd(exec.Command("/bin/bash", "-c", cmd)); err != nil {
		return errors.Wrapf(err, "kubeadm reset: cmd: %q", rr.Command())
	}

	return nil
}

// PullImages downloads images that will be used by Kubernetes
func (k *Bootstrapper) PullImages(k8s config.KubernetesConfig) error {
	version, err := bsutil.ParseKubernetesVersion(k8s.KubernetesVersion)
	if err != nil {
		return errors.Wrap(err, "parsing kubernetes version")
	}
	if version.LT(semver.MustParse("1.11.0")) {
		return fmt.Errorf("pull command is not supported by kubeadm v%s", version)
	}

	rr, err := k.c.RunCmd(exec.Command("/bin/bash", "-c", fmt.Sprintf("%s config images pull --config %s", bsutil.InvokeKubeadm(k8s.KubernetesVersion), bsutil.KubeadmYamlPath)))
	if err != nil {
		return errors.Wrapf(err, "running cmd: %q", rr.Command())
	}
	return nil
}

// SetupCerts sets up certificates within the cluster.
func (k *Bootstrapper) SetupCerts(k8s config.KubernetesConfig, n config.Node) error {
	return bootstrapper.SetupCerts(k.c, k8s, n)
}

// UpdateCluster updates the cluster
func (k *Bootstrapper) UpdateCluster(cfg config.MachineConfig) error {
	images, err := images.Kubeadm(cfg.KubernetesConfig.ImageRepository, cfg.KubernetesConfig.KubernetesVersion)
	if err != nil {
		return errors.Wrap(err, "kubeadm images")
	}

	if cfg.KubernetesConfig.ShouldLoadCachedImages {
		if err := machine.LoadImages(&cfg, k.c, images, constants.ImageCacheDir); err != nil {
			out.FailureT("Unable to load cached images: {{.error}}", out.V{"error": err})
		}
	}
	r, err := cruntime.New(cruntime.Config{Type: cfg.KubernetesConfig.ContainerRuntime,
		Runner: k.c, Socket: cfg.KubernetesConfig.CRISocket})
	if err != nil {
		return errors.Wrap(err, "runtime")
	}
	kubeadmCfg, err := bsutil.GenerateKubeadmYAML(cfg, r)
	if err != nil {
		return errors.Wrap(err, "generating kubeadm cfg")
	}

	// TODO: multiple nodes
	kubeletCfg, err := bsutil.NewKubeletConfig(cfg, cfg.Nodes[0], r)
	if err != nil {
		return errors.Wrap(err, "generating kubelet config")
	}

	kubeletService, err := bsutil.NewKubeletService(cfg.KubernetesConfig)
	if err != nil {
		return errors.Wrap(err, "generating kubelet service")
	}

	glog.Infof("kubelet %s config:\n%+v", kubeletCfg, cfg.KubernetesConfig)

	stopCmd := exec.Command("/bin/bash", "-c", "pgrep kubelet && sudo systemctl stop kubelet")
	// stop kubelet to avoid "Text File Busy" error
	if rr, err := k.c.RunCmd(stopCmd); err != nil {
		glog.Warningf("unable to stop kubelet: %s command: %q output: %q", err, rr.Command(), rr.Output())
	}

	if err := bsutil.TransferBinaries(cfg.KubernetesConfig, k.c); err != nil {
		return errors.Wrap(err, "downloading binaries")
	}

	var cniFile []byte
	if cfg.KubernetesConfig.EnableDefaultCNI {
		cniFile = []byte(defaultCNIConfig)
	}
	files := bsutil.ConfigFileAssets(cfg.KubernetesConfig, kubeadmCfg, kubeletCfg, kubeletService, cniFile)

	// Combine mkdir request into a single call to reduce load
	dirs := []string{}
	for _, f := range files {
		dirs = append(dirs, f.GetTargetDir())
	}
	args := append([]string{"mkdir", "-p"}, dirs...)
	if _, err := k.c.RunCmd(exec.Command("sudo", args...)); err != nil {
		return errors.Wrap(err, "mkdir")
	}

	for _, f := range files {
		if err := k.c.Copy(f); err != nil {
			return errors.Wrapf(err, "copy")
		}
	}

	if _, err := k.c.RunCmd(exec.Command("/bin/bash", "-c", "sudo systemctl daemon-reload && sudo systemctl start kubelet")); err != nil {
		return errors.Wrap(err, "starting kubelet")
	}
	return nil
}

// applyKicOverlay applies the CNI plugin needed to make kic work
func (k *Bootstrapper) applyKicOverlay(cfg config.MachineConfig) error {
	cmd := exec.Command("sudo",
		path.Join("/var/lib/minikube/binaries", cfg.KubernetesConfig.KubernetesVersion, "kubectl"), "create", "--kubeconfig=/var/lib/minikube/kubeconfig",
		"-f", "-")
	b := bytes.Buffer{}
	if err := kicCNIConfig.Execute(&b, struct{ ImageName string }{ImageName: kic.OverlayImage}); err != nil {
		return err
	}
	cmd.Stdin = bytes.NewReader(b.Bytes())
	if rr, err := k.c.RunCmd(cmd); err != nil {
		return errors.Wrapf(err, "cmd: %s output: %s", rr.Command(), rr.Output())
	}
	return nil
}

// stops conflicting processes, useful for none driver
func stopConflictingProcesses(r command.Runner, cr cruntime.Manager) error {
	glog.Infof("stopping kubelet & kube-system containers")

	cmd := exec.Command("sudo", "systemctl", "stop", "kubelet.service")
	if _, err := r.RunCmd(cmd); err != nil {
		glog.Errorf("stop kubelet: %v", err)
	}

	containers, err := cr.ListContainers(cruntime.ListOptions{Namespaces: []string{"kube-system"}})
	if err != nil {
		glog.Warningf("unable to list kube-system containers: %v", err)
	}
	if len(containers) > 0 {
		glog.Warningf("found %d kube-system containers to stop", len(containers))
		if err := cr.StopContainers(containers); err != nil {
			return err
		}
	}
	return nil
}
