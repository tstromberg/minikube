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
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/golang/glog"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/cpu"
	gopshost "github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cmdcfg "k8s.io/minikube/cmd/minikube/cmd/config"
	"k8s.io/minikube/pkg/drivers/kic/oci"
	"k8s.io/minikube/pkg/minikube/bootstrapper/bsutil"
	"k8s.io/minikube/pkg/minikube/bootstrapper/images"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/cruntime"
	"k8s.io/minikube/pkg/minikube/download"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/kubeconfig"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/node"
	"k8s.io/minikube/pkg/minikube/notify"
	"k8s.io/minikube/pkg/minikube/out"
	"k8s.io/minikube/pkg/minikube/proxy"
	"k8s.io/minikube/pkg/minikube/registry"
	"k8s.io/minikube/pkg/minikube/translate"
	pkgutil "k8s.io/minikube/pkg/util"
	"k8s.io/minikube/pkg/version"
)

const (
	isoURL                  = "iso-url"
	memory                  = "memory"
	cpus                    = "cpus"
	humanReadableDiskSize   = "disk-size"
	nfsSharesRoot           = "nfs-shares-root"
	nfsShare                = "nfs-share"
	kubernetesVersion       = "kubernetes-version"
	hostOnlyCIDR            = "host-only-cidr"
	containerRuntime        = "container-runtime"
	criSocket               = "cri-socket"
	networkPlugin           = "network-plugin"
	enableDefaultCNI        = "enable-default-cni"
	hypervVirtualSwitch     = "hyperv-virtual-switch"
	hypervUseExternalSwitch = "hyperv-use-external-switch"
	hypervExternalAdapter   = "hyperv-external-adapter"
	kvmNetwork              = "kvm-network"
	kvmQemuURI              = "kvm-qemu-uri"
	kvmGPU                  = "kvm-gpu"
	kvmHidden               = "kvm-hidden"
	minikubeEnvPrefix       = "MINIKUBE"
	installAddons           = "install-addons"
	defaultDiskSize         = "20000mb"
	keepContext             = "keep-context"
	createMount             = "mount"
	featureGates            = "feature-gates"
	apiServerName           = "apiserver-name"
	apiServerPort           = "apiserver-port"
	dnsDomain               = "dns-domain"
	serviceCIDR             = "service-cluster-ip-range"
	imageRepository         = "image-repository"
	imageMirrorCountry      = "image-mirror-country"
	mountString             = "mount-string"
	disableDriverMounts     = "disable-driver-mounts"
	cacheImages             = "cache-images"
	uuid                    = "uuid"
	vpnkitSock              = "hyperkit-vpnkit-sock"
	vsockPorts              = "hyperkit-vsock-ports"
	embedCerts              = "embed-certs"
	noVTXCheck              = "no-vtx-check"
	downloadOnly            = "download-only"
	dnsProxy                = "dns-proxy"
	hostDNSResolver         = "host-dns-resolver"
	waitUntilHealthy        = "wait"
	force                   = "force"
	dryRun                  = "dry-run"
	interactive             = "interactive"
	waitTimeout             = "wait-timeout"
	nativeSSH               = "native-ssh"
	minUsableMem            = 1024 // Kubernetes will not start with less than 1GB
	minRecommendedMem       = 2000 // Warn at no lower than existing configurations
	minimumCPUS             = 2
	minimumDiskSize         = "2000mb"
	autoUpdate              = "auto-update-drivers"
	hostOnlyNicType         = "host-only-nic-type"
	natNicType              = "nat-nic-type"
)

var (
	registryMirror   []string
	insecureRegistry []string
	apiServerNames   []string
	apiServerIPs     []net.IP
)

func init() {
	initMinikubeFlags()
	initKubernetesFlags()
	initDriverFlags()
	initNetworkingFlags()
	if err := viper.BindPFlags(startCmd.Flags()); err != nil {
		exit.WithError("unable to bind flags", err)
	}
}

// initMinikubeFlags includes commandline flags for minikube.
func initMinikubeFlags() {
	viper.SetEnvPrefix(minikubeEnvPrefix)
	// Replaces '-' in flags with '_' in env variables
	// e.g. iso-url => $ENVPREFIX_ISO_URL
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	startCmd.Flags().Bool(force, false, "Force minikube to perform possibly dangerous operations")
	startCmd.Flags().Bool(interactive, true, "Allow user prompts for more information")
	startCmd.Flags().Bool(dryRun, false, "dry-run mode. Validates configuration, but does not mutate system state")

	startCmd.Flags().Int(cpus, 2, "Number of CPUs allocated to Kubernetes.")
	startCmd.Flags().String(memory, "", "Amount of RAM to allocate to Kubernetes (format: <number>[<unit>], where unit = b, k, m or g).")
	startCmd.Flags().String(humanReadableDiskSize, defaultDiskSize, "Disk size allocated to the minikube VM (format: <number>[<unit>], where unit = b, k, m or g).")
	startCmd.Flags().Bool(downloadOnly, false, "If true, only download and cache files for later use - don't install or start anything.")
	startCmd.Flags().Bool(cacheImages, true, "If true, cache docker images for the current bootstrapper and load them into the machine. Always false with --driver=none.")
	startCmd.Flags().StringSlice(isoURL, download.DefaultISOURLs(), "Locations to fetch the minikube ISO from.")
	startCmd.Flags().Bool(keepContext, false, "This will keep the existing kubectl context and will create a minikube context.")
	startCmd.Flags().Bool(embedCerts, false, "if true, will embed the certs in kubeconfig.")
	startCmd.Flags().String(containerRuntime, "docker", "The container runtime to be used (docker, crio, containerd).")
	startCmd.Flags().Bool(createMount, false, "This will start the mount daemon and automatically mount files into minikube.")
	startCmd.Flags().String(mountString, constants.DefaultMountDir+":/minikube-host", "The argument to pass the minikube mount command on start.")
	startCmd.Flags().StringArrayVar(&node.AddonList, "addons", nil, "Enable addons. see `minikube addons list` for a list of valid addon names.")
	startCmd.Flags().String(criSocket, "", "The cri socket path to be used.")
	startCmd.Flags().String(networkPlugin, "", "The name of the network plugin.")
	startCmd.Flags().Bool(enableDefaultCNI, false, "Enable the default CNI plugin (/etc/cni/net.d/k8s.conf). Used in conjunction with \"--network-plugin=cni\".")
	startCmd.Flags().Bool(waitUntilHealthy, true, "Block until the apiserver is servicing API requests")
	startCmd.Flags().Duration(waitTimeout, 6*time.Minute, "max time to wait per Kubernetes core services to be healthy.")
	startCmd.Flags().Bool(nativeSSH, true, "Use native Golang SSH client (default true). Set to 'false' to use the command line 'ssh' command when accessing the docker machine. Useful for the machine drivers when they will not start with 'Waiting for SSH'.")
	startCmd.Flags().Bool(autoUpdate, true, "If set, automatically updates drivers to the latest version. Defaults to true.")
	startCmd.Flags().Bool(installAddons, true, "If set, install addons. Defaults to true.")
}

