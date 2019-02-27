package rexec

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
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
