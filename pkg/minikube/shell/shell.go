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

// Package rcmd runs commands locally or remotely.
//
// Example:
//
// s := shell.Local()
// err := s.Run()
// if err != nil {
//	panic("in the disco")
// }
//
// o, err := s.Output()
// fmt.Println(o.Stdout)
//
//
package shell

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"sync"

	"github.com/golang/glog"
	"golang.org/x/crypto/ssh"
)

const (
	stderrLogPrefix = "! "
	stdoutLogPrefix = "> "
)

// Runner is an interface for running a command
type Runner interface {
	// Run executes a command
	Run(cmd string) error
	// Result executes a command returning output
	Output(cmd string) (*OutResult, error)
}

// OutResult contains output and an exit code
type OutResult struct {
	// Stdout is the bytes sent to stdout - useful for parsing
	Stdout []byte
	// Stderr is the bytes sent to stderr
	Stderr []byte
	// Combined is a combined stream of bytes sent to stdout/stderr - useful for error reporting.
	Combined []byte
	// ExitCode is the exit code from the command
	ExitCode int
}

// Streamer is an interface for running a command with streaming output
type Streamer interface {
	// Stream executes a command, streaming stdout, stderr appropriately
	Stream(cmd string, opts StreamOpts) (Waiter, error)
}

// Writer is an interface for writing content to the destination
type Writer interface {
	// Copy copies a source path to a target path
	Copy(src string, target string, perms os.FileMode) error
	// WriteFile writes content to a target path
	WriteFile(src io.Reader, target string, len int64, perms os.FileMode) error
}

// Waiter is returned by Stream so that callers may block until the command has completed
type Waiter interface {
	Wait() error
	ExitCode() int
}

// Commander is the complete interface to exec commands
type Commander interface {
	Runner
	Streamer
	Writer
}

// singleWriter is a writer which forces synchronized writes through locking.
type singleWriter struct {
	b  bytes.Buffer
	mu sync.Mutex
}

// Write bytes to the buffer, secured by a lock.
func (w *singleWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

// Bytes returns bytes written to the synchronized buffer
func (w *singleWriter) Bytes() []byte {
	return w.b.Bytes()
}

type logger func(format string, args ...interface{})

type Config struct {
	SSHClient       *ssh.Client
	Logger          logger
	StdoutLogPrefix string
	StderrLogPrefix string
}

// NewShell returns the appropriately configured Runner/Commander
func NewShell(c Config) Commander {
	// if c.SSHClient  != nil {
	// 	return SSH{config: c}
	// }
	if c.StdoutLogPrefix == "" {
		c.StdoutLogPrefix = stdoutLogPrefix
	}
	if c.StderrLogPrefix == "" {
		c.StderrLogPrefix = stderrLogPrefix
	}
	if c.Logger == nil {
		c.Logger = glog.Infof
	}
	return NewLocal(c)
}

// copyToWriteFile adapts the Copy interface to the WriteFile interface
func copyToWriteFile(src string, target string, perms os.FileMode, w Writer) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	return w.WriteFile(f, target, fi.Size(), perms)
}

// StreamOpts are options to pass to Stream
type StreamOpts struct {
	Stdout   io.Writer
	Stderr   io.Writer
	Combined io.Writer
}

// LogTee copies bytes from a reader to multiple writers, logging each new line.
func LogTee(prefix string, logger func(format string, args ...interface{}), r io.Reader, writers ...io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanBytes)
	var line bytes.Buffer

	for scanner.Scan() {
		b := scanner.Bytes()
		for _, w := range writers {
			if _, err := w.Write(b); err != nil {
				return err
			}
		}

		if bytes.IndexAny(b, "\r\n") == 0 {
			if line.Len() > 0 {
				logger("%s%s", prefix, line.String())
				line.Reset()
			}
			continue
		}
		line.Write(b)
	}
	// Catch trailing output in case stream does not end with a newline
	if line.Len() > 0 {
		logger("%s%s", prefix, line.String())
	}
	return nil
}
