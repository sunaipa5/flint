//go:build !windows

package main

import "os/exec"

func applySysProcAttr(cmd *exec.Cmd) {
	// No-op for Linux and macOS
}
