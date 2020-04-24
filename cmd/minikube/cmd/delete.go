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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/machine/libmachine/mcnerror"
	"github.com/golang/glog"
	"github.com/mitchellh/go-ps"
	"github.com/pkg/errors"

	"github.com/docker/machine/libmachine"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cmdcfg "k8s.io/minikube/cmd/minikube/cmd/config"
	"k8s.io/minikube/pkg/drivers/kic/oci"
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
)

var deleteAll bool
var purge bool

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes a local kubernetes cluster",
	Long: `Deletes a local kubernetes cluster. This command deletes the VM, and removes all
associated files.`,
	Run: runDelete,
}

type typeOfError int

const (
	// Fatal is a type of DeletionError
	Fatal typeOfError = 0
	// MissingProfile is a type of DeletionError
	MissingProfile typeOfError = 1
	// MissingCluster is a type of DeletionError
	MissingCluster typeOfError = 2
)

// DeletionError can be returned from DeleteProfiles
type DeletionError struct {
	Err     error
	Errtype typeOfError
}

func (error DeletionError) Error() string {
	return error.Err.Error()
}

func init() {
	deleteCmd.Flags().BoolVar(&deleteAll, "all", false, "Set flag to delete all profiles")
	deleteCmd.Flags().BoolVar(&purge, "purge", false, "Set this flag to delete the '.minikube' folder from your user directory.")

	if err := viper.BindPFlags(deleteCmd.Flags()); err != nil {
		exit.WithError("unable to bind flags", err)
	}
	RootCmd.AddCommand(deleteCmd)
}

func deleteContainersAndVolumes() {
	delLabel := fmt.Sprintf("%s=%s", oci.CreatedByLabelKey, "true")
	errs := oci.DeleteContainersByLabel(oci.Docker, delLabel)
	if len(errs) > 0 { // it will error if there is no container to delete
		glog.Infof("error delete containers by label %q (might be okay): %+v", delLabel, errs)
	}

	errs = oci.DeleteAllVolumesByLabel(oci.Docker, delLabel)
	if len(errs) > 0 { // it will not error if there is nothing to delete
		glog.Warningf("error delete volumes by label %q (might be okay): %+v", delLabel, errs)
	}

	errs = oci.PruneAllVolumesByLabel(oci.Docker, delLabel)
	if len(errs) > 0 { // it will not error if there is nothing to delete
		glog.Warningf("error pruning volumes by label %q (might be okay): %+v", delLabel, errs)
	}
}

// runDelete handles the executes the flow of "minikube delete"
func runDelete(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		exit.UsageT("Usage: minikube delete")
	}

	validProfiles, invalidProfiles, err := config.ListProfiles()
	if err != nil {
		glog.Warningf("'error loading profiles in minikube home %q: %v", localpath.MiniPath(), err)
	}
	profilesToDelete := append(validProfiles, invalidProfiles...)
	// in the case user has more than 1 profile and runs --purge
	// to prevent abandoned VMs/containers, force user to run with delete --all
	if purge && len(profilesToDelete) > 1 && !deleteAll {
		out.ErrT(out.Notice, "Multiple minikube profiles were found - ")
		for _, p := range profilesToDelete {
			out.T(out.Notice, "    - {{.profile}}", out.V{"profile": p.Name})
		}
		exit.UsageT("Usage: minikube delete --all --purge")
	}

	if deleteAll {
		deleteContainersAndVolumes()

		errs := DeleteProfiles(profilesToDelete)
		if len(errs) > 0 {
			HandleDeletionErrors(errs)
		} else {
			out.T(out.DeletingHost, "Successfully deleted all profiles")
		}
	} else {
		if len(args) > 0 {
			exit.UsageT("usage: minikube delete")
		}

		cname := ClusterFlagValue()
		profile, err := config.LoadProfile(cname)
		if err != nil {
			out.ErrT(out.Meh, `"{{.name}}" profile does not exist, trying anyways.`, out.V{"name": cname})
		}

		deletePossibleKicLeftOver(cname)

		errs := DeleteProfiles([]*config.Profile{profile})
		if len(errs) > 0 {
			HandleDeletionErrors(errs)
		}
	}

	// If the purge flag is set, go ahead and delete the .minikube directory.
	if purge {
		purgeMinikubeDirectory()
	}
}

