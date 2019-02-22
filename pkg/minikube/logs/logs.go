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

// package logs are convenience methods for fetching logs from a minikube cluster
package logs

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/golang/glog"
	"k8s.io/minikube/pkg/minikube/bootstrapper"
	"k8s.io/minikube/pkg/minikube/console"
	"k8s.io/minikube/pkg/minikube/cruntime"
	"k8s.io/minikube/pkg/minikube/rexec"
)

// rootCauseRe is a regular expression that matches known failure root causes
var rootCauseRe = regexp.MustCompile(`^error: |eviction manager: pods.* evicted|unknown flag: --`)

// importantPods are a list of pods to retrieve logs for, in addition to the bootstrapper logs.
var importantPods = []string{
	"k8s_kube-apiserver",
	"k8s_coredns_coredns",
	"k8s_kube-scheduler",
}

// lookbackwardsCount is how far back to look in a log for problems. This should be large enough to
// include usage messages from a failed binary, but small enough to not include irrelevant problems.
const lookBackwardsCount = 200

// Follow follows logs from multiple files in tail(1) format
func Follow(r cruntime.Manager, bs bootstrapper.Bootstrapper, st rexec.Streamer) (rexec.Waiter, error) {
	cs := []string{}
	for _, v := range logCommands(r, bs, 0, true) {
		cs = append(cs, v+" &")
	}
	cs = append(cs, "wait")
	return st.Stream(strings.Join(cs, " "), os.Stdout, os.Stdout)
}

// IsProblem returns whether this line matches a known problem
func IsProblem(line string) bool {
	return rootCauseRe.MatchString(line)
}

// FindProblems finds possible root causes among the logs
func FindProblems(r cruntime.Manager, bs bootstrapper.Bootstrapper, st rexec.Streamer) map[string][]string {
	pMap := map[string][]string{}
	cmds := logCommands(r, bs, lookBackwardsCount, false)
	for name, cmd := range cmds {
		glog.Infof("Gathering logs for %s ...", name)
		var b bytes.Buffer
		waiter, err := st.Stream(cmds[name], &b, &b)
		if err != nil {
			glog.Warningf("failed %s: %s: %v", name, cmd, err)
			continue
		}
		scanner := bufio.NewScanner(&b)
		problems := []string{}
		for scanner.Scan() {
			l := scanner.Text()
			if IsProblem(l) {
				glog.Warningf("Found %s problem: %s", name, l)
				problems = append(problems, l)
			}
		}
		if err := waiter.Wait(); err != nil {
			glog.Warningf("failed wait: %v", err)
		}
		if len(problems) > 0 {
			pMap[name] = problems
		}
	}
	return pMap
}

// OutputProblems outputs discovered problems.
func OutputProblems(problems map[string][]string, maxLines int) {
	for name, lines := range problems {
		console.OutStyle("failure", "Problems detected in %q:", name)
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		for _, l := range lines {
			console.OutStyle("log-entry", l)
		}
	}
}

// Output displays logs from multiple sources in tail(1) format
func Output(r cruntime.Manager, bs bootstrapper.Bootstrapper, st rexec.Streamer, lines int) error {
	cmds := logCommands(r, bs, lines, false)
	names := []string{}
	for k := range cmds {
		names = append(names, k)
	}
	sort.Strings(names)

	failed := []string{}
	for _, name := range names {
		console.OutLn("==> %s <==", name)
		var b bytes.Buffer
		waiter, err := st.Stream(cmds[name], &b, &b)
		if err != nil {
			glog.Errorf("failed: %v", err)
			failed = append(failed, name)
			continue
		}
		scanner := bufio.NewScanner(&b)
		for scanner.Scan() {
			console.OutLn(scanner.Text())
		}
		if err := waiter.Wait(); err != nil {
			glog.Errorf("wait failed: %v", err)
		}
		console.OutLn("")
	}
	if len(failed) > 0 {
		return fmt.Errorf("unable to fetch logs for: %s", strings.Join(failed, ", "))
	}
	return nil
}

// logCommands returns a list of commands that would be run to receive the anticipated logs
func logCommands(r cruntime.Manager, bs bootstrapper.Bootstrapper, length int, follow bool) map[string]string {
	cmds := bs.LogCommands(bootstrapper.LogOptions{Lines: length, Follow: follow})
	for _, pod := range importantPods {
		ids, err := r.ListContainers(pod)
		if err != nil {
			glog.Errorf("Failed to list containers for %q: %v", pod, err)
			continue
		}
		glog.Infof("%d containers: %s", len(ids), ids)
		if len(ids) == 0 {
			cmds[pod] = fmt.Sprintf("No container was found matching %q", pod)
			continue
		}
		cmds[pod] = r.ContainerLogCmd(ids[0], length, follow)
	}
	return cmds
}
