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

// Package console provides a mechanism for sending localized, stylized output to the console.
package console

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
	isatty "github.com/mattn/go-isatty"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// By design, this package uses global references to language and output objects, in preference
// to passing a console object throughout the code base. Typical usage is:
//
// console.SetOutFile(os.Stdout)
// console.Out("Starting up!")
// console.OutStyle("status-change", "Configuring things")

// console.SetErrFile(os.Stderr)
// console.Fatal("Oh no, everything failed.")

// NOTE: If you do not want colorized output, set MINIKUBE_IN_COLOR=false in your environment.

var (
	// outFile is where Out* functions send output to. Set using SetOutFile()
	outFile fdWriter
	// errFile is where Err* functions send output to. Set using SetErrFile()
	errFile fdWriter
	// preferredLanguage is the default language messages will be output in
	preferredLanguage = language.AmericanEnglish
	// our default language
	defaultLanguage = language.AmericanEnglish
	// useColor is whether or not color output should be used, updated by Set*Writer.
	useColor = false
	// OverrideEnv is the environment variable used to override color/emoji usage
	OverrideEnv = "MINIKUBE_IN_COLOR"
)

// fdWriter is the subset of file.File that implements io.Writer and Fd()
type fdWriter interface {
	io.Writer
	Fd() uintptr
}

// HasStyle checks if a style exists
func HasStyle(style string) bool {
	return hasStyle(style)
}

// OutStyle writes a stylized and formatted message to stdout
func OutStyle(style, format string, a ...interface{}) error {
	OutStyle, err := applyStyle(style, useColor, fmt.Sprintf(format, a...))
	if err != nil {
		glog.Errorf("applyStyle(%s): %v", style, err)
		if oerr := OutLn(format, a...); oerr != nil {
			glog.Errorf("Out failed: %v", oerr)
		}
		return err
	}
	return Out(OutStyle)
}

// Out writes a basic formatted string to stdout
func Out(format string, a ...interface{}) error {
	p := message.NewPrinter(preferredLanguage)
	if outFile == nil {
		if _, err := p.Fprintf(os.Stdout, "(stdout unset)"+format, a...); err != nil {
			return err
		}
		return fmt.Errorf("no output file has been set")
	}
	_, err := p.Fprintf(outFile, format, a...)
	return err
}

// OutLn writes a basic formatted string with a newline to stdout
func OutLn(format string, a ...interface{}) error {
	return Out(format+"\n", a...)
}

// ErrStyle writes a stylized and formatted error message to stderr
func ErrStyle(style, format string, a ...interface{}) error {
	format, err := applyStyle(style, useColor, fmt.Sprintf(format, a...))
	if err != nil {
		glog.Errorf("applyStyle(%s): %v", style, err)
		if oerr := ErrLn(format, a...); oerr != nil {
			glog.Errorf("Err(%s) failed: %v", format, oerr)
		}
		return err
	}
	return Err(format)
}

// Err writes a basic formatted string to stderr
func Err(format string, a ...interface{}) error {
	p := message.NewPrinter(preferredLanguage)
	if errFile == nil {
		if _, err := p.Fprintf(os.Stderr, "(stderr unset)"+format, a...); err != nil {
			return err
		}
		return fmt.Errorf("no error file has been set")
	}
	_, err := p.Fprintf(errFile, format, a...)
	return err
}

// ErrLn writes a basic formatted string with a newline to stderr
func ErrLn(format string, a ...interface{}) error {
	return Err(format+"\n", a...)
}

// Success is a shortcut for writing a styled success message to stdout
func Success(format string, a ...interface{}) error {
	return OutStyle("success", format, a...)
}

// Fatal is a shortcut for writing a styled fatal message to stderr
func Fatal(format string, a ...interface{}) error {
	return ErrStyle("fatal", format, a...)
}

// Warning is a shortcut for writing a styled warning message to stderr
func Warning(format string, a ...interface{}) error {
	return ErrStyle("warning", format, a...)
}

// Failure is a shortcut for writing a styled failure message to stderr
func Failure(format string, a ...interface{}) error {
	return ErrStyle("failure", format, a...)
}

// SetPreferredLanguageTag configures which language future messages should use.
func SetPreferredLanguageTag(l language.Tag) {
	glog.Infof("Setting Language to %s ...", l)
	preferredLanguage = l
}

// SetPreferredLanguage configures which language future messages should use, based on a LANG string.
func SetPreferredLanguage(s string) error {
	// "C" is commonly used to denote a neutral POSIX locale. See http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap07.html#tag_07_02
	if s == "" || s == "C" {
		SetPreferredLanguageTag(defaultLanguage)
		return nil
	}
	// Handles "de_DE" or "de_DE.utf8"
	// We don't process encodings, since Rob Pike invented utf8 and we're mostly stuck with it.
	parts := strings.Split(s, ".")
	l, err := language.Parse(parts[0])
	if err != nil {
		return err
	}
	SetPreferredLanguageTag(l)
	return nil
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
	// MINIKUBE_IN_COLOR=[1, T, true, TRUE]
	// MINIKUBE_IN_COLOR=[0, f, false, FALSE]
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
	// Example: term-256color
	if !strings.Contains(term, "color") {
		glog.Infof("TERM=%s, which probably does not support color", term)
		return false
	}

	isT := isatty.IsTerminal(fd)
	glog.Infof("isatty.IsTerminal(%d) = %v\n", fd, isT)
	return isT
}
