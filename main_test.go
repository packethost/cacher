package main

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

func TestNoIngest(t *testing.T) {
	t.Run("IngestEnabledFails", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "./cacher")
		cmd.Env = []string{}
		err := cmd.Run()

		assert.Error(t, err)
		eerr, ok := err.(*exec.ExitError)
		assert.True(t, ok)

		assert.Equal(t, 2, eerr.ExitCode())
	})
	t.Run("IngestDisabled", func(t *testing.T) {
		cmd := exec.Command("./cacher")
		cmd.Env = []string{
			"CACHER_NO_INGEST=true",
		}
		err := cmd.Start()
		assert.NoError(t, err)

		go func() {
			time.Sleep(200 * time.Millisecond)
			cmd.Process.Signal(syscall.SIGHUP)
		}()

		err = cmd.Wait()
		assert.Error(t, err)

		status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
		assert.True(t, ok)

		fmt.Println(cmd.ProcessState.String())

		assert.True(t, status.Signaled())
		assert.Equal(t, syscall.SIGHUP, status.Signal())
	})
}