// initKubernetesFlags inits the commandline flags for kubernetes related options
func initKubernetesFlags() {
	startCmd.Flags().String(kubernetesVersion, "", "The kubernetes version that the minikube VM will use (ex: v1.2.3)")
	startCmd.Flags().Var(&node.ExtraOptions, "extra-config",
		`A set of key=value pairs that describe configuration that may be passed to different components.
		The key should be '.' separated, and the first part before the dot is the component to apply the configuration to.
		Valid components are: kubelet, kubeadm, apiserver, controller-manager, etcd, proxy, scheduler
		Valid kubeadm parameters: `+fmt.Sprintf("%s, %s", strings.Join(bsutil.KubeadmExtraArgsWhitelist[bsutil.KubeadmCmdParam], ", "), strings.Join(bsutil.KubeadmExtraArgsWhitelist[bsutil.KubeadmConfigParam], ",")))
	startCmd.Flags().String(featureGates, "", "A set of key=value pairs that describe feature gates for alpha/experimental features.")
	startCmd.Flags().String(dnsDomain, constants.ClusterDNSDomain, "The cluster dns domain name used in the kubernetes cluster")
	startCmd.Flags().Int(apiServerPort, constants.APIServerPort, "The apiserver listening port")
	startCmd.Flags().String(apiServerName, constants.APIServerName, "The apiserver name which is used in the generated certificate for kubernetes.  This can be used if you want to make the apiserver available from outside the machine")
	startCmd.Flags().StringArrayVar(&apiServerNames, "apiserver-names", nil, "A set of apiserver names which are used in the generated certificate for kubernetes.  This can be used if you want to make the apiserver available from outside the machine")
	startCmd.Flags().IPSliceVar(&apiServerIPs, "apiserver-ips", nil, "A set of apiserver IP Addresses which are used in the generated certificate for kubernetes.  This can be used if you want to make the apiserver available from outside the machine")
}

// initDriverFlags inits the commandline flags for vm drivers
func initDriverFlags() {
	startCmd.Flags().String("driver", "", fmt.Sprintf("Driver is one of: %v (defaults to auto-detect)", driver.DisplaySupportedDrivers()))
	startCmd.Flags().String("vm-driver", "", "DEPRECATED, use `driver` instead.")
	startCmd.Flags().Bool(disableDriverMounts, false, "Disables the filesystem mounts provided by the hypervisors")

	// kvm2
	startCmd.Flags().String(kvmNetwork, "default", "The KVM network name. (kvm2 driver only)")
	startCmd.Flags().String(kvmQemuURI, "qemu:///system", "The KVM QEMU connection URI. (kvm2 driver only)")
	startCmd.Flags().Bool(kvmGPU, false, "Enable experimental NVIDIA GPU support in minikube")
	startCmd.Flags().Bool(kvmHidden, false, "Hide the hypervisor signature from the guest in minikube (kvm2 driver only)")

	// virtualbox
	startCmd.Flags().String(hostOnlyCIDR, "192.168.99.1/24", "The CIDR to be used for the minikube VM (virtualbox driver only)")
	startCmd.Flags().Bool(dnsProxy, false, "Enable proxy for NAT DNS requests (virtualbox driver only)")
	startCmd.Flags().Bool(hostDNSResolver, true, "Enable host resolver for NAT DNS requests (virtualbox driver only)")
	startCmd.Flags().Bool(noVTXCheck, false, "Disable checking for the availability of hardware virtualization before the vm is started (virtualbox driver only)")
	startCmd.Flags().String(hostOnlyNicType, "virtio", "NIC Type used for host only network. One of Am79C970A, Am79C973, 82540EM, 82543GC, 82545EM, or virtio (virtualbox driver only)")
	startCmd.Flags().String(natNicType, "virtio", "NIC Type used for host only network. One of Am79C970A, Am79C973, 82540EM, 82543GC, 82545EM, or virtio (virtualbox driver only)")

	// hyperkit
	startCmd.Flags().StringSlice(vsockPorts, []string{}, "List of guest VSock ports that should be exposed as sockets on the host (hyperkit driver only)")
	startCmd.Flags().String(uuid, "", "Provide VM UUID to restore MAC address (hyperkit driver only)")
	startCmd.Flags().String(vpnkitSock, "", "Location of the VPNKit socket used for networking. If empty, disables Hyperkit VPNKitSock, if 'auto' uses Docker for Mac VPNKit connection, otherwise uses the specified VSock (hyperkit driver only)")
	startCmd.Flags().StringSlice(nfsShare, []string{}, "Local folders to share with Guest via NFS mounts (hyperkit driver only)")
	startCmd.Flags().String(nfsSharesRoot, "/nfsshares", "Where to root the NFS Shares, defaults to /nfsshares (hyperkit driver only)")

	// hyperv
	startCmd.Flags().String(hypervVirtualSwitch, "", "The hyperv virtual switch name. Defaults to first found. (hyperv driver only)")
	startCmd.Flags().Bool(hypervUseExternalSwitch, false, "Whether to use external switch over Default Switch if virtual switch not explicitly specified. (hyperv driver only)")
	startCmd.Flags().String(hypervExternalAdapter, "", "External Adapter on which external switch will be created if no external switch is found. (hyperv driver only)")
}

