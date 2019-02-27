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

// Package rexec provides an abstraction layer for running commands remotely or locally.
package rexec

import (
	"bytes"
	"io"
	"os"
	"sync"
)

// Waiter is returned by Stream
type Waiter interface {
	Wait() error
}

// Runner is an interface for running a command
type Runner interface {
	// Run executes a command
	Run(cmd string) error
}

// OutRunner is an interface to run a command returning output
type OutRunner interface {
	// Out executes a command, returning stdout, stderr.
	Out(cmd string) ([]byte, []byte, error)
}

// CombinedRunner is an interface for running a command with combined output
type CombinedRunner interface {
	// Combined executes a command, returning a combined stdout and stderr
	Combined(cmd string) ([]byte, error)
}

// Streamer is an interface for running a command with streaming output
type Streamer interface {
	// Stream executes a command, streaming stdout, stderr appropriately
	Stream(cmd string, stdout io.Writer, stderr io.Writer) (Waiter, error)
}

// Writer is an interface for writing content to the destination
type Writer interface {
	// Copy copies a source path to a target path
	Copy(src string, target string, perms os.FileMode) error
	// WriteFile writes content to a target path
	WriteFile(src io.Reader, target string, len int64, perms os.FileMode) error
}

// Executor is the complete interface to exec commands
type Executor interface {
	Runner
	OutRunner
	CombinedRunner
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

// streamToOut adapts the Stream interface to the Out interface
func streamToOut(cmd string, s Streamer) ([]byte, []byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	w, err := s.Stream(cmd, &stdout, &stderr)
	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), err
	}
	err = w.Wait()
	return stdout.Bytes(), stderr.Bytes(), err
}

// streamToCombined adapts the Stream interface to the Combined interface
func streamToCombined(cmd string, s Streamer) ([]byte, error) {
	var combined singleWriter
	w, err := s.Stream(cmd, &combined, &combined)
	if err != nil {
		return combined.Bytes(), err
	}
	err = w.Wait()
	return combined.Bytes(), err
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
