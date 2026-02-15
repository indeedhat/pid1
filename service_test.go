package main

import (
	"bytes"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
)

func TestNonCriticalService(t *testing.T) {
	conf := defaultP1TestConf()
	done := make(chan runResponse, 1)

	go func() {
		os.Setenv("PID1_ADITIONAL_SERVICES", "./fixtures/non-critical.icl")
		defer os.Unsetenv("PID1_ADITIONAL_SERVICES")

		code, err := run([]string{
			"-orphan-policy",
			"kill",
			"./fixtures/service_non_critical.sh",
		}, conf)

		done <- runResponse{code, err}
	}()

	select {
	case resp := <-done:
		t.Fatalf("services failed to start with code: '%d' and error: '%s'", resp.code, resp.err)
	case <-time.After(500 * time.Millisecond):
		running, err := isProcRunning("sleep 10")
		require.Nil(t, err)
		require.True(t, running, "sleep 60 is not running")
	}

	conf.sigCh <- syscall.SIGTERM

	select {
	case resp := <-done:
		require.Nil(t, resp.err)
		// indicates the process was killed by SIGTERM
		require.Equal(t, 143, resp.code)

		running, err := isProcRunning("sleep 60")
		require.Nil(t, err)
		require.False(t, running, "sleep 60 is still running")
		running, err = isProcRunning("sleep 10")
		require.Nil(t, err)
		require.False(t, running, "sleep 10 is still running")
	case <-time.After(5 * time.Second):
		t.Fatal("timout reached waiting for run to exit")
	}
}

func TestCriticalService(t *testing.T) {
	conf := defaultP1TestConf()
	done := make(chan runResponse, 1)

	go func() {
		os.Setenv("PID1_ADITIONAL_SERVICES", "./fixtures/critical.icl")
		defer os.Unsetenv("PID1_ADITIONAL_SERVICES")

		code, err := run([]string{
			"-orphan-policy",
			"kill",
			"./fixtures/service_critical.sh",
		}, conf)

		done <- runResponse{code, err}
	}()

	select {
	case resp := <-done:
		require.Nil(t, resp.err)
		require.Equal(t, 143, resp.code)

		running, err := isProcRunning("sleep 10")
		require.Nil(t, err)
		require.False(t, running, "sleep 10 is still running")
	case <-time.After(5 * time.Second):
		t.Fatal("timout reached waiting for run to exit")
	}
}

func TestCaptureServiceOutput(t *testing.T) {
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	conf := &pid1Config{
		sigCh:  make(chan os.Signal, 1),
		stdin:  bytes.NewReader(nil),
		stdout: &stdoutBuf,
		stderr: &stderrBuf,
	}

	done := make(chan runResponse, 1)

	go func() {
		os.Setenv("PID1_ADITIONAL_SERVICES", "./fixtures/capture-output.icl")
		defer os.Unsetenv("PID1_ADITIONAL_SERVICES")

		code, err := run([]string{
			"-orphan-policy",
			"adopt",
			"./fixtures/service_output.sh",
		}, conf)

		done <- runResponse{code, err}
	}()

	select {
	case resp := <-done:
		require.Nil(t, resp.err)
		require.Equal(t, 0, resp.code)

		spew.Dump(stdoutBuf.String(), stderrBuf.String())
		t.Fatal("cos")

	case <-time.After(11 * time.Second):
		t.Fatal("timout reached waiting for run to exit")
	}
}
