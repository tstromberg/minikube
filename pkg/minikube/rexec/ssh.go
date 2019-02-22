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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"k8s.io/minikube/pkg/util"
)

// SSH runs commands through SSH, implementing the FullRunner interface.
type SSH struct {
	c sshClient
}

// sshClient implements the ssh.Client methods used by this package
type sshClient interface {
	NewSession() (*ssh.Session, error)
}

// sshSession implements the ssh.Session methods used by this package
type sshSession interface {
	Close() error
	StderrPipe() (io.Reader, error)
	StdoutPipe() (io.Reader, error)
	StdinPipe() (io.WriteCloser, error)
	Start(cmd string) error
	Run(cmd string) error
	Wait() error
}

// NewSSH returns a new SSH implementation
func NewSSH(c sshClient) *SSH {
	return &SSH{c}
}

// Run executes a command
func (s *SSH) Run(cmd string) error {
	_, _, err := s.Out(cmd)
	return err
}

// Out executes a command, returning stdout, stderr.
func (s *SSH) Out(cmd string) ([]byte, []byte, error) {
	return streamToOut(cmd, s)
}

// Combined executes a command, returning a combined stdout and stderr
func (s *SSH) Combined(cmd string) ([]byte, error) {
	return streamToCombined(cmd, s)
}

// Stream executes a command, writing stdout and stderr appropriately.
func (s *SSH) Stream(cmd string, stdout io.Writer, stderr io.Writer) (Waiter, error) {
	glog.Infof("SSH: %s", cmd)
	sess, err := s.c.NewSession()
	if err != nil {
		return sess, errors.Wrap(err, "ssh")
	}
	defer sess.Close()

	outPipe, err := sess.StdoutPipe()
	if err != nil {
		return sess, errors.Wrap(err, "stdout")
	}

	errPipe, err := sess.StderrPipe()
	if err != nil {
		return sess, errors.Wrap(err, "stderr")
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

	err = sess.Start(cmd)
	return sess, err
}

// Copy copies a source path to a target path
func (s *SSH) Copy(src string, target string, perms os.FileMode) error {
	return copyToWriteFile(src, target, perms, s)
}

// WriteFile writes content to a target path
func (s *SSH) WriteFile(src io.Reader, target string, len int64, perms os.FileMode) error {
	glog.Infof("Writing %d bytes to %s via ssh (perm=%s)", len, target, perms)
	sess, err := s.c.NewSession()
	if err != nil {
		return errors.Wrap(err, "ssh")
	}

	w, err := sess.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "stdin")
	}
	defer w.Close()

	var g errgroup.Group
	g.Go(func() error {
		header := fmt.Sprintf("C%s %d %s\n", perms, len, target)
		_, err := fmt.Fprint(w, header)
		if err != nil {
			return errors.Wrap(err, "header")
		}
		_, err = io.Copy(w, src)
		if err != nil {
			return errors.Wrap(err, "copy")
		}
		_, err = fmt.Fprint(w, "\x00")
		if err != nil {
			return errors.Wrap(err, "tail")
		}
		return nil
	})

	if err = sess.Run(fmt.Sprintf("sudo scp -qt %s", filepath.Dir(target))); err != nil {
		return err
	}

	if err = g.Wait(); err != nil {
		return err
	}

	return sess.Close()
}
