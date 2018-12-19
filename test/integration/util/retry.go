package util

import (
	"context"
	"time"
)

func Retry(ctx context.Context, callback func() error, sleep time.Duration, max time.Duration) (err error) {

	// TODO: check context health here.
	for i := 0; i < attempts; i++ {
		err = callback()
		if err == nil {
			return nil
		}
		time.Sleep(d)
	}
	return err
}

func MustRetry(ctx context.Context, callback func() error, sleep time.Duration, max time.Duration) (err error) {
	err := Retry(ctx, callback, d, attempts)
	if err != nil {
		t.Fatalf("must retry: %v", err)
	}
}
