package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func executeCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	applySysProcAttr(cmd)
	return cmd.Start()
}

func openOSNative(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	applySysProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening path native launcher: %v\n", err)
	}
}

func executeBackgroundCmd(commandLine string) {
	if strings.TrimSpace(commandLine) == "" {
		fmt.Fprintln(os.Stdout, "Error: workspace process command is empty")
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", commandLine)
	} else {
		cmd = exec.Command("sh", "-c", commandLine)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	applySysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to execute workspace process [%s]: %v\n", commandLine, err)
	}
}
