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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
)

// NewSSH returns a new remote shell via SSH
func NewSSH(c Config) *Local {
	return &Local{config: c}
}

// SSH runs commands through SSH, implementing the FullRunner interface.
type SSH struct {
	config Config
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

// Run executes a command
func (s *SSH) Run(cmd string) error {
	_, err := s.Output(cmd)
	return err
}

// Out executes a command, returning output
func (s *SSH) Output(cmd string) (*OutResult, error) {
	var combined singleWriter
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	glog.Infof("SSH Out: %s", cmd)
	sess, err := s.config.SSHClient.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "ssh")
	}
	defer sess.Close()

	_, err = streamCmd(s.config, sess, &stdout, ioutil.NopCloser(&stderr), &combined)
	if err != nil {
		return nil, err
	}

	err = sess.Wait()
	return &OutResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		Combined: combined.Bytes(),
		ExitCode: -1,
	}, err
}

// Stream executes a command, writing stdout and stderr appropriately.
func (s *SSH) Stream(cmd string, stdout io.Writer, stderr io.Writer) (Waiter, error) {
	glog.Infof("SSH Stream: %s", cmd)
	sess, err := s.config.SSHClient.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "ssh")
	}
	defer sess.Close()

	_, err = streamCmd(s.config, sess, stdout, stderr, nil)
	if err != nil {
		return nil, err
	}
	err := sess.Start(cmd)
	return &SSHWaiter{sess: sess, cmd: c}, err
}

// Copy copies a source path to a target path
func (s *SSH) Copy(src string, target string, perms os.FileMode) error {
	return copyToWriteFile(src, target, perms, s)
}

// WriteFile writes content to a target path
func (s *SSH) WriteFile(src io.Reader, target string, len int64, perms os.FileMode) error {
	glog.Infof("Writing %d bytes to %s via ssh (perm=%s)", len, target, perms)
	sess, err := s.config.SSHClient.NewSession()
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

	if err = sess.Run(fmt.Sprintf("sudo scp -t %s", filepath.Dir(target))); err != nil {
		return errors.Wrap(err, "run")
	}

	if err = g.Wait(); err != nil {
		return errors.Wrap(err, "wait")
	}

	return sess.Close()
}

// SSHWaiter is returned by Stream so callers can block until completion
type SSHWaiter struct {
	wg   sync.WaitGroup
	sess *ssh.Session
}

// ExitCode returns the exit code from the stream. Only usable after Wait()
func (sw *SSHWaiter) ExitCode() int {
	// not yet implemented
	return -1
}

// Wait waits until the command and byte buffers are closed
func (sw *SSHWaiter) Wait() error {
	err := sw.sess.Wait()
	if err != nil {
		return err
	}
	sw.wg.Wait()
	return nil
}
