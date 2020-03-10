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

package out

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"k8s.io/minikube/pkg/minikube/tests"
	"k8s.io/minikube/pkg/minikube/translate"
)

func TestOutT(t *testing.T) {
	// Set the system locale to Arabic and define a dummy translation file.
	if err := translate.SetPreferredLanguage("ar"); err != nil {
		t.Fatalf("SetPreferredLanguage: %v", err)
	}
	translate.Translations = map[string]interface{}{
		"Installing Kubernetes version {{.version}} ...": "... {{.version}} تثبيت Kubernetes الإصدار",
	}

	var testCases = []struct {
		style     StyleEnum
		message   string
		params    V
		want      string
		wantASCII string
	}{
		{Happy, "Happy", nil, "😄  Happy\n", "* Happy\n"},
		{Option, "Option", nil, "    ▪ Option\n", "  - Option\n"},
		{Warning, "Warning", nil, "❗  Warning\n", "! Warning\n"},
		{FatalType, "Fatal: {{.error}}", V{"error": "ugh"}, "💣  Fatal: ugh\n", "X Fatal: ugh\n"},
		{Issue, "http://i/{{.number}}", V{"number": 10000}, "    ▪ http://i/10000\n", "  - http://i/10000\n"},
		{Usage, "raw: {{.one}} {{.two}}", V{"one": "'%'", "two": "%d"}, "💡  raw: '%' %d\n", "* raw: '%' %d\n"},
		{Running, "Installing Kubernetes version {{.version}} ...", V{"version": "v1.13"}, "🏃  ... v1.13 تثبيت Kubernetes الإصدار\n", "* ... v1.13 تثبيت Kubernetes الإصدار\n"},
	}
	for _, tc := range testCases {
		for _, override := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s-override-%v", tc.message, override), func(t *testing.T) {
				// Set MINIKUBE_IN_STYLE=<override>
				os.Setenv(OverrideEnv, strconv.FormatBool(override))
				f := tests.NewFakeFile()
				SetOutFile(f)
				T(tc.style, tc.message, tc.params)
				got := f.String()
				want := tc.wantASCII
				if override {
					want = tc.want
				}
				if got != want {
					t.Errorf("OutStyle() = %q (%d runes), want %q (%d runes)", got, len(got), want, len(want))
				}
			})
		}
	}
}

func TestOut(t *testing.T) {
	os.Setenv(OverrideEnv, "")

	var testCases = []struct {
		format string
		arg    interface{}
		want   string
	}{
		{format: "xyz123", want: "xyz123"},
		{format: "Installing Kubernetes version %s ...", arg: "v1.13", want: "Installing Kubernetes version v1.13 ..."},
		{format: "Parameter encoding: %s", arg: "%s%%%d", want: "Parameter encoding: %s%%%d"},
	}
	for _, tc := range testCases {
		t.Run(tc.format, func(t *testing.T) {
			f := tests.NewFakeFile()
			SetOutFile(f)
			ErrLn("unrelated message")
			if tc.arg == nil {
				String(tc.format)
			} else {
				String(tc.format, tc.arg)
			}
			got := f.String()
			if got != tc.want {
				t.Errorf("Out(%s, %s) = %q, want %q", tc.format, tc.arg, got, tc.want)
			}
		})
	}
}

func TestErr(t *testing.T) {
	os.Setenv(OverrideEnv, "0")
	f := tests.NewFakeFile()
	SetErrFile(f)
	Err("xyz123 %s\n", "%s%%%d")
	Ln("unrelated message")
	got := f.String()
	want := "xyz123 %s%%%d\n"

	if got != want {
		t.Errorf("Err() = %q, want %q", got, want)
	}
}
