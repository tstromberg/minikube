package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"

	commonutil "k8s.io/minikube/pkg/util"
)

const kubectlBinary = "kubectl"

type KubectlRunner struct {
	BinaryPath string
}

func NewKubectlRunner(t *testing.T) *KubectlRunner {
	p, err := exec.LookPath(kubectlBinary)
	if err != nil {
		t.Fatalf("Couldn't find kubectl on path.")
	}
	return &KubectlRunner{BinaryPath: p}
}

func (k *KubectlRunner) RunJSON(ctx, args string, outputObj interface{}) error {
	args = append(args, "-o=json")
	output, err := k.Run(ctx, args)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(output))
	if err := d.Decode(outputObj); err != nil {
		return err
	}
	return nil
}

func (k *KubectlRunner) Run(ctx context.Context, args string) (stdout []byte, stderr []byte, err error) {
	inner := func() error {
		cmd := exec.Command(k.BinaryPath, args...)
		stdout, stderr, err = cmd.CombinedOutput()
		if err != nil {
			retriable := &commonutil.RetriableError{Err: fmt.Errorf("error running command %s: %v. Stdout: \n %s", args, err, stdout)}
			k.T.Log(retriable)
			return retriable
		}
		return nil
	}

	err = commonutil.RetryAfter(3, inner, 2*time.Second)
	return nil, stdout, err
}