func purgeMinikubeDirectory() {
	glog.Infof("Purging the '.minikube' directory located at %s", localpath.MiniPath())
	if err := os.RemoveAll(localpath.MiniPath()); err != nil {
		exit.WithError("unable to delete minikube config folder", err)
	}
	out.T(out.Deleted, "Successfully purged minikube directory located at - [{{.minikubeDirectory}}]", out.V{"minikubeDirectory": localpath.MiniPath()})
}

// DeleteProfiles deletes one or more profiles
func DeleteProfiles(profiles []*config.Profile) []error {
	var errs []error
	for _, profile := range profiles {
		err := deleteProfile(profile)

		if err != nil {
			mm, loadErr := machine.LoadMachine(profile.Name)

			if !profile.IsValid() || (loadErr != nil || !mm.IsValid()) {
				invalidProfileDeletionErrs := deleteInvalidProfile(profile)
				if len(invalidProfileDeletionErrs) > 0 {
					errs = append(errs, invalidProfileDeletionErrs...)
				}
			} else {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func deletePossibleKicLeftOver(name string) {
	delLabel := fmt.Sprintf("%s=%s", oci.ProfileLabelKey, name)
	for _, bin := range []string{oci.Docker, oci.Podman} {
		cs, err := oci.ListContainersByLabel(bin, delLabel)
		if err == nil && len(cs) > 0 {
			for _, c := range cs {
				out.T(out.DeletingHost, `Deleting container "{{.name}}" ...`, out.V{"name": name})
				err := oci.DeleteContainer(bin, c)
				if err != nil { // it will error if there is no container to delete
					glog.Errorf("error deleting container %q. you might want to delete that manually :\n%v", name, err)
				}

			}
		}

		errs := oci.DeleteAllVolumesByLabel(bin, delLabel)
		if errs != nil { // it will not error if there is nothing to delete
			glog.Warningf("error deleting volumes (might be okay).\nTo see the list of volumes run: 'docker volume ls'\n:%v", errs)
		}

		errs = oci.PruneAllVolumesByLabel(bin, delLabel)
		if len(errs) > 0 { // it will not error if there is nothing to delete
			glog.Warningf("error pruning volume (might be okay):\n%v", errs)
		}
	}
}

func deleteProfile(profile *config.Profile) error {
	viper.Set(config.ProfileName, profile.Name)
	if profile.Config != nil {
		// if driver is oci driver, delete containers and volumes
		if driver.IsKIC(profile.Config.Driver) {
			out.T(out.DeletingHost, `Deleting "{{.profile_name}}" in {{.driver_name}} ...`, out.V{"profile_name": profile.Name, "driver_name": profile.Config.Driver})
			deletePossibleKicLeftOver(profile.Name)
		}
	}

	api, err := machine.NewAPIClient()
	if err != nil {
		delErr := profileDeletionErr(profile.Name, fmt.Sprintf("error getting client %v", err))
		return DeletionError{Err: delErr, Errtype: Fatal}
	}
	defer api.Close()

	cc, err := config.Load(profile.Name)
	if err != nil && !config.IsNotExist(err) {
		delErr := profileDeletionErr(profile.Name, fmt.Sprintf("error loading profile config: %v", err))
		return DeletionError{Err: delErr, Errtype: MissingProfile}
	}

	if err == nil && driver.BareMetal(cc.Driver) {
		if err := uninstallKubernetes(api, *cc, cc.Nodes[0], viper.GetString(cmdcfg.Bootstrapper)); err != nil {
			deletionError, ok := err.(DeletionError)
			if ok {
				delErr := profileDeletionErr(profile.Name, fmt.Sprintf("%v", err))
				deletionError.Err = delErr
				return deletionError
			}
			return err
		}
	}

	if err := killMountProcess(); err != nil {
		out.FailureT("Failed to kill mount process: {{.error}}", out.V{"error": err})
	}

	deleteHosts(api, cc)

	// In case DeleteHost didn't complete the job.
	deleteProfileDirectory(profile.Name)

	if err := deleteConfig(profile.Name); err != nil {
		return err
	}

	if err := deleteContext(profile.Name); err != nil {
		return err
	}
	out.T(out.Deleted, `Removed all traces of the "{{.name}}" cluster.`, out.V{"name": profile.Name})
	return nil
}

func deleteHosts(api libmachine.API, cc *config.ClusterConfig) {
	if cc != nil {
		for _, n := range cc.Nodes {
			machineName := driver.MachineName(*cc, n)
			if err := machine.DeleteHost(api, machineName); err != nil {
				switch errors.Cause(err).(type) {
				case mcnerror.ErrHostDoesNotExist:
					glog.Infof("Host %s does not exist. Proceeding ahead with cleanup.", machineName)
				default:
					out.FailureT("Failed to delete cluster: {{.error}}", out.V{"error": err})
					out.T(out.Notice, `You may need to manually remove the "{{.name}}" VM from your hypervisor`, out.V{"name": machineName})
				}
			}
		}
	}
}

func deleteConfig(cname string) error {
	if err := config.DeleteProfile(cname); err != nil {
		if config.IsNotExist(err) {
			delErr := profileDeletionErr(cname, fmt.Sprintf("\"%s\" profile does not exist", cname))
			return DeletionError{Err: delErr, Errtype: MissingProfile}
		}
		delErr := profileDeletionErr(cname, fmt.Sprintf("failed to remove profile %v", err))
		return DeletionError{Err: delErr, Errtype: Fatal}
	}
	return nil
}

func deleteContext(machineName string) error {
	if err := kubeconfig.DeleteContext(machineName); err != nil {
		return DeletionError{Err: fmt.Errorf("update config: %v", err), Errtype: Fatal}
	}

	if err := cmdcfg.Unset(config.ProfileName); err != nil {
		return DeletionError{Err: fmt.Errorf("unset minikube profile: %v", err), Errtype: Fatal}
	}
	return nil
}

func deleteInvalidProfile(profile *config.Profile) []error {
	out.T(out.DeletingHost, "Trying to delete invalid profile {{.profile}}", out.V{"profile": profile.Name})

	var errs []error
	pathToProfile := config.ProfileFolderPath(profile.Name, localpath.MiniPath())
	if _, err := os.Stat(pathToProfile); !os.IsNotExist(err) {
		err := os.RemoveAll(pathToProfile)
		if err != nil {
			errs = append(errs, DeletionError{err, Fatal})
		}
	}

	pathToMachine := localpath.MachinePath(profile.Name, localpath.MiniPath())
	if _, err := os.Stat(pathToMachine); !os.IsNotExist(err) {
		err := os.RemoveAll(pathToMachine)
		if err != nil {
			errs = append(errs, DeletionError{err, Fatal})
		}
	}
	return errs
}

func profileDeletionErr(cname string, additionalInfo string) error {
	return fmt.Errorf("error deleting profile \"%s\": %s", cname, additionalInfo)
}

func uninstallKubernetes(api libmachine.API, cc config.ClusterConfig, n config.Node, bsName string) error {
	out.T(out.Resetting, "Uninstalling Kubernetes {{.kubernetes_version}} using {{.bootstrapper_name}} ...", out.V{"kubernetes_version": cc.KubernetesConfig.KubernetesVersion, "bootstrapper_name": bsName})
	host, err := machine.LoadHost(api, driver.MachineName(cc, n))
	if err != nil {
		return DeletionError{Err: fmt.Errorf("unable to load host: %v", err), Errtype: MissingCluster}
	}

	r, err := machine.CommandRunner(host)
	if err != nil {
		return DeletionError{Err: fmt.Errorf("unable to get command runner %v", err), Errtype: MissingCluster}
	}

	clusterBootstrapper, err := cluster.Bootstrapper(api, bsName, cc, r)
	if err != nil {
		return DeletionError{Err: fmt.Errorf("unable to get bootstrapper: %v", err), Errtype: Fatal}
	}

	cr, err := cruntime.New(cruntime.Config{Type: cc.KubernetesConfig.ContainerRuntime, Runner: r})
	if err != nil {
		return DeletionError{Err: fmt.Errorf("unable to get runtime: %v", err), Errtype: Fatal}
	}

	// Unpause the cluster if necessary to avoid hung kubeadm
	_, err = cluster.Unpause(cr, r, nil)
	if err != nil {
		glog.Errorf("unpause failed: %v", err)
	}

	if err = clusterBootstrapper.DeleteCluster(cc.KubernetesConfig); err != nil {
		return DeletionError{Err: fmt.Errorf("failed to delete cluster: %v", err), Errtype: Fatal}
	}
	return nil
}

// HandleDeletionErrors handles deletion errors from DeleteProfiles
func HandleDeletionErrors(errors []error) {
	if len(errors) == 1 {
		handleSingleDeletionError(errors[0])
	} else {
		handleMultipleDeletionErrors(errors)
	}
}

func handleSingleDeletionError(err error) {
	deletionError, ok := err.(DeletionError)

	if ok {
		switch deletionError.Errtype {
		case Fatal:
			out.FatalT(deletionError.Error())
		case MissingProfile:
			out.ErrT(out.Sad, deletionError.Error())
		case MissingCluster:
			out.ErrT(out.Meh, deletionError.Error())
		default:
			out.FatalT(deletionError.Error())
		}
	} else {
		exit.WithError("Could not process error from failed deletion", err)
	}
}

func handleMultipleDeletionErrors(errors []error) {
	out.ErrT(out.Sad, "Multiple errors deleting profiles")

	for _, err := range errors {
		deletionError, ok := err.(DeletionError)

		if ok {
			glog.Errorln(deletionError.Error())
		} else {
			exit.WithError("Could not process errors from failed deletion", err)
		}
	}
}

func deleteProfileDirectory(profile string) {
	machineDir := filepath.Join(localpath.MiniPath(), "machines", profile)
	if _, err := os.Stat(machineDir); err == nil {
		out.T(out.DeletingHost, `Removing {{.directory}} ...`, out.V{"directory": machineDir})
		err := os.RemoveAll(machineDir)
		if err != nil {
			exit.WithError("Unable to remove machine directory", err)
		}
	}
}

// killMountProcess kills the mount process, if it is running
func killMountProcess() error {
	pidPath := filepath.Join(localpath.MiniPath(), constants.MountProcessFileName)
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		return nil
	}

	glog.Infof("Found %s ...", pidPath)
	out, err := ioutil.ReadFile(pidPath)
	if err != nil {
		return errors.Wrap(err, "ReadFile")
	}
	glog.Infof("pidfile contents: %s", out)
	pid, err := strconv.Atoi(string(out))
	if err != nil {
		return errors.Wrap(err, "error parsing pid")
	}
	// os.FindProcess does not check if pid is running :(
	entry, err := ps.FindProcess(pid)
	if err != nil {
		return errors.Wrap(err, "ps.FindProcess")
	}
	if entry == nil {
		glog.Infof("Stale pid: %d", pid)
		if err := os.Remove(pidPath); err != nil {
			return errors.Wrap(err, "Removing stale pid")
		}
		return nil
	}

	// We found a process, but it still may not be ours.
	glog.Infof("Found process %d: %s", pid, entry.Executable())
	proc, err := os.FindProcess(pid)
	if err != nil {
		return errors.Wrap(err, "os.FindProcess")
	}

	glog.Infof("Killing pid %d ...", pid)
	if err := proc.Kill(); err != nil {
		glog.Infof("Kill failed with %v - removing probably stale pid...", err)
		if err := os.Remove(pidPath); err != nil {
			return errors.Wrap(err, "Removing likely stale unkillable pid")
		}
		return errors.Wrap(err, fmt.Sprintf("Kill(%d/%s)", pid, entry.Executable()))
	}
	return nil
}
