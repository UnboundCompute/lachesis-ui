//go:build !(aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris)

package mcp

import "os/exec"

// Non-POSIX runners do not expose a portable process-group API here. The parent
// process is still bounded and killed, matching the pre-group behavior.
func configureProcess(_ *exec.Cmd) {}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
