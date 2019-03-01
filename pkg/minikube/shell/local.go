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

package shell

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

// NewLocal returns a new local shell
func NewLocal(c Config) *Local {
	return &Local{config: c}
}

// Local runs commands locally, implementing the Commander interface.
type Local struct {
	config Config
}

// Run executes a command
func (l *Local) Run(cmd string) error {
	_, err := l.Output(cmd)
	return err
}

// Out executes a command, returning output
func (l *Local) Output(cmd string) (*OutResult, error) {
	var combined singleWriter
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	waiter, err := l.Stream(cmd, StreamOpts{&stdout, &stderr, &combined})
	if err != nil {
		return nil, err
	}
	err = waiter.Wait()
	return &OutResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		Combined: combined.Bytes(),
		ExitCode: waiter.ExitCode(),
	}, err
}

// Stream executes a command, streaming stdout, stderr appropriately
func (l *Local) Stream(cmd string, opts StreamOpts) (Waiter, error) {
	glog.Infof("Local: %s", cmd)

	var ew, ow []io.Writer
	if opts.Stderr != nil {
		ew = append(ew, opts.Stderr)
	}
	if opts.Stdout != nil {
		ow = append(ow, opts.Stdout)
	}
	if opts.Combined != nil {
		ew = append(ew, opts.Combined)
		ow = append(ow, opts.Combined)
	}

	c := exec.Command("/bin/bash", "-c", cmd)
	var wg sync.WaitGroup
	if len(ew) > 0 {
		errPipe, err := c.StderrPipe()
		if err != nil {
			return nil, errors.Wrap(err, "stderr")
		}
		wg.Add(1)
		go func() {
			if err := LogTee(l.config.StderrLogPrefix, l.config.Logger, errPipe, ew...); err != nil {
				glog.Errorf("tee stderr: %v", err)
			}
			wg.Done()
		}()
	}

	if len(ow) > 0 {
		outPipe, err := c.StdoutPipe()
		if err != nil {
			return nil, errors.Wrap(err, "stdout")
		}
		wg.Add(1)
		go func() {
			if err := LogTee(l.config.StdoutLogPrefix, l.config.Logger, outPipe, ow...); err != nil {
				glog.Errorf("tee stdout: %v", err)
			}
			wg.Done()
		}()
	}
	err := c.Start()
	return &LocalWaiter{wg: wg, cmd: c}, err
}

// Copy copies a source path to a target path
func (l *Local) Copy(src string, target string, perms os.FileMode) error {
	return copyToWriteFile(src, target, perms, l)
}

// WriteFile writes content to a target path
func (l *Local) WriteFile(src io.Reader, target string, len int64, perms os.FileMode) error {
	tdir := filepath.Dir(target)
	if _, err := os.Stat(tdir); os.IsNotExist(err) {
		dperm := perms | 0700
		glog.Infof("Recursively creating %s (perm=%s)", tdir, dperm)
		err := os.MkdirAll(filepath.Dir(target), perms|0700)
		if err != nil {
			glog.Errorf("failed to create directories: %s", err)
		}
	}

	glog.Infof("Writing %d bytes to %s locally (perm=%s)", len, target, perms)
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perms)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, src)
	if err != nil {
		return err
	}
	return f.Close()
}

// LocalWaiter is returned by Stream so callers can block until completion
type LocalWaiter struct {
	wg  sync.WaitGroup
	cmd *exec.Cmd
}

// ExitCode returns the exit code from the stream. Only usable after Wait()
func (lw *LocalWaiter) ExitCode() int {
	return lw.cmd.ProcessState.ExitCode()
}

// Wait waits until the command and byte buffers are closed
func (lw *LocalWaiter) Wait() error {
	err := lw.cmd.Wait()
	if err != nil {
		return err
	}
	lw.wg.Wait()
	return nil
}