// initNetworkingFlags inits the commandline flags for connectivity related flags for start
func initNetworkingFlags() {
	startCmd.Flags().StringSliceVar(&insecureRegistry, "insecure-registry", nil, "Insecure Docker registries to pass to the Docker daemon.  The default service CIDR range will automatically be added.")
	startCmd.Flags().StringSliceVar(&registryMirror, "registry-mirror", nil, "Registry mirrors to pass to the Docker daemon")
	startCmd.Flags().String(imageRepository, "", "Alternative image repository to pull docker images from. This can be used when you have limited access to gcr.io. Set it to \"auto\" to let minikube decide one for you. For Chinese mainland users, you may use local gcr.io mirrors such as registry.cn-hangzhou.aliyuncs.com/google_containers")
	startCmd.Flags().String(imageMirrorCountry, "", "Country code of the image mirror to be used. Leave empty to use the global one. For Chinese mainland users, set it to cn.")
	startCmd.Flags().String(serviceCIDR, constants.DefaultServiceCIDR, "The CIDR to be used for service cluster IPs.")
	startCmd.Flags().StringArrayVar(&node.DockerEnv, "docker-env", nil, "Environment variables to pass to the Docker daemon. (format: key=value)")
	startCmd.Flags().StringArrayVar(&node.DockerOpt, "docker-opt", nil, "Specify arbitrary flags to pass to the Docker daemon. (format: key=value)")
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts a local kubernetes cluster",
	Long:  "Starts a local kubernetes cluster",
	Run:   runStart,
}

// platform generates a user-readable platform message
func platform() string {
	var s strings.Builder

	// Show the distro version if possible
	hi, err := gopshost.Info()
	if err == nil {
		s.WriteString(fmt.Sprintf("%s %s", strings.Title(hi.Platform), hi.PlatformVersion))
		glog.Infof("hostinfo: %+v", hi)
	} else {
		glog.Warningf("gopshost.Info returned error: %v", err)
		s.WriteString(runtime.GOOS)
	}

	vsys, vrole, err := gopshost.Virtualization()
	if err != nil {
		glog.Warningf("gopshost.Virtualization returned error: %v", err)
	} else {
		glog.Infof("virtualization: %s %s", vsys, vrole)
	}

	// This environment is exotic, let's output a bit more.
	if vrole == "guest" || runtime.GOARCH != "amd64" {
		if vsys != "" {
			s.WriteString(fmt.Sprintf(" (%s/%s)", vsys, runtime.GOARCH))
		} else {
			s.WriteString(fmt.Sprintf(" (%s)", runtime.GOARCH))
		}
	}
	return s.String()
}

// runStart handles the executes the flow of "minikube start"
func runStart(cmd *cobra.Command, args []string) {
	displayVersion(version.GetVersion())
	displayEnviron(os.Environ())

	// if --registry-mirror specified when run minikube start,
	// take arg precedence over MINIKUBE_REGISTRY_MIRROR
	// actually this is a hack, because viper 1.0.0 can assign env to variable if StringSliceVar
	// and i can't update it to 1.4.0, it affects too much code
	// other types (like String, Bool) of flag works, so imageRepository, imageMirrorCountry
	// can be configured as MINIKUBE_IMAGE_REPOSITORY and IMAGE_MIRROR_COUNTRY
	// this should be updated to documentation
	if len(registryMirror) == 0 {
		registryMirror = viper.GetStringSlice("registry_mirror")
	}

	existing, err := config.Load(viper.GetString(config.ProfileName))
	if err != nil && !config.IsNotExist(err) {
		exit.WithCodeT(exit.Data, "Unable to load config: {{.error}}", out.V{"error": err})
	}

	ds := selectDriver(existing)
	driverName := ds.Name
	glog.Infof("selected driver: %s", driverName)
	validateDriver(ds, existing)
	err = autoSetDriverOptions(cmd, driverName)
	if err != nil {
		glog.Errorf("Error autoSetOptions : %v", err)
	}

	validateFlags(cmd, driverName)
	validateUser(driverName)

	// Download & update the driver, even in --download-only mode
	if !viper.GetBool(dryRun) {
		updateDriver(driverName)
	}

	k8sVersion := getKubernetesVersion(existing)
	mc, n, err := generateCfgFromFlags(cmd, k8sVersion, driverName)
	if err != nil {
		exit.WithError("Failed to generate config", err)
	}

	// This is about as far as we can go without overwriting config files
	if viper.GetBool(dryRun) {
		out.T(out.DryRun, `dry-run validation complete!`)
		return
	}

	if !driver.BareMetal(driverName) && !driver.IsKIC(driverName) {
		url, err := download.ISO(viper.GetStringSlice(isoURL), cmd.Flags().Changed(isoURL))
		if err != nil {
			exit.WithError("Failed to cache ISO", err)
		}
		mc.MinikubeISO = url
	}

	if viper.GetBool(nativeSSH) {
		ssh.SetDefaultClient(ssh.Native)
	} else {
		ssh.SetDefaultClient(ssh.External)
	}

	var existingAddons map[string]bool
	if viper.GetBool(installAddons) {
		existingAddons = map[string]bool{}
		if existing != nil && existing.Addons != nil {
			existingAddons = existing.Addons
		}
	}
	kubeconfig, err := node.Start(mc, n, true, existingAddons)
	if err != nil {
		exit.WithError("Starting node", err)
	}

	if err := showKubectlInfo(kubeconfig, k8sVersion, mc.Name); err != nil {
		glog.Errorf("kubectl info: %v", err)
	}
}

