package util

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"google/shlex"

	"github.com/docker/machine/libmachine/state"
	"k8s.io/minikube/pkg/minikube/assets"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/test/integration/util"
)

type Config struct {
	BinaryPath string
	Args       string
	StartArgs  string
	LogArgs    string
	Runtime    string
	VMDriver   string
}

type MinikubeRunner struct {
	c Config
}

func NewMinikubeRunner(c Config) (util.MinikubeRunner, error) {
	path, err := filepath.Abs(m.BinaryPath)
	if err != nil {
		return err
	}

	c.BinaryPath = path
	if c.LogArgs == "" {
		c.LogArgs = "--alsologtostderr -v 8"
	}
	return util.MinikubeRunner{c: c}
}

func (m *MinikubeRunner) Copy(f assets.CopyableFile) error {
	path, _ := filepath.Abs(m.BinaryPath)
	cmd := exec.Command("/bin/bash", "-c", path, "ssh", "--", fmt.Sprintf("cat >> %s", filepath.Join(f.GetTargetDir(), f.GetTargetName())))
	return cmd.Run()
}

// RunWithContext calls the minikube command with a context, useful for timeouts.
func (m *MinikubeRunner) Run(ctx context.Context, command string) ([]byte, []byte, error) {
	args, err := shlex.Split(command)
	if err != nil {
		return []byte{}, []byte{}, err
	}
	path, _ := filepath.Abs(m.BinaryPath)
	cmd := exec.Command(path, commandArr...)
	stdout, err := cmd.Output()

	if checkError && err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			m.T.Fatalf("Error running command: %s %s. Output: %s", command, exitError.Stderr, stdout)
		} else {
			m.T.Fatalf("Error running command: %s %v. Output: %s", command, err, stdout)
		}
	}
	return string(stdout)

	commandArr := strings.Split(command, " ")
	path, _ := filepath.Abs(m.BinaryPath)
	return exec.CommandContext(ctx, path, commandArr...).CombinedOutput()
}

// RunWithContext calls the minikube command with a context, useful for timeouts.
func (m *MinikubeRunner) MustRun(ctx context.Context, command string) ([]byte, error) {
	//	commandArr := strings.Split(command, " ")
	//	path, _ := filepath.Abs(m.BinaryPath)
	//	return exec.CommandContext(ctx, path, commandArr...).CombinedOutput()

	// t.Fatalf(util.ErrMsg(ctx, "start", err, Logs{stdout: stdout, stderr: stderr, minikube: mk)})
}

func (m *MinikubeRunner) StreamOutput(ctx context.Context, command string) (*exec.Cmd, *bufio.Reader) {
	commandArr := strings.Split(command, " ")
	path, _ := filepath.Abs(m.BinaryPath)
	cmd := exec.Command(path, commandArr...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		m.T.Fatalf("stdout pipe failed: %s %v", command, err)
	}

	err = cmd.Start()
	if err != nil {
		m.T.Fatalf("Error running command: %s %v", command, err)
	}
	return cmd, bufio.NewReader(stdoutPipe)
}

// StartArgs returns the appropriate start arguments for the configured environment.
func (m *MinikubeRunner) StartArgs() string {
	if m.Runtime == constants.ContainerdRuntime {
		cflags := "--container-runtime=containerd --network-plugin=cni --docker-opt containerd=/var/run/containerd/containerd.sock"
		return fmt.Sprintf("start %s %s %s", m.StartArgs, m.Args, cflags)
	}
	return fmt.Sprintf("start %s %s", m.StartArgs, m.Args)
}

func (m *MinikubeRunner) Status() string {
	return m.RunCommand(fmt.Sprintf("status --format={{.MinikubeStatus}} %s", m.Args), false)
}

func (m *MinikubeRunner) MustBeInState(want state.State) bool {
	got := Status()
	if got != want.String() {
		return fmt.Errorf("state=%q, want: %q", st, want)
	}
}

func (m *MinikubeRunner) WaitForState(want state.State) bool {
	checkStop := func() error {
		mk.MustBeInState(want)
	}
	util.MustRetry(t, ctx, checkStop, 5*time.Second, 6)
}
