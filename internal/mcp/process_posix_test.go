//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package mcp

import (
	"os/exec"
	"testing"
)

func TestConfigureProcessUsesDedicatedGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "true")
	configureProcess(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("engine command is not configured with a dedicated process group")
	}
}
