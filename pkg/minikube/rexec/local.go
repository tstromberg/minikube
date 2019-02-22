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

package rexec

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/util"
)

// Local runs commands locally, implementing the FullRunner interface.
type Local struct{}

// NewSSH returns a new SSH implementation
func NewLocal() *Local {
	return &Local{}
}

// Run executes a command
func (l *Local) Run(cmd string) error {
	_, _, err := streamToOut(cmd, l)
	return err
}

// Out executes a command, returning stdout, stderr.
func (l *Local) Out(cmd string) ([]byte, []byte, error) {
	return streamToOut(cmd, l)
}

// Combined executes a command, returning a combined stdout and stderr
func (l *Local) Combined(cmd string) ([]byte, error) {
	return streamToCombined(cmd, l)
}

// Stream executes a command, streaming stdout, stderr appropriately
func (l *Local) Stream(cmd string, stdout io.Writer, stderr io.Writer) (Waiter, error) {
	glog.Infof("Local: %s", cmd)

	c := exec.Command("/bin/sh", "-c", cmd)
	outPipe, err := c.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "stdout")
	}

	errPipe, err := c.StderrPipe()
	if err != nil {
		return nil, errors.Wrap(err, "stderr")
	}
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		if err := util.TeePrefix(util.ErrPrefix, errPipe, stderr, glog.Infof); err != nil {
			glog.Errorf("tee stderr: %v", err)
		}
		wg.Done()
	}()
	go func() {
		if err := util.TeePrefix(util.OutPrefix, outPipe, stdout, glog.Infof); err != nil {
			glog.Errorf("tee stdout: %v", err)
		}
		wg.Done()
	}()

	err = c.Start()
	return c, err
}

// Copy copies a source path to a target path
func (l *Local) Copy(src string, target string, perms os.FileMode) error {
	return copyToWriteFile(src, target, perms, l)
}

// WriteFile writes content to a target path
func (l *Local) WriteFile(src io.Reader, target string, len int64, perms os.FileMode) error {
	glog.Infof("Writing %d bytes to %s locally (perm=%s)", len, target, perms)
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	if err := os.Chmod(target, perms); err != nil {
		return err
	}

	_, err = io.Copy(f, src)
	if err != nil {
		return err
	}
	return f.Close()
}