func updateDriver(driverName string) {
	v, err := version.GetSemverVersion()
	if err != nil {
		out.WarningT("Error parsing minikube version: {{.error}}", out.V{"error": err})
	} else if err := driver.InstallOrUpdate(driverName, localpath.MakeMiniPath("bin"), v, viper.GetBool(interactive), viper.GetBool(autoUpdate)); err != nil {
		out.WarningT("Unable to update {{.driver}} driver: {{.error}}", out.V{"driver": driverName, "error": err})
	}
}

func displayVersion(version string) {
	prefix := ""
	if viper.GetString(config.ProfileName) != constants.DefaultClusterName {
		prefix = fmt.Sprintf("[%s] ", viper.GetString(config.ProfileName))
	}

	versionState := out.Happy
	if notify.MaybePrintUpdateTextFromGithub() {
		versionState = out.Meh
	}

	out.T(versionState, "{{.prefix}}minikube {{.version}} on {{.platform}}", out.V{"prefix": prefix, "version": version, "platform": platform()})
}

// displayEnviron makes the user aware of environment variables that will affect how minikube operates
func displayEnviron(env []string) {
	for _, kv := range env {
		bits := strings.SplitN(kv, "=", 2)
		k := bits[0]
		v := bits[1]
		if strings.HasPrefix(k, "MINIKUBE_") || k == constants.KubeconfigEnvVar {
			out.T(out.Option, "{{.key}}={{.value}}", out.V{"key": k, "value": v})
		}
	}
}

