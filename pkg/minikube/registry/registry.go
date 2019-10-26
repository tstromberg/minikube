/*
Copyright 2018 The Kubernetes Authors All rights reserved.

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

package registry

import (
	"fmt"
	"sync"

	"github.com/docker/machine/libmachine/drivers"

	"k8s.io/minikube/pkg/minikube/config"
)

// Priority is how we determine what driver to default to
type Priority int

const (
	Unknown Priority = iota
	Discouraged
	Deprecated
	Fallback
	Default
	Preferred
	StronglyPreferred
)

// Registry contains all the supported driver definitions on the host
type Registry interface {
	// Register a driver in registry
	Register(driver DriverDef) error

	// Driver returns the registered driver from a given name
	Driver(name string) (DriverDef, error)

	// List
	List() []DriverDef
}

// Configurator emits a struct to be marshalled into JSON for Machine Driver
type Configurator func(config.MachineConfig) interface{}

// Loader is a function that loads a byte stream and creates a driver.
type Loader func() drivers.Driver

// StatusChecker checks if a driver is available, offering a
type StatusChecker func() State

// State is the current state of the driver and its dependencies
type State struct {
	Installed bool
	Healthy   bool
	Error     error
	Fix       string
	Doc       string
}

// DriverDef defines how to initialize and load a machine driver
type DriverDef struct {
	// Name of the machine driver. It has to be unique.
	Name string

	// Config is a function that emits a configured driver struct
	Config Configurator

	// Init is a function that initializes a machine driver, if built-in to the minikube binary
	Init Loader

	// Status returns the installation status of the driver
	Status StatusChecker

	// Priority returns the prioritization for selecting a driver by default.
	Priority Priority
}

// Empty returns true if the driver is nil
func (d DriverDef) Empty() bool {
	return d.Name == ""
}

func (d DriverDef) String() string {
	return d.Name
}

type driverRegistry struct {
	drivers map[string]DriverDef
	lock    sync.Mutex
}

func newRegistry() *driverRegistry {
	return &driverRegistry{
		drivers: make(map[string]DriverDef),
	}
}

// Register registers a driver
func (r *driverRegistry) Register(def DriverDef) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.drivers[def.Name]; ok {
		return fmt.Errorf("%q is already registered: %+v", def.Name, def)
	}

	r.drivers[def.Name] = def
	return nil
}

// List returns a list of registered drivers
func (r *driverRegistry) List() []DriverDef {
	r.lock.Lock()
	defer r.lock.Unlock()

	result := make([]DriverDef, 0, len(r.drivers))

	for _, def := range r.drivers {
		result = append(result, def)
	}

	return result
}

// Driver returns a driver given a name
func (r *driverRegistry) Driver(name string) DriverDef {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.drivers[name]
}
