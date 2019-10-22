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

package driver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blang/semver"
	"github.com/golang/glog"
	"github.com/hashicorp/go-getter"
	"github.com/pkg/errors"

	"k8s.io/minikube/pkg/minikube/out"
	"k8s.io/minikube/pkg/util"
)

// InstallOrUpdate downloads driver if it is not present, or updates it if there's a newer version
func InstallOrUpdate(name string, directory string, v semver.Version, interactive bool, autoUpdate bool) error {
	if name != KVM2 && name != HyperKit {
		return nil
	}

	executable := fmt.Sprintf("docker-machine-driver-%s", name)
	exists := driverExists(executable)
	path, err := validateDriver(executable, v)
	if !exists || (err != nil && autoUpdate) {
		glog.Warningf("%s: %v", executable, err)
		path = filepath.Join(directory, executable)
		derr := download(executable, path, v)
		if derr != nil {
			return derr
		}
	}
	return fixDriverPermissions(name, path, interactive)
}

// fixDriverPermissions fixes the permissions on a driver
func fixDriverPermissions(name string, path string, interactive bool) error {
	if name != HyperKit {
		return nil
	}

	// Using the find command for hyperkit is far easier than cross-platform uid checks in Go.
	stdout, err := exec.Command("find", path, "-uid", "0", "-perm", "4755").Output()
	glog.Infof("stdout: %s", stdout)
	if err == nil && strings.TrimSpace(string(stdout)) == path {
		glog.Infof("%s looks good", path)
		return nil
	}

	cmds := []*exec.Cmd{
		exec.Command("sudo", "chown", "root:wheel", path),
		exec.Command("sudo", "chmod", "u+s", path),
	}

	var example strings.Builder
	for _, c := range cmds {
		example.WriteString(fmt.Sprintf("    $ %s \n", strings.Join(c.Args, " ")))
	}

	out.T(out.Permissions, "The '{{.driver}}' driver requires elevated permissions. The following commands will be executed:\n\n{{ .example }}\n", out.V{"driver": name, "example": example.String()})
	for _, c := range cmds {
		testArgs := append([]string{"-n"}, c.Args[1:]...)
		test := exec.Command("sudo", testArgs...)
		glog.Infof("testing: %v", test.Args)
		if err := test.Run(); err != nil {
			glog.Infof("%v may require a password: %v", c.Args, err)
			if !interactive {
				return fmt.Errorf("%v requires a password, and --interactive=false", c.Args)
			}
		}
		glog.Infof("running: %v", c.Args)
		err := c.Run()
		if err != nil {
			return errors.Wrapf(err, "%v", c.Args)
		}
	}
	return nil
}

// validateDriver validates if a driver appears to be up-to-date and installed properly
func validateDriver(executable string, v semver.Version) (string, error) {
	glog.Infof("Validating %s, PATH=%s", executable, os.Getenv("PATH"))
	path, err := exec.LookPath(executable)
	if err != nil {
		return path, err
	}

	output, err := exec.Command(path, "version").Output()
	if err != nil {
		return path, err
	}

	ev := extractVMDriverVersion(string(output))
	if len(ev) == 0 {
		return path, fmt.Errorf("%s: unable to extract version from %q", executable, output)
	}

	vmDriverVersion, err := semver.Make(ev)
	if err != nil {
		return path, errors.Wrap(err, "can't parse driver version")
	}
	if vmDriverVersion.LT(v) {
		return path, fmt.Errorf("%s is version %s, want %s", executable, vmDriverVersion, v)
	}
	return path, nil
}

func driverWithChecksumURL(name string, v semver.Version) string {
	base := fmt.Sprintf("https://github.com/kubernetes/minikube/releases/download/v%s/%s", v, name)
	return fmt.Sprintf("%s?checksum=file:%s.sha256", base, base)
}

// download an arbitrary driver
func download(name string, destination string, v semver.Version) error {
	out.T(out.FileDownload, "Downloading driver {{.driver}}:", out.V{"driver": name})
	os.Remove(destination)
	url := driverWithChecksumURL(name, v)
	client := &getter.Client{
		Src:     url,
		Dst:     destination,
		Mode:    getter.ClientModeFile,
		Options: []getter.ClientOption{getter.WithProgress(util.DefaultProgressBar)},
	}

	glog.Infof("Downloading: %+v", client)
	if err := client.Get(); err != nil {
		return errors.Wrapf(err, "download failed: %s", url)
	}
	// Give downloaded drivers a baseline decent file permission
	return os.Chmod(destination, 0755)
}

// extractVMDriverVersion extracts the driver version.
// KVM and Hyperkit drivers support the 'version' command, that display the information as:
// version: vX.X.X
// commit: XXXX
// This method returns the version 'vX.X.X' or empty if the version isn't found.
func extractVMDriverVersion(s string) string {
	versionRegex := regexp.MustCompile(`version:(.*)`)
	matches := versionRegex.FindStringSubmatch(s)

	if len(matches) != 2 {
		return ""
	}

	v := strings.TrimSpace(matches[1])
	return strings.TrimPrefix(v, "v")
}

func driverExists(driver string) bool {
	_, err := exec.LookPath(driver)
	return err == nil
}
