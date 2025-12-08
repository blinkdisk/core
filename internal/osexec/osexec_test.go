package osexec_test

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blinkdisk/core/internal/osexec"
)

func TestDisableInterruptSignal(t *testing.T) {
	c := &exec.Cmd{}

	osexec.DisableInterruptSignal(c)
	require.NotNil(t, c.SysProcAttr)
}
