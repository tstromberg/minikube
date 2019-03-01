// +build darwin linux

package shell

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// fakeSSHClient implements a fake SSH client
type fakeSSHClient struct {
	t *testing.T
}

// fakeSSHSession implements a poorly written fake SSH session
type fakeSSHSession struct {
	stdin    bytes.Buffer
	stdout   bytes.Buffer
	stderr   bytes.Buffer
	exitCode int
	cmd      string
	t        *testing.T
}

// FakeSSHClient returns a fake ssh client
func FakeSSHClient(t *testing.T) sshClient {
	return &fakeSSHClient{t}
}

// NewSession returns a new fake SSH session
func (fc *fakeSSHClient) NewSession() (sshSession, error) {
	return &fakeSSHSession{
		exitCode: -1,
		cmd:      "",
		t:        fc.t,
	}, nil
}

// Close closes a fake SSH session
func (fs *fakeSSHSession) Close() error {
	return nil
}

// StderrPipe returns a pipe that will be connected to the fake commands stderr
func (fs *fakeSSHSession) StderrPipe() (io.Reader, error) {
	return bufio.NewReader(&fs.stderr), nil
}

// StdoutPipe returns a pipe that will be connected to the fake commands stdout
func (fs *fakeSSHSession) StdoutPipe() (io.Reader, error) {
	return bufio.NewReader(&fs.stdout), nil
}

// StdinPipe returns a pipe that will be connected to the fake commands stdin
func (fs *fakeSSHSession) StdinPipe() (io.WriteCloser, error) {
	return writeNopCloser{bufio.NewWriter(&fs.stdin)}, nil
}

// Start runs cmd on the fake host.
func (fs *fakeSSHSession) Start(cmd string) error {
	if fs.cmd != "" {
		return fmt.Errorf("fake: ssh.Session supports only one command per session")
	}
	fs.cmd = cmd
	fs.t.Logf("fake ssh running: %s", cmd)

	cmds := strings.Split(cmd, "; ")
	for _, c := range cmds {
		args := strings.Split(c, " ")
		switch args[0] {
		case "exit":
			fs.exitCode = 1
			if args[1] == "0" {
				fs.exitCode = 0
			}
		case "echo":
			fs.exitCode = 0
			if len(args) == 3 {
				if args[2] == "1>&2" {
					fs.t.Logf("fake: writing to stderr: %s\n", args[1])
					fs.stderr.Write([]byte(args[1]))
					fs.stderr.Write([]byte("\n"))
				} else if args[1] == "-n" {
					fs.t.Logf("fake: writing to stdout: %s", args[2])
					fs.stdout.Write([]byte(args[2]))
				}
			} else {
				fs.t.Logf("fake: writing to stdout: %s\n", args[1])
				fs.stdout.Write([]byte(args[1]))
				fs.stdout.Write([]byte{'\n'})
			}
		}
	}
	return nil
}

// Run runs cmd on the remote host, waiting for a return.
func (fs *fakeSSHSession) Run(cmd string) error {
	if fs.cmd != "" {
		return fmt.Errorf("fake: ssh.Session supports only one command per session")
	}
	return nil
}

// Wait waits for the fake command to exit.
func (fs *fakeSSHSession) Wait() error {
	if fs.exitCode == 0 {
		return nil
	}
	return fmt.Errorf("fake: %s returned exit code %d", fs.cmd, fs.exitCode)
}

type writeNopCloser struct {
	io.Writer
}

func (wc writeNopCloser) Close() error {
	return nil
}

func TestRun(t *testing.T) {
	var tests = []struct {
		cmd     string
		wantErr bool
	}{
		{"exit 0", false},
		{"exit 1", true},
	}

	implementations := map[string]Commander{
		"local": NewLocal(Config{Logger: t.Logf}),
		"ssh":   NewSSH(Config{Logger: t.Logf}),
	}

	for rname, runner := range implementations {
		for _, tc := range tests {
			t.Run(rname+"_"+tc.cmd, func(t *testing.T) {
				got := runner.Run(tc.cmd)
				if got == nil && tc.wantErr {
					t.Errorf("Run(%s) = %v, wanted error", tc.cmd, got)
				}
				if got != nil && !tc.wantErr {
					t.Errorf("Run(%s) = %v, wanted no error", tc.cmd, got)
				}
			})
		}
	}
}

func TestOutput(t *testing.T) {
	var tests = []struct {
		name      string
		cmd       string
		status    int
		stdout    string
		stderr    string
		combined  string
		shouldErr bool
	}{
		{"ok", "exit 0", 0, "", "", "", false},
		{"non-zero", "exit 2", 2, "", "", "", true},
		{"stdout", "echo o", 0, "o\n", "", "o\n", false},
		{"stderr", "echo e 1>&2", 0, "", "e\n", "e\n", false},
		{"out-err", "echo o; echo e 1>&2", 0, "o\n", "e\n", "o\ne\n", false},
		{"err-out", "echo e 1>&2; echo o", 0, "o\n", "e\n", "e\no\n", false},
		{"err-exit", "echo err 1>&2; exit 1", 1, "", "err\n", "err\n", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := NewShell(Config{})
			res, err := l.Output(tc.cmd)
			if err == nil && tc.shouldErr {
				t.Errorf("error = %v, want error", err)
			}
			if res.ExitCode != tc.status {
				t.Errorf("ExitCode = %d, want %d", res.ExitCode, tc.status)
			}
			if !cmp.Equal(string(res.Stdout), tc.stdout) {
				t.Errorf("Stdout = %q, want %q", res.Stdout, tc.stdout)
			}
			if !cmp.Equal(string(res.Stderr), tc.stderr) {
				t.Errorf("Stderr = %q, want %q", res.Stderr, tc.stderr)
			}
			if !cmp.Equal(string(res.Combined), tc.combined) {
				t.Errorf("Combined = %q, want %q", res.Combined, tc.combined)
			}
		})
	}
}

func TestStream(t *testing.T) {
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
			l := NewShell(Config{})
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
