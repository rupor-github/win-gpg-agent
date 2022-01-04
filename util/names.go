package util

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// Shared names.
const (
	SSHAgentPipeName = "\\\\.\\pipe\\openssh-ssh-agent"
	MaxNameLen       = windows.UNIX_PATH_MAX

	// openssh-portable has it at 256 * 1024.
	// gpg-agent is using 16 * 1024.
	// Putty seems to have it at 8 + 1024.
	MaxAgentMsgLen = 256 * 1024

	GPGAgentName             = "gpg-agent"
	WinAgentName             = "agent-gui"
	SocketAgentName          = "S." + GPGAgentName
	SocketAgentBrowserName   = "S." + GPGAgentName + ".browser"
	SocketAgentExtraName     = "S." + GPGAgentName + ".extra"
	SocketAgentSSHName       = "S." + GPGAgentName + ".ssh"
	SocketAgentSSHCygwinName = "S." + GPGAgentName + ".ssh.cyg"
)

// PrepareWindowsPath prepares Windows path for use on unix shell line without quoting.
func PrepareWindowsPath(path string) string {
	return filepath.ToSlash(path)
}

// FileExists check if file exists.
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.Mode().IsRegular()
}

// WaitForFileArrival checks for files existence once a second for requested waiting period.
func WaitForFileArrival(period time.Duration, filenames ...string) bool {

	l := len(filenames)
	if l == 0 {
		return true
	}

	fns := make([]string, l)
	copy(fns, filenames)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	tCur := time.Now()
	tEnd := tCur.Add(period)
	for ; tCur.Before(tEnd); tCur = <-ticker.C {
		for i := 0; i < len(fns); i++ {
			if len(fns[i]) == 0 {
				continue
			}
			if FileExists(fns[i]) {
				fns[i] = ""
				l--
			}
		}
		if l == 0 {
			return true
		}
	}
	return false
}

// WaitForFileDeparture attempts to remove files once a second for requested waiting period.
func WaitForFileDeparture(period time.Duration, filenames ...string) {

	l := len(filenames)
	if l == 0 {
		return
	}

	fns := make([]string, l)
	copy(fns, filenames)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	tCur := time.Now()
	tEnd := tCur.Add(period)
	for ; tCur.Before(tEnd) && l > 0; tCur = <-ticker.C {
		for i := 0; i < len(fns); i++ {
			if len(fns[i]) == 0 {
				continue
			}
			if !FileExists(fns[i]) {
				fns[i] = ""
				l--
				continue
			}
			if err := os.Remove(fns[i]); err == nil {
				fns[i] = ""
				l--
			} else {
				log.Printf("Departing file removal problem: %s", err.Error())
			}
		}
	}
}

// IsNetClosing exists because ErrNetClosing is not exported. This is probably going to change in 1.16.
func IsNetClosing(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}
