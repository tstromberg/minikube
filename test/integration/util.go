package integration

// These are utility functions for integration tests:
//
// - Do not accept *testing.T arguments (see helpers.go)
// - Do not directly read flags (see main.go)
// - Are used in multiple tests
// - Must not compare test values

import (
	"bufio"
	"fmt"
	"time"
)

// ReadLineWithTimeout reads a line of text from a buffer with a timeout
func ReadLineWithTimeout(b *bufio.Reader, timeout time.Duration) (string, error) {
	s := make(chan string)
	e := make(chan error)
	go func() {
		read, err := b.ReadString('\n')
		if err != nil {
			e <- err
		} else {
			s <- read
		}
		close(s)
		close(e)
	}()

	select {
	case line := <-s:
		return line, nil
	case err := <-e:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout after %s", timeout)
	}
}
