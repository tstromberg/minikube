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

package console

import (
	"bytes"
	"os"
	"testing"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// fakeFile satisfies fdWriter
type fakeFile struct {
	b bytes.Buffer
}

func newFakeFile() *fakeFile {
	return &fakeFile{}
}

func (f *fakeFile) Fd() uintptr {
	return uintptr(0)
}

func (f *fakeFile) Write(p []byte) (int, error) {
	return f.b.Write(p)
}
func (f *fakeFile) String() string {
	return f.b.String()
}

func TestOutStyle(t *testing.T) {

	var tests = []struct {
		style    string
		envValue string
		message  string
		want     string
	}{
		{"happy", "true", "This is happy.", "😄  This is happy.\n"},
		{"Docker", "true", "This is Docker.", "🐳  This is Docker.\n"},
		{"option", "true", "This is option.", "    ▪ This is option.\n"},

		{"happy", "false", "This is happy.", "o   This is happy.\n"},
		{"Docker", "false", "This is Docker.", "-   This is Docker.\n"},
		{"option", "false", "This is option.", "    - This is option.\n"},
	}
	for _, tc := range tests {
		t.Run(tc.style+"-"+tc.envValue, func(t *testing.T) {
			os.Setenv(OverrideEnv, tc.envValue)
			f := newFakeFile()
			SetOutFile(f)
			if err := OutStyle(tc.style, tc.message); err != nil {
				t.Errorf("unexpected error: %q", err)
			}
			got := f.String()
			if got != tc.want {
				t.Errorf("OutStyle() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOut(t *testing.T) {
	os.Setenv(OverrideEnv, "")
	// An example translation just to assert that this code path is executed.
	err := message.SetString(language.Arabic, "Installing Kubernetes version %s ...", "... %s تثبيت Kubernetes الإصدار")
	if err != nil {
		t.Fatalf("setstring: %v", err)
	}

	var tests = []struct {
		format string
		lang   language.Tag
		arg    string
		want   string
	}{
		{format: "xyz123", want: "xyz123"},
		{format: "Installing Kubernetes version %s ...", lang: language.Arabic, arg: "v1.13", want: "... v1.13 تثبيت Kubernetes الإصدار"},
		{format: "Installing Kubernetes version %s ...", lang: language.AmericanEnglish, arg: "v1.13", want: "Installing Kubernetes version v1.13 ..."},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			SetPreferredLanguageTag(tc.lang)
			f := newFakeFile()
			SetOutFile(f)
			ErrLn("unrelated message")
			if err := Out(tc.format, tc.arg); err != nil {
				t.Errorf("unexpected error: %q", err)
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
	f := newFakeFile()
	SetErrFile(f)
	if err := Err("xyz123\n"); err != nil {
		t.Errorf("unexpected error: %q", err)
	}

	OutLn("unrelated message")
	got := f.String()
	want := "xyz123\n"

	if got != want {
		t.Errorf("Err() = %q, want %q", got, want)
	}
}

func TestErrStyle(t *testing.T) {
	os.Setenv(OverrideEnv, "1")
	f := newFakeFile()
	SetErrFile(f)
	if err := ErrStyle("fatal", "It broke"); err != nil {
		t.Errorf("unexpected error: %q", err)
	}
	got := f.String()
	want := "💣  It broke\n"
	if got != want {
		t.Errorf("ErrStyle() = %q, want %q", got, want)
	}
}

func TestSetPreferredLanguage(t *testing.T) {
	os.Setenv(OverrideEnv, "0")
	var tests = []struct {
		input string
		want  language.Tag
	}{
		{"", language.AmericanEnglish},
		{"C", language.AmericanEnglish},
		{"zh", language.Chinese},
		{"fr_FR.utf8", language.French},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			// Set something so that we can assert change.
			SetPreferredLanguageTag(language.Icelandic)
			if err := SetPreferredLanguage(tc.input); err != nil {
				t.Errorf("unexpected error: %q", err)
			}

			// Just compare the bases ("en", "fr"), since I can't seem to refer directly to them
			want, _ := tc.want.Base()
			got, _ := preferredLanguage.Base()
			if got != want {
				t.Errorf("SetPreferredLanguage(%s) = %q, want %q", tc.input, got, want)
			}
		})
	}

}
