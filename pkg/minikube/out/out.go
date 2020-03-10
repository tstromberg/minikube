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

// Package out provides a mechanism for sending localized, stylized output to the console.
package out

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
	isatty "github.com/mattn/go-isatty"
)

// By design, this package uses global references to language and output objects, in preference
// to passing a console object throughout the code base. Typical usage is:
//
// out.SetOutFile(os.Stdout)
// out.String("Starting up!")
// out.T(out.StatusChange, "Configuring things")

// out.SetErrFile(os.Stderr)
// out.Fatal("Oh no, everything failed.")

// NOTE: If you do not want colorized output, set MINIKUBE_IN_STYLE=false in your environment.

var (
	// outFile is where Out* functions send output to. Set using SetOutFile()
	outFile fdWriter
	// errFile is where Err* functions send output to. Set using SetErrFile()
	errFile fdWriter
	// useColor is whether or not color output should be used, updated by Set*Writer.
	useColor = false
	// OverrideEnv is the environment variable used to override color/emoji usage
	OverrideEnv = "MINIKUBE_IN_STYLE"
)

// fdWriter is the subset of file.File that implements io.Writer and Fd()
type fdWriter interface {
	io.Writer
	Fd() uintptr
}

// V is a convenience wrapper for templating, it represents the variable key/value pair.
type V map[string]interface{}

// T writes a stylized and templated message to stdout
func T(style StyleEnum, format string, a ...V) {
	outStyled := applyTemplateFormatting(style, useColor, format, a...)
	String(outStyled)
}

// String writes a basic formatted string to stdout
func String(format string, a ...interface{}) {
	if outFile == nil {
		glog.Warningf("[unset outFile]: %s", fmt.Sprintf(format, a...))
		return
	}
	_, err := fmt.Fprintf(outFile, format, a...)
	if err != nil {
		glog.Errorf("Fprintf failed: %v", err)
	}
}

// Ln writes a basic formatted string with a newline to stdout
func Ln(format string, a ...interface{}) {
	String(format+"\n", a...)
}

// ErrT writes a stylized and templated error message to stderr
func ErrT(style StyleEnum, format string, a ...V) {
	errStyled := applyTemplateFormatting(style, useColor, format, a...)
	Err(errStyled)
}

// Err writes a basic formatted string to stderr
func Err(format string, a ...interface{}) {
	if errFile == nil {
		glog.Errorf("[unset errFile]: %s", fmt.Sprintf(format, a...))
		return
	}
	_, err := fmt.Fprintf(errFile, format, a...)
	if err != nil {
		glog.Errorf("Fprint failed: %v", err)
	}
}

// ErrLn writes a basic formatted string with a newline to stderr
func ErrLn(format string, a ...interface{}) {
	Err(format+"\n", a...)
}

// SuccessT is a shortcut for writing a templated success message to stdout
func SuccessT(format string, a ...V) {
	T(SuccessType, format, a...)
}

// FatalT is a shortcut for writing a templated fatal message to stderr
func FatalT(format string, a ...V) {
	ErrT(FatalType, format, a...)
}

// WarningT is a shortcut for writing a templated warning message to stderr
func WarningT(format string, a ...V) {
	ErrT(Warning, format, a...)
}

// FailureT is a shortcut for writing a templated failure message to stderr
func FailureT(format string, a ...V) {
	ErrT(FailureType, format, a...)
}

// SetOutFile configures which writer standard output goes to.
func SetOutFile(w fdWriter) {
	glog.Infof("Setting OutFile to fd %d ...", w.Fd())
	outFile = w
	useColor = wantsColor(w.Fd())
}

// SetErrFile configures which writer error output goes to.
func SetErrFile(w fdWriter) {
	glog.Infof("Setting ErrFile to fd %d...", w.Fd())
	errFile = w
	useColor = wantsColor(w.Fd())
}

// wantsColor determines if the user might want colorized output.
func wantsColor(fd uintptr) bool {
	// First process the environment: we allow users to force colors on or off.
	//
	// MINIKUBE_IN_STYLE=[1, T, true, TRUE]
	// MINIKUBE_IN_STYLE=[0, f, false, FALSE]
	//
	// If unset, we try to automatically determine suitability from the environment.
	val := os.Getenv(OverrideEnv)
	if val != "" {
		glog.Infof("%s=%q\n", OverrideEnv, os.Getenv(OverrideEnv))
		override, err := strconv.ParseBool(val)
		if err != nil {
			// That's OK, we will just fall-back to automatic detection.
			glog.Errorf("ParseBool(%s): %v", OverrideEnv, err)
		} else {
			return override
		}
	}

	term := os.Getenv("TERM")
	colorTerm := os.Getenv("COLORTERM")
	// Example: term-256color
	if !strings.Contains(term, "color") && !strings.Contains(colorTerm, "truecolor") && !strings.Contains(colorTerm, "24bit") && !strings.Contains(colorTerm, "yes") {
		glog.Infof("TERM=%s,COLORTERM=%s, which probably does not support color", term, colorTerm)
		return false
	}

	isT := isatty.IsTerminal(fd)
	glog.Infof("isatty.IsTerminal(%d) = %v\n", fd, isT)
	return isT
}
