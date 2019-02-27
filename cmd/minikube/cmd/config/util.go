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
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/minikube/assets"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/storageclass"
)

// Runs all the validation or callback functions and collects errors
func run(name string, value string, fns []setFn) error {
	var errors []error
	for _, fn := range fns {
		err := fn(name, value)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("%v", errors)
	}
	return nil
}

func findSetting(name string) (Setting, error) {
	for _, s := range settings {
		if name == s.name {
			return s, nil
		}
	}
	return Setting{}, fmt.Errorf("Property name %s not found", name)
}

// Set Functions

func SetString(m config.MinikubeConfig, name string, val string) error {
	m[name] = val
	return nil
}

func SetMap(m config.MinikubeConfig, name string, val map[string]interface{}) error {
	m[name] = val
	return nil
}

func SetConfigMap(m config.MinikubeConfig, name string, val string) error {
	list := strings.Split(val, ",")
	v := make(map[string]interface{})
	for _, s := range list {
		v[s] = nil
	}
	m[name] = v
	return nil
}

func SetInt(m config.MinikubeConfig, name string, val string) error {
	i, err := strconv.Atoi(val)
	if err != nil {
		return err
	}
	m[name] = i
	return nil
}

func SetBool(m config.MinikubeConfig, name string, val string) error {
	b, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}
	m[name] = b
	return nil
}

// EnableOrDisableAddon updates addon status executing any commands necessary
func EnableOrDisableAddon(name string, val string) error {
	enable, err := strconv.ParseBool(val)
	if err != nil {
		return errors.Wrapf(err, "parsing bool: %s", name)
	}

	//TODO(r2d4): config package should not reference API, pull this out
	api, err := machine.NewAPIClient()
	if err != nil {
		return errors.Wrap(err, "machine client")
	}
	defer api.Close()
	cluster.EnsureMinikubeRunningOrExit(api, 0)

	addon := assets.Addons[name]
	host, err := cluster.CheckIfHostExistsAndLoad(api, config.GetMachineName())
	if err != nil {
		return errors.Wrap(err, "getting host")
	}
	cmd, err := machine.CommandRunner(host)
	if err != nil {
		return errors.Wrap(err, "command runner")
	}
	if enable {
		for _, addon := range addon.Assets {
			if err := cmd.Copy(addon); err != nil {
				return errors.Wrapf(err, "enabling addon %s", addon.AssetName)
			}
		}
	} else {
		for _, addon := range addon.Assets {
			if err := cmd.Remove(addon); err != nil {
				return errors.Wrapf(err, "disabling addon %s", addon.AssetName)
			}
		}
	}
	return nil
}

func EnableOrDisableStorageClasses(name, val string) error {
	enable, err := strconv.ParseBool(val)
	if err != nil {
		return errors.Wrap(err, "Error parsing boolean")
	}

	class := constants.DefaultStorageClassProvisioner
	if name == "storage-provisioner-gluster" {
		class = "glusterfile"
	}

	if enable {
		// Only StorageClass for 'name' should be marked as default
		err := storageclass.SetDefaultStorageClass(class)
		if err != nil {
			return errors.Wrapf(err, "Error making %s the default storage class", class)
		}
	} else {
		// Unset the StorageClass as default
		err := storageclass.DisableDefaultStorageClass(class)
		if err != nil {
			return errors.Wrapf(err, "Error disabling %s as the default storage class", class)
		}
	}

	return EnableOrDisableAddon(name, val)
}