func showKubectlInfo(kcs *kubeconfig.Settings, k8sVersion string, machineName string) error {
	if kcs.KeepContext {
		out.T(out.Kubectl, "To connect to this cluster, use: kubectl --context={{.name}}", out.V{"name": kcs.ClusterName})
	} else {
		out.T(out.Ready, `Done! kubectl is now configured to use "{{.name}}"`, out.V{"name": machineName})
	}

	path, err := exec.LookPath("kubectl")
	if err != nil {
		out.T(out.Tip, "For best results, install kubectl: https://kubernetes.io/docs/tasks/tools/install-kubectl/")
		return nil
	}

	j, err := exec.Command(path, "version", "--client", "--output=json").Output()
	if err != nil {
		return errors.Wrap(err, "exec")
	}

	cv := struct {
		ClientVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"clientVersion"`
	}{}
	err = json.Unmarshal(j, &cv)
	if err != nil {
		return errors.Wrap(err, "unmarshal")
	}

	client, err := semver.Make(strings.TrimPrefix(cv.ClientVersion.GitVersion, version.VersionPrefix))
	if err != nil {
		return errors.Wrap(err, "client semver")
	}

	cluster := semver.MustParse(strings.TrimPrefix(k8sVersion, version.VersionPrefix))
	minorSkew := int(math.Abs(float64(int(client.Minor) - int(cluster.Minor))))
	glog.Infof("kubectl: %s, cluster: %s (minor skew: %d)", client, cluster, minorSkew)

	if client.Major != cluster.Major || minorSkew > 1 {
		out.WarningT("{{.path}} is version {{.client_version}}, and is incompatible with Kubernetes {{.cluster_version}}. You will need to update {{.path}} or use 'minikube kubectl' to connect with this cluster",
			out.V{"path": path, "client_version": client, "cluster_version": cluster})
	}
	return nil
}

func selectDriver(existing *config.ClusterConfig) registry.DriverState {
	// Technically unrelated, but important to perform before detection
	driver.SetLibvirtURI(viper.GetString(kvmQemuURI))

	// By default, the driver is whatever we used last time
	if existing != nil && existing.Driver != "" {
		ds := driver.Status(existing.Driver)
		out.T(out.Sparkle, `Using the {{.driver}} driver based on existing profile`, out.V{"driver": ds.String()})
		return ds
	}

	// Default to looking at the new driver parameter
	if viper.GetString("driver") != "" {
		ds := driver.Status(viper.GetString("driver"))
		out.T(out.Sparkle, `Using the {{.driver}} driver based on user configuration`, out.V{"driver": ds.String()})
		return ds
	}

	// Fallback to old driver parameter
	if viper.GetString("vm-driver") != "" {
		ds := driver.Status(viper.GetString("vm-driver"))
		out.T(out.Sparkle, `Using the {{.driver}} driver based on user configuration`, out.V{"driver": ds.String()})
		return ds
	}

	pick, alts := driver.Suggest(driver.Choices())
	if pick.Name == "" {
		exit.WithCodeT(exit.Config, "Unable to determine a default driver to use. Try specifying --driver, or see https://minikube.sigs.k8s.io/docs/start/")
	}

	if len(alts) > 1 {
		altNames := []string{}
		for _, a := range alts {
			altNames = append(altNames, a.String())
		}
		out.T(out.Sparkle, `Automatically selected the {{.driver}} driver. Other choices: {{.alternates}}`, out.V{"driver": pick.Name, "alternates": strings.Join(altNames, ", ")})
	} else {
		out.T(out.Sparkle, `Automatically selected the {{.driver}} driver`, out.V{"driver": pick.String()})
	}
	return pick
}

// validateDriver validates that the selected driver appears sane, exits if not
func validateDriver(ds registry.DriverState, existing *config.ClusterConfig) {
	name := ds.Name
	glog.Infof("validating driver %q against %+v", name, existing)
	if !driver.Supported(name) {
		exit.WithCodeT(exit.Unavailable, "The driver {{.experimental}} '{{.driver}}' is not supported on {{.os}}", out.V{"driver": name, "os": runtime.GOOS})
	}

	st := ds.State
	glog.Infof("status for %s: %+v", name, st)

	if st.Error != nil {
		out.ErrLn("")

		out.WarningT("'{{.driver}}' driver reported an issue: {{.error}}", out.V{"driver": name, "error": st.Error})
		out.ErrT(out.Tip, "Suggestion: {{.fix}}", out.V{"fix": translate.T(st.Fix)})
		if st.Doc != "" {
			out.ErrT(out.Documentation, "Documentation: {{.url}}", out.V{"url": st.Doc})
		}
		out.ErrLn("")

		if !st.Installed && !viper.GetBool(force) {
			if existing != nil && name == existing.Driver {
				exit.WithCodeT(exit.Unavailable, "{{.driver}} does not appear to be installed, but is specified by an existing profile. Please run 'minikube delete' or install {{.driver}}", out.V{"driver": name})
			}
			exit.WithCodeT(exit.Unavailable, "{{.driver}} does not appear to be installed", out.V{"driver": name})
		}
	}

	if existing == nil {
		return
	}

	api, err := machine.NewAPIClient()
	if err != nil {
		glog.Warningf("selectDriver NewAPIClient: %v", err)
		return
	}

	cp, err := config.PrimaryControlPlane(existing)
	if err != nil {
		exit.WithError("Error getting primary cp", err)
	}

	machineName := driver.MachineName(*existing, cp)
	h, err := api.Load(machineName)
	if err != nil {
		glog.Warningf("selectDriver api.Load: %v", err)
		return
	}

	if h.Driver.DriverName() == name {
		return
	}

	out.ErrT(out.Conflict, `The existing "{{.profile_name}}" VM that was created using the "{{.old_driver}}" driver, and is incompatible with the "{{.driver}}" driver.`,
		out.V{"profile_name": machineName, "driver": name, "old_driver": h.Driver.DriverName()})

	out.ErrT(out.Workaround, `To proceed, either:

    1) Delete the existing "{{.profile_name}}" cluster using: '{{.command}} delete'

    * or *

    2) Start the existing "{{.profile_name}}" cluster using: '{{.command}} start --driver={{.old_driver}}'
	`, out.V{"command": minikubeCmd(), "old_driver": h.Driver.DriverName(), "profile_name": machineName})

	exit.WithCodeT(exit.Config, "Exiting.")
}

func selectImageRepository(mirrorCountry string) (bool, string, error) {
	var tryCountries []string
	var fallback string
	glog.Infof("selecting image repository for country %s ...", mirrorCountry)

	if mirrorCountry != "" {
		localRepos, ok := constants.ImageRepositories[mirrorCountry]
		if !ok || len(localRepos) == 0 {
			return false, "", fmt.Errorf("invalid image mirror country code: %s", mirrorCountry)
		}

		tryCountries = append(tryCountries, mirrorCountry)

		// we'll use the first repository as fallback
		// when none of the mirrors in the given location is available
		fallback = localRepos[0]

	} else {
		// always make sure global is preferred
		tryCountries = append(tryCountries, "global")
		for k := range constants.ImageRepositories {
			if strings.ToLower(k) != "global" {
				tryCountries = append(tryCountries, k)
			}
		}
	}

	checkRepository := func(repo string) error {
		pauseImage := images.Pause(repo)
		ref, err := name.ParseReference(pauseImage, name.WeakValidation)
		if err != nil {
			return err
		}

		_, err = remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
		return err
	}

	for _, code := range tryCountries {
		localRepos := constants.ImageRepositories[code]
		for _, repo := range localRepos {
			err := checkRepository(repo)
			if err == nil {
				return true, repo, nil
			}
		}
	}

	return false, fallback, nil
}

// Return a minikube command containing the current profile name
func minikubeCmd() string {
	if viper.GetString(config.ProfileName) != constants.DefaultClusterName {
		return fmt.Sprintf("minikube -p %s", config.ProfileName)
	}
	return "minikube"
}

// validateUser validates minikube is run by the recommended user (privileged or regular)
func validateUser(drvName string) {
	u, err := user.Current()
	if err != nil {
		glog.Errorf("Error getting the current user: %v", err)
		return
	}

	useForce := viper.GetBool(force)

	if driver.NeedsRoot(drvName) && u.Uid != "0" && !useForce {
		exit.WithCodeT(exit.Permissions, `The "{{.driver_name}}" driver requires root privileges. Please run minikube using 'sudo minikube --driver={{.driver_name}}'.`, out.V{"driver_name": drvName})
	}

	if driver.NeedsRoot(drvName) || u.Uid != "0" {
		return
	}

	out.T(out.Stopped, `The "{{.driver_name}}" driver should not be used with root privileges.`, out.V{"driver_name": drvName})
	out.T(out.Tip, "If you are running minikube within a VM, consider using --driver=none:")
	out.T(out.Documentation, "  https://minikube.sigs.k8s.io/docs/reference/drivers/none/")

	if !useForce {
		os.Exit(exit.Permissions)
	}
	_, err = config.Load(viper.GetString(config.ProfileName))
	if err == nil || !config.IsNotExist(err) {
		out.T(out.Tip, "Tip: To remove this root owned cluster, run: sudo {{.cmd}} delete", out.V{"cmd": minikubeCmd()})
	}
	if !useForce {
		exit.WithCodeT(exit.Permissions, "Exiting")
	}
}

// memoryLimits returns the amount of memory allocated to the system and hypervisor
func memoryLimits(drvName string) (int, int, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return -1, -1, err
	}
	sysLimit := int(v.Total / 1024 / 1024)
	containerLimit := 0

	if driver.IsKIC(drvName) {
		s, err := oci.DaemonInfo(drvName)
		if err != nil {
			return -1, -1, err
		}
		containerLimit = int(s.TotalMemory / 1024 / 1024)
	}
	return sysLimit, containerLimit, nil
}

// suggestMemoryAllocation calculates the default memory footprint in MB
func suggestMemoryAllocation(sysLimit int, containerLimit int) int {
	fallback := 2200
	maximum := 6000

	if sysLimit > 0 && fallback > sysLimit {
		return sysLimit
	}

	// If there are container limits, add tiny bit of slack for non-minikube components
	if containerLimit > 0 {
		if fallback > containerLimit {
			return containerLimit
		}
		maximum = containerLimit - 48
	}

	// Suggest 25% of RAM, rounded to nearest 100MB. Hyper-V requires an even number!
	suggested := int(sysLimit/400.0) * 100

	if suggested > maximum {
		return maximum
	}

	if suggested < fallback {
		return fallback
	}

	return suggested
}

// defaultMemorySize calculates the default memory footprint in MB
func suggest(drvName string, available int) int {
	fallback := 2200
	maximum := 6000

	// For KIC, do not allocate more memory than the container has available (+ some slack)
	if driver.IsKIC(drvName) {
		maximum = available - 100
	}

	// Suggest 25% of RAM, rounded to nearest 100MB
	suggested := int(available/400.0) * 100

	if suggested > maximum {
		suggested = maximum
	}

	if suggested < fallback {
		suggested = fallback
	}

	glog.Infof("Selecting memory default of %dMB, given %dMB available and %dMB maximum", suggested, available, maximum)
	return suggested
}

// validateMemorySize validates the memory size matches the minimum recommended
func validateMemorySize() {
	req := pkgutil.CalculateSizeInMB(viper.GetString(memory))
	if req < minUsableMem && !viper.GetBool(force) {
		exit.WithCodeT(exit.Config, "Requested memory allocation {{.requested}}MB is less than the usable minimum of {{.minimum}}MB",
			out.V{"requested": req, "mininum": minUsableMem})
	}
	if req < minRecommendedMem && !viper.GetBool(force) {
		out.T(out.Notice, "Requested memory allocation ({{.requested}}MB) is less than the recommended minimum {{.recommended}}MB. Kubernetes may crash unexpectedly.",
			out.V{"requested": req, "recommended": minRecommendedMem})
	}
}

// validateCPUCount validates the cpu count matches the minimum recommended
func validateCPUCount(local bool) {
	var cpuCount int
	if local {
		// Uses the gopsutil cpu package to count the number of physical cpu cores
		ci, err := cpu.Counts(false)
		if err != nil {
			glog.Warningf("Unable to get CPU info: %v", err)
		} else {
			cpuCount = ci
		}
	} else {
		cpuCount = viper.GetInt(cpus)
	}
	if cpuCount < minimumCPUS && !viper.GetBool(force) {
		exit.UsageT("Requested cpu count {{.requested_cpus}} is less than the minimum allowed of {{.minimum_cpus}}", out.V{"requested_cpus": cpuCount, "minimum_cpus": minimumCPUS})
	}
}

// validateFlags validates the supplied flags against known bad combinations
func validateFlags(cmd *cobra.Command, drvName string) {
	if cmd.Flags().Changed(humanReadableDiskSize) {
		diskSizeMB := pkgutil.CalculateSizeInMB(viper.GetString(humanReadableDiskSize))
		if diskSizeMB < pkgutil.CalculateSizeInMB(minimumDiskSize) && !viper.GetBool(force) {
			exit.WithCodeT(exit.Config, "Requested disk size {{.requested_size}} is less than minimum of {{.minimum_size}}", out.V{"requested_size": diskSizeMB, "minimum_size": pkgutil.CalculateSizeInMB(minimumDiskSize)})
		}
	}

	if cmd.Flags().Changed(cpus) {
		validateCPUCount(driver.BareMetal(drvName))
		if !driver.HasResourceLimits(drvName) {
			out.WarningT("The '{{.name}}' driver does not respect the --cpus flag", out.V{"name": drvName})
		}
	}

	if cmd.Flags().Changed(memory) {
		validateMemorySize()
		if !driver.HasResourceLimits(drvName) {
			out.WarningT("The '{{.name}}' driver does not respect the --memory flag", out.V{"name": drvName})
		}
	}

	if driver.BareMetal(drvName) {
		if viper.GetString(config.ProfileName) != constants.DefaultClusterName {
			exit.WithCodeT(exit.Config, "The '{{.name}} driver does not support multiple profiles: https://minikube.sigs.k8s.io/docs/reference/drivers/none/", out.V{"name": drvName})
		}

		runtime := viper.GetString(containerRuntime)
		if runtime != "docker" {
			out.WarningT("Using the '{{.runtime}}' runtime with the 'none' driver is an untested configuration!", out.V{"runtime": runtime})
		}
	}

	// check that kubeadm extra args contain only whitelisted parameters
	for param := range node.ExtraOptions.AsMap().Get(bsutil.Kubeadm) {
		if !config.ContainsParam(bsutil.KubeadmExtraArgsWhitelist[bsutil.KubeadmCmdParam], param) &&
			!config.ContainsParam(bsutil.KubeadmExtraArgsWhitelist[bsutil.KubeadmConfigParam], param) {
			exit.UsageT("Sorry, the kubeadm.{{.parameter_name}} parameter is currently not supported by --extra-config", out.V{"parameter_name": param})
		}
	}

	validateRegistryMirror()
}

// This function validates if the --registry-mirror
// args match the format of http://localhost
func validateRegistryMirror() {

	if len(registryMirror) > 0 {
		for _, loc := range registryMirror {
			URL, err := url.Parse(loc)
			if err != nil {
				glog.Errorln("Error Parsing URL: ", err)
			}
			if (URL.Scheme != "http" && URL.Scheme != "https") || URL.Path != "" {
				exit.UsageT("Sorry, the url provided with the --registry-mirror flag is invalid: {{.url}}", out.V{"url": loc})
			}

		}
	}
}

// generateCfgFromFlags generates config.Config based on flags and supplied arguments
func generateCfgFromFlags(cmd *cobra.Command, k8sVersion string, drvName string) (config.ClusterConfig, config.Node, error) {
	r, err := cruntime.New(cruntime.Config{Type: viper.GetString(containerRuntime)})
	if err != nil {
		return config.ClusterConfig{}, config.Node{}, err
	}

	// Pick good default values for --network-plugin and --enable-default-cni based on runtime.
	selectedEnableDefaultCNI := viper.GetBool(enableDefaultCNI)
	selectedNetworkPlugin := viper.GetString(networkPlugin)
	if r.DefaultCNI() && !cmd.Flags().Changed(networkPlugin) {
		selectedNetworkPlugin = "cni"
		if !cmd.Flags().Changed(enableDefaultCNI) {
			selectedEnableDefaultCNI = true
		}
	}

	// Feed Docker our host proxy environment by default, so that it can pull images
	if _, ok := r.(*cruntime.Docker); ok && !cmd.Flags().Changed("docker-env") {
		setDockerProxy()
	}

	repository := viper.GetString(imageRepository)
	mirrorCountry := strings.ToLower(viper.GetString(imageMirrorCountry))
	if strings.ToLower(repository) == "auto" || mirrorCountry != "" {
		found, autoSelectedRepository, err := selectImageRepository(mirrorCountry)
		if err != nil {
			exit.WithError("Failed to check main repository and mirrors for images for images", err)
		}

		if !found {
			if autoSelectedRepository == "" {
				exit.WithCodeT(exit.Failure, "None of the known repositories is accessible. Consider specifying an alternative image repository with --image-repository flag")
			} else {
				out.WarningT("None of the known repositories in your location are accessible. Using {{.image_repository_name}} as fallback.", out.V{"image_repository_name": autoSelectedRepository})
			}
		}

		repository = autoSelectedRepository
	}

	if cmd.Flags().Changed(imageRepository) {
		out.T(out.SuccessType, "Using image repository {{.name}}", out.V{"name": repository})
	}

	var kubeNodeName string
	if drvName != driver.None {
		kubeNodeName = "m01"
	}

	sysLimit, containerLimit, err := memoryLimits(drvName)
	if err != nil {
		glog.Warningf("Unable to query memory limits: %v", err)
	}

	mem := suggestMemoryAllocation(sysLimit, containerLimit)
	if viper.GetString(memory) != "" {
		mem = pkgutil.CalculateSizeInMB(viper.GetString(memory))
	} else {
		glog.Infof("Using suggested %dMB memory alloc based on sys=%dMB, container=%dMB", mem, sysLimit, containerLimit)
	}

	// Create the initial node, which will necessarily be a control plane
	cp := config.Node{
		Port:              viper.GetInt(apiServerPort),
		KubernetesVersion: k8sVersion,
		Name:              kubeNodeName,
		ControlPlane:      true,
		Worker:            true,
	}

	cfg := config.ClusterConfig{
		Name:                    viper.GetString(config.ProfileName),
		KeepContext:             viper.GetBool(keepContext),
		EmbedCerts:              viper.GetBool(embedCerts),
		MinikubeISO:             viper.GetString(isoURL),
		Memory:                  mem,
		CPUs:                    viper.GetInt(cpus),
		DiskSize:                pkgutil.CalculateSizeInMB(viper.GetString(humanReadableDiskSize)),
		Driver:                  drvName,
		HyperkitVpnKitSock:      viper.GetString(vpnkitSock),
		HyperkitVSockPorts:      viper.GetStringSlice(vsockPorts),
		NFSShare:                viper.GetStringSlice(nfsShare),
		NFSSharesRoot:           viper.GetString(nfsSharesRoot),
		DockerEnv:               node.DockerEnv,
		DockerOpt:               node.DockerOpt,
		InsecureRegistry:        insecureRegistry,
		RegistryMirror:          registryMirror,
		HostOnlyCIDR:            viper.GetString(hostOnlyCIDR),
		HypervVirtualSwitch:     viper.GetString(hypervVirtualSwitch),
		HypervUseExternalSwitch: viper.GetBool(hypervUseExternalSwitch),
		HypervExternalAdapter:   viper.GetString(hypervExternalAdapter),
		KVMNetwork:              viper.GetString(kvmNetwork),
		KVMQemuURI:              viper.GetString(kvmQemuURI),
		KVMGPU:                  viper.GetBool(kvmGPU),
		KVMHidden:               viper.GetBool(kvmHidden),
		DisableDriverMounts:     viper.GetBool(disableDriverMounts),
		UUID:                    viper.GetString(uuid),
		NoVTXCheck:              viper.GetBool(noVTXCheck),
		DNSProxy:                viper.GetBool(dnsProxy),
		HostDNSResolver:         viper.GetBool(hostDNSResolver),
		HostOnlyNicType:         viper.GetString(hostOnlyNicType),
		NatNicType:              viper.GetString(natNicType),
		KubernetesConfig: config.KubernetesConfig{
			KubernetesVersion:      k8sVersion,
			ClusterName:            viper.GetString(config.ProfileName),
			APIServerName:          viper.GetString(apiServerName),
			APIServerNames:         apiServerNames,
			APIServerIPs:           apiServerIPs,
			DNSDomain:              viper.GetString(dnsDomain),
			FeatureGates:           viper.GetString(featureGates),
			ContainerRuntime:       viper.GetString(containerRuntime),
			CRISocket:              viper.GetString(criSocket),
			NetworkPlugin:          selectedNetworkPlugin,
			ServiceCIDR:            viper.GetString(serviceCIDR),
			ImageRepository:        repository,
			ExtraOptions:           node.ExtraOptions,
			ShouldLoadCachedImages: viper.GetBool(cacheImages),
			EnableDefaultCNI:       selectedEnableDefaultCNI,
		},
		Nodes: []config.Node{cp},
	}
	return cfg, cp, nil
}

// setDockerProxy sets the proxy environment variables in the docker environment.
func setDockerProxy() {
	for _, k := range proxy.EnvVars {
		if v := os.Getenv(k); v != "" {
			// convert https_proxy to HTTPS_PROXY for linux
			// TODO (@medyagh): if user has both http_proxy & HTTPS_PROXY set merge them.
			k = strings.ToUpper(k)
			if k == "HTTP_PROXY" || k == "HTTPS_PROXY" {
				if strings.HasPrefix(v, "localhost") || strings.HasPrefix(v, "127.0") {
					out.WarningT("Not passing {{.name}}={{.value}} to docker env.", out.V{"name": k, "value": v})
					continue
				}
			}
			node.DockerEnv = append(node.DockerEnv, fmt.Sprintf("%s=%s", k, v))
		}
	}
}

// autoSetDriverOptions sets the options needed for specific driver automatically.
func autoSetDriverOptions(cmd *cobra.Command, drvName string) (err error) {
	err = nil
	hints := driver.FlagDefaults(drvName)
	if !cmd.Flags().Changed("extra-config") && len(hints.ExtraOptions) > 0 {
		for _, eo := range hints.ExtraOptions {
			glog.Infof("auto setting extra-config to %q.", eo)
			err = node.ExtraOptions.Set(eo)
			if err != nil {
				err = errors.Wrapf(err, "setting extra option %s", eo)
			}
		}
	}

	if !cmd.Flags().Changed(cacheImages) {
		viper.Set(cacheImages, hints.CacheImages)
	}

	if !cmd.Flags().Changed(containerRuntime) && hints.ContainerRuntime != "" {
		viper.Set(containerRuntime, hints.ContainerRuntime)
		glog.Infof("auto set %s to %q.", containerRuntime, hints.ContainerRuntime)
	}

	if !cmd.Flags().Changed(cmdcfg.Bootstrapper) && hints.Bootstrapper != "" {
		viper.Set(cmdcfg.Bootstrapper, hints.Bootstrapper)
		glog.Infof("auto set %s to %q.", cmdcfg.Bootstrapper, hints.Bootstrapper)

	}

	return err
}

// getKubernetesVersion ensures that the requested version is reasonable
func getKubernetesVersion(old *config.ClusterConfig) string {
	paramVersion := viper.GetString(kubernetesVersion)

	if paramVersion == "" { // if the user did not specify any version then ...
		if old != nil { // .. use the old version from config (if any)
			paramVersion = old.KubernetesConfig.KubernetesVersion
		}
		if paramVersion == "" { // .. otherwise use the default version
			paramVersion = constants.DefaultKubernetesVersion
		}
	}

	nvs, err := semver.Make(strings.TrimPrefix(paramVersion, version.VersionPrefix))
	if err != nil {
		exit.WithCodeT(exit.Data, `Unable to parse "{{.kubernetes_version}}": {{.error}}`, out.V{"kubernetes_version": paramVersion, "error": err})
	}
	nv := version.VersionPrefix + nvs.String()

	if old == nil || old.KubernetesConfig.KubernetesVersion == "" {
		return nv
	}

	oldestVersion, err := semver.Make(strings.TrimPrefix(constants.OldestKubernetesVersion, version.VersionPrefix))
	if err != nil {
		exit.WithCodeT(exit.Data, "Unable to parse oldest Kubernetes version from constants: {{.error}}", out.V{"error": err})
	}
	defaultVersion, err := semver.Make(strings.TrimPrefix(constants.DefaultKubernetesVersion, version.VersionPrefix))
	if err != nil {
		exit.WithCodeT(exit.Data, "Unable to parse default Kubernetes version from constants: {{.error}}", out.V{"error": err})
	}

	if nvs.LT(oldestVersion) {
		out.WarningT("Specified Kubernetes version {{.specified}} is less than the oldest supported version: {{.oldest}}", out.V{"specified": nvs, "oldest": constants.OldestKubernetesVersion})
		if viper.GetBool(force) {
			out.WarningT("Kubernetes {{.version}} is not supported by this release of minikube", out.V{"version": nvs})
		} else {
			exit.WithCodeT(exit.Data, "Sorry, Kubernetes {{.version}} is not supported by this release of minikube", out.V{"version": nvs})
		}
	}

	ovs, err := semver.Make(strings.TrimPrefix(old.KubernetesConfig.KubernetesVersion, version.VersionPrefix))
	if err != nil {
		glog.Errorf("Error parsing old version %q: %v", old.KubernetesConfig.KubernetesVersion, err)
	}

	if nvs.LT(ovs) {
		nv = version.VersionPrefix + ovs.String()
		profileArg := ""
		if old.Name != constants.DefaultClusterName {
			profileArg = fmt.Sprintf(" -p %s", old.Name)
		}

		suggestedName := old.Name + "2"
		out.T(out.Conflict, "You have selected Kubernetes v{{.new}}, but the existing cluster is running Kubernetes v{{.old}}", out.V{"new": nvs, "old": ovs, "profile": profileArg})
		exit.WithCodeT(exit.Config, `Non-destructive downgrades are not supported, but you can proceed with one of the following options:

  1) Recreate the cluster with Kubernetes v{{.new}}, by running:

    minikube delete{{.profile}}
    minikube start{{.profile}} --kubernetes-version={{.new}}

  2) Create a second cluster with Kubernetes v{{.new}}, by running:

    minikube start -p {{.suggestedName}} --kubernetes-version={{.new}}

  3) Use the existing cluster at version Kubernetes v{{.old}}, by running:

    minikube start{{.profile}} --kubernetes-version={{.old}}
`, out.V{"new": nvs, "old": ovs, "profile": profileArg, "suggestedName": suggestedName})

	}
	if defaultVersion.GT(nvs) {
		out.T(out.ThumbsUp, "Kubernetes {{.new}} is now available. If you would like to upgrade, specify: --kubernetes-version={{.new}}", out.V{"new": defaultVersion})
	}
	return nv
}
