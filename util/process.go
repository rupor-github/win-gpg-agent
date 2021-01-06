package util

import (
	"os"
	"strings"

	"github.com/mitchellh/go-ps"
)

// KillRunningAgent uses Os functions to terminate gpg-agent ungracefully.
func KillRunningAgent() error {
	processes, err := ps.Processes()
	if err != nil {
		return err
	}
	for _, p := range processes {
		if !strings.EqualFold(p.Executable(), GPGAgentName+".exe") {
			continue
		}
		if proc, err := os.FindProcess(p.Pid()); err != nil {
			return err
		} else if err = proc.Kill(); err != nil {
			return err
		}
		break
	}
	return nil
}
