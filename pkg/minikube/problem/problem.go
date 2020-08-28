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

package problem

import (
	"fmt"
	"regexp"
)

const issueBase = "https://github.com/kubernetes/minikube/issues"

// Problem represents a known issue in minikube.
type Problem struct {
	// ID is an arbitrary unique and stable string describing this issue
	ID string
	// Regexp is which regular expression this issue matches
	Regexp *regexp.Regexp
	// Operating systems this error is specific to
	GOOS []string

	// Advice is actionable text that the user should follow
	Advice string
	// URL is a reference URL for more information
	URL string
	// Issues are a list of related issues to this Problem
	Issues []int
	// Show the new issue link
	ShowNewIssueLink bool
	// ExitCode to be used (defaults to 1)
	ExitCode int
	// ErrorStyle
	ErrorStyle string
}

func (p *Problem) IssueURLs() []string {
	is := []string{}
	for _, i := range p.Issues {
		is = append(is, fmt.Sprintf("%s/%d", issueBase, i))
	}
	return is
}

func knownIssues() []Problem {
	ps := []Problem{}
	// This is intentionally in dependency order
	ps = append(ps, ProgramIssues...)
	ps = append(ps, ResourceIssues...)
	ps = append(ps, HostIssues...)
	ps = append(ps, ProviderIssues...)
	ps = append(ps, DriverIssues...)
	ps = append(ps, LocalNetworkIssues...)
	ps = append(ps, InternetIssues...)
	ps = append(ps, GuestIssues...)
	ps = append(ps, RuntimeIssues...)
	ps = append(ps, ControlPlaneIssues...)
	ps = append(ps, ServiceIssues...)
	return ps
}

// FromError returns a known issue from an error on an OS
func FromError(id string, err error, goos string) *Problem {
	var genericMatch *Problem

	for _, p := range knownIssues() {
		p := p
		if !p.Regexp.MatchString(err.Error()) {
			continue
		}

		// Does this match require an OS matchup?
		if len(p.GOOS) > 0 {
			for _, o := range p.GOOS {
				if o == goos {
					return &p
				}
			}
		}
		if genericMatch == nil {
			genericMatch = &p
		}
	}

	return genericMatch
}
