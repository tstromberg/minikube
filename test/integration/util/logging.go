package util

import (
	"context"
	"fmt"
)

type Logs struct {
	stdout   []byte
	stderr   []byte
	minikube MinikubeRunner
	kubectl  KubectlRunner
}

func ErrMsg(ctx context.Context, err error, prefix string, logs Logs) string {
	return Msg(ctx, fmt.Sprintf("%s: %v", prefix, err), logs)
}

func Msg(ctx context.Context, msg string, logs Logs) string {
	return fmt.Sprintf("%s\n", msg)
}
