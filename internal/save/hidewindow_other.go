//go:build !windows

package save

import "os/exec"

func hideWindow(*exec.Cmd) {}
