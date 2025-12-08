//go:build !linux

package blinkdiskrunner

import "os/exec"

func setpdeath(c *exec.Cmd) *exec.Cmd {
	return c
}
