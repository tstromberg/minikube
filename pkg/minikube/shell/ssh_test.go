package rexec

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestSSHRun(t *testing.T) {
	var tests = []struct {
		cmd     string
		wantErr bool
	}{
		{"exit 0", false},
		{"exit 1", true},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			l := NewSSH(FakeSSHClient(t))
			got := l.Run(tc.cmd)
			if got == nil && tc.wantErr {
				t.Errorf("Run(%s) = %v, wanted error", tc.cmd, got)
			}
			if got != nil && !tc.wantErr {
				t.Errorf("Run(%s) = %v, wanted no error", tc.cmd, got)
			}
		})
	}
}

func TestSSHOut(t *testing.T) {
	var tests = []struct {
		cmd        string
		wantStdout []byte
		wantStderr []byte
		wantErr    bool
	}{
		{"exit 0", nil, nil, false},
		{"exit 1", nil, nil, true},

		{"echo tt", []byte{'t', 't', '\n'}, nil, false},
		{"echo tt 1>&2", nil, []byte{'t', 't', '\n'}, false},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			l := NewSSH(FakeSSHClient(t))
			gotStdout, gotStderr, err := l.Out(tc.cmd)
			if err == nil && tc.wantErr {
				t.Errorf("Out.error = %v, want error", err)
			}
			if err != nil && !tc.wantErr {
				t.Errorf("Out.error = %v, want no error", err)
			}
			if !cmp.Equal(gotStdout, tc.wantStdout) {
				t.Errorf("Out.stdout = %v, want %v", gotStdout, tc.wantStdout)
			}
			if !cmp.Equal(gotStderr, tc.wantStderr) {
				t.Errorf("Out.stderr = %v, want %v", gotStderr, tc.wantStderr)
			}
		})
	}
}

func TestSSHCombined(t *testing.T) {
	var tests = []struct {
		cmd     string
		wantOut []byte
		wantErr bool
	}{
		{"exit 0", nil, false},
		{"exit 1", nil, true},

		{"echo -n t", []byte{'t'}, false},
		{"echo tt 1>&2", []byte{'t', 't', '\n'}, false},
		{"echo y; echo t 1>&2", []byte{'y', '\n', 't', '\n'}, false},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			l := NewSSH(FakeSSHClient(t))
			got, err := l.Combined(tc.cmd)
			if err == nil && tc.wantErr {
				t.Errorf("Out.error = %v, want error", err)
			}
			if err != nil && !tc.wantErr {
				t.Errorf("Out.error = %v, want no error", err)
			}
			if !cmp.Equal(got, tc.wantOut) {
				t.Errorf("Out = %v, want %v", got, tc.wantOut)
			}
		})
	}
}

func TestSSHStream(t *testing.T) {
	var tests = []struct {
		cmd        string
		wantStdout []byte
		wantStderr []byte
	}{
		// NOTE: sleep(1) only accepts sub-second sleep statements on Linux and Darwin
		{"echo t; sleep 0.2", []byte{'t', '\n'}, nil},
		{"echo t 1>&2; sleep 0.2", nil, []byte{'t', '\n'}},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			l := NewSSH(FakeSSHClient(t))
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			w, err := l.Stream(tc.cmd, &stdout, &stderr)
			if err != nil {
				t.Errorf("unepexected Stream error: %v", err)
			}

			// Sleep for half the expected runtime
			time.Sleep(100 * time.Millisecond)
			gotStdout := stdout.Bytes()
			gotStderr := stderr.Bytes()

			if !cmp.Equal(gotStdout, tc.wantStdout) {
				t.Errorf("Stream.stdout = %v, want %v", gotStdout, tc.wantStdout)
			}
			if !cmp.Equal(gotStderr, tc.wantStderr) {
				t.Errorf("Stream.stderr = %v, want %v", gotStderr, tc.wantStderr)
			}

			err = w.Wait()
			if err != nil {
				t.Errorf("unepexected Wait error: %v", err)
			}
		})
	}
}

func TestTeePrefix(t *testing.T) {
	var in bytes.Buffer
	var out bytes.Buffer
	var logged strings.Builder

	logSink := func(format string, args ...interface{}) {
		logged.WriteString("(" + fmt.Sprintf(format, args...) + ")")
	}

	// Simulate the primary use case: tee in the background. This also helps avoid I/O races.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		TeePrefix(":", &in, &out, logSink)
		wg.Done()
	}()

	in.Write([]byte("goo"))
	in.Write([]byte("\n"))
	in.Write([]byte("g\r\n\r\n"))
	in.Write([]byte("le"))
	wg.Wait()

	gotBytes := out.Bytes()
	wantBytes := []byte("goo\ng\r\n\r\nle")
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Errorf("output=%q, want: %q", gotBytes, wantBytes)
	}

	gotLog := logged.String()
	wantLog := "(:goo)(:g)(:le)"
	if gotLog != wantLog {
		t.Errorf("log=%q, want: %q", gotLog, wantLog)
	}
}
