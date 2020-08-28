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
	"strings"

	"k8s.io/minikube/pkg/minikube/out/register"
	"k8s.io/minikube/pkg/minikube/problem"
	"k8s.io/minikube/pkg/minikube/translate"
)

// Problem displays a problem
func Problem(p problem.Problem, format string, a ...V) {
	if JSON {
		displayJSON(p, format, a...)
	} else {
		displayText(p, format, a...)
	}
}

func displayText(p problem.Problem, format string, a ...V) {
	Ln("")
	ErrT(FailureType, format, a...)
	ErrT(Tip, "Suggestion: {{.advice}}", V{"advice": translate.T(p.Advice)})
	if p.URL != "" {
		ErrT(Documentation, "Documentation: {{.url}}", V{"url": p.URL})
	}

	issueURLs := p.IssueURLs()
	if len(issueURLs) > 0 {
		ErrT(Issues, "Related issues:")
		for _, i := range issueURLs {
			ErrT(Issue, "{{.url}}", V{"url": i})
		}
	}

	if p.ShowNewIssueLink {
		ErrT(Empty, "")
		ErrT(Sad, "If the above advice does not help, please let us know: ")
		ErrT(URL, "https://github.com/kubernetes/minikube/issues/new/choose")
	}
}

func displayJSON(p problem.Problem, format string, a ...V) {
	msg := ApplyTemplateFormatting(FailureType, false, format, a...)
	register.PrintErrorExitCode(strings.TrimSpace(msg), p.ExitCode, map[string]string{
		"name":   p.ID,
		"advice": p.Advice,
		"url":    p.URL,
		"issues": strings.Join(p.IssueURLs(), ","),
	})
}
