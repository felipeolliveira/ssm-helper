//go:build !windows

package tunnel

import "os/exec"

func configureCommand(*exec.Cmd) {}

func attachToJob(*exec.Cmd) {}
