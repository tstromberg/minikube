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

// Package exit contains functions useful for exiting gracefully.
package exit

import (
	"os"
	"runtime"
	"runtime/debug"

	"github.com/golang/glog"
	"k8s.io/minikube/pkg/minikube/out"
	"k8s.io/minikube/pkg/minikube/problem"
)

// UsageT outputs a templated usage error and exits with error code 64
func UsageT(format string, a ...out.V) {
	exitcode := problem.ProgramUsage
	out.ErrWithExitCode(out.Usage, format, exitcode, a...)
	os.Exit(exitcode)
}

// WithCodeT outputs a templated fatal error message and exits with the supplied error code.
func WithCodeT(code int, format string, a ...out.V) {
	out.ErrWithExitCode(out.FatalType, format, code, a...)
	os.Exit(code)
}

// WithError outputs an error and exits.
func WithError(id string, msg string, err error) {
	glog.Infof("WithError(%s, %v) called from:\n%s", msg, err, debug.Stack())
	problem := problem.FromError(id, err, runtime.GOOS)
	WithProblem(*problem, "Error: {{.err}}", out.V{"err": err})
}

// WithProblem outputs an error and exits.
func WithProblem(p problem.Problem, format string, a ...out.V) {
	glog.Infof("WithProblem(%+v, %s, %s) called from:\n%s", p, format, a, debug.Stack())

	if p.ExitCode == 0 {
		p.ExitCode = problem.ProgramError
	}

	out.Problem(p, format, a...)
	os.Exit(p.ExitCode)
}
