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
// res, err := s.Result()
// fmt.Println(res.Stdout)
//
//
package command

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"sync"
)

// Runner is an interface for running a command
type Runner interface {
	// Run executes a command
	Run(cmd string) error
	// Result executes a command returning results and output
	Result(cmd string) (Result, error)
}

// Result contains output and an exit code
type Result struct {
	// ExitStatus is the exit code from the command
	ExitStatus int
	// Stdout is the bytes sent to stdout - useful for parsing
	Stdout []byte
	// Stderr is the bytes sent to stderr
	Stderr []byte
	// Combined is a combined stream of bytes sent to stdout/stderr - useful for error reporting.
	Combined []byte
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

// Waiter is returned by Stream
type Waiter interface {
	Wait() error
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

// TeePrefix copies bytes from a reader to writer, logging each new line.
func TeePrefix(prefix string, r io.Reader, w io.Writer, logger func(format string, args ...interface{})) error {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanBytes)
	var line bytes.Buffer

	for scanner.Scan() {
		b := scanner.Bytes()
		if _, err := w.Write(b); err != nil {
			return err
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
