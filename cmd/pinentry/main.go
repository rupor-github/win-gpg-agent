package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pborman/getopt/v2"
	"golang.org/x/sys/windows"

	"github.com/rupor-github/win-gpg-agent/assuan/common"
	"github.com/rupor-github/win-gpg-agent/config"
	"github.com/rupor-github/win-gpg-agent/misc"
	"github.com/rupor-github/win-gpg-agent/pinentry"
	"github.com/rupor-github/win-gpg-agent/util"
	"github.com/rupor-github/win-gpg-agent/wincred"
)

var (
	title   = "pinentry"
	tooltip = "Pinentry program for GnuPG"
	verStr  = fmt.Sprintf("%s (%s) %s", misc.GetVersion(), runtime.Version(), misc.GetGitHash())
	// arguments
	cli         = getopt.New()
	aConfigName = title + ".conf"
	aShowHelp   bool
	aShowVer    bool
	aDebug      bool
	aNoGrab     bool
	aParent     uint64
	aTimeout    int
	// aDisplay, aTTYName, aTTYType, aLCType, aLCMessages string
)

func createCommonError(code common.ErrorCode, msg string) *common.Error {
	return &common.Error{Src: common.ErrSrcPinentry, Code: code, SrcName: "pinentry", Message: msg}
}

func sendStatus(pipe *common.Pipe, cmd string) *common.Error {
	if err := pipe.WriteLine("S", cmd); err != nil {
		log.Println("... IO error, dropping session:", err)
		return createCommonError(common.ErrAssWriteError, "unable to return status")
	}
	return nil
}

// We may need to keep some additional state between calls - pinentry state machine is old...
type callbacksState struct {
	cfg *config.Config
}

func getCachedCredential(pipe *common.Pipe, s *pinentry.Settings) (string, *common.Error) {
	cred, err := wincred.GetGenericCredential(pinentry.CredentialName(s.KeyInfo))
	if err != nil && !errors.Is(err, windows.ERROR_NOT_FOUND) {
		log.Printf("GetGenericCredential cannot access vault: %s", err.Error())
		s.Opts.AllowExtPasswdCache = false
		return "", nil
	}
	if cred == nil {
		// this should never happen, but just in case
		return "", nil
	}
	if err := sendStatus(pipe, "PASSWORD_FROM_CACHE"); err != nil {
		return "", err
	}
	return string(cred.CredentialBlob), nil
}

func addCachedCredential(name, passwd string) {
	cred := wincred.NewGenericCredential(pinentry.CredentialName(name))
	cred.CredentialBlob = []byte(passwd)
	cred.Persist = wincred.PersistLocalMachine
	if err := cred.Write(); err != nil {
		log.Printf("Unable to store credential: %s", name)
	}
}

func (cbs *callbacksState) GetPIN(pipe *common.Pipe, s *pinentry.Settings) (string, *common.Error) {

	if len(s.Error) == 0 && len(s.RepeatPrompt) == 0 && s.Opts.AllowExtPasswdCache && len(s.KeyInfo) != 0 {
		// GnuPG calls it "reading from password cache" - let's try it
		if passwd, err := getCachedCredential(pipe, s); err != nil {
			return "", err
		} else if len(passwd) > 0 {
			return passwd, nil
		}
		// we never store enmpty pasword
	}

	var (
		cancelOp, cachePasswd bool
		passwd1, passwd2      string
	)

	for attempt := 0; ; attempt++ {

		var errMsg string
		if attempt == 0 {
			if len(s.Error) > 0 {
				errMsg = s.Error
			}
		} else {
			// we are repeating - passwords did not match
			if len(s.RepeatError) > 0 {
				errMsg = s.RepeatError
			} else {
				errMsg = "Does not match - try again"
			}
		}

		cancelOp, passwd1, cachePasswd = util.PromptForWindowsCredentials(cbs.cfg.GUI.PinDlg, errMsg, s.Desc, s.Prompt, s.Opts.AllowExtPasswdCache && len(s.KeyInfo) != 0)
		if cancelOp {
			return "", createCommonError(common.ErrCanceled, "operation canceled")
		}

		if len(s.RepeatPrompt) == 0 {
			break
		}

		cancelOp, passwd2, _ = util.PromptForWindowsCredentials(cbs.cfg.GUI.PinDlg, "", s.Desc, s.RepeatPrompt, false)
		if cancelOp {
			return "", createCommonError(common.ErrCanceled, "operation canceled")
		}

		if passwd1 == passwd2 {
			if err := sendStatus(pipe, "PIN_REPEATED"); err != nil {
				return "", err
			}
			break
		}
	}

	// Everything went well - let's see if we could save password for later use.
	if s.Opts.AllowExtPasswdCache && len(s.KeyInfo) != 0 && cachePasswd && len(passwd1) > 0 {
		addCachedCredential(s.KeyInfo, passwd1)
	}
	return passwd1, nil
}

func (cbs *callbacksState) Confirm(_ *common.Pipe, s *pinentry.Settings) (bool, *common.Error) {
	return util.PromptForConfirmaion(util.DlgDetails{}, s.Desc, s.Prompt, strings.Trim(s.CmdArgs, " ") == "--one-button"), nil
}

func (cbs *callbacksState) Msg(_ *common.Pipe, s *pinentry.Settings) *common.Error {
	util.PromptForConfirmaion(util.DlgDetails{}, s.Desc, s.Prompt, true)
	return nil
}

func main() {

	// Turn it on by default to trace parameters parsing
	util.NewLogWriter(title, 0, true)

	log.Println("Starting...")

	// configuration will be picked up at the same place where executable is
	expath, err := os.Executable()
	if err == nil {
		aConfigName = filepath.Join(filepath.Dir(expath), aConfigName)
	}

	cli.SetProgram("pinentry.exe")
	cli.SetParameters("")
	cli.FlagLong(&aConfigName, "config", 'c', "Configuration file", "path")
	cli.FlagLong(&aShowVer, "version", 0, "Show version information")
	cli.FlagLong(&aShowHelp, "help", 'h', "Show help")
	cli.FlagLong(&aDebug, "debug", 'd', "Turn on debugging")
	// cli.FlagLong(&aNoGrab, "no-global-grab", 'g', "Grab the keyboard only when the window is focused")
	// cli.FlagLong(&aParent, "parent-wid", 'W', "Use window handle as the parent window for positioning the window", "HWND")
	// cli.FlagLong(&aTimeout, "timeout", 'o', "Give up waiting for input from the user after the specified number of seconds and return an error", "SECONDS")
	// cli.FlagLong(&aDisplay, "display", 'D', "console vs windows ?", "STRING")
	// cli.FlagLong(&aTTYName, "ttyname", 'T', "", "STRING")
	// cli.FlagLong(&aTTYType, "ttytype", 'N', "", "STRING")
	// cli.FlagLong(&aLCType, "lc-ctype", 'C', "", "STRING")
	// cli.FlagLong(&aLCMessages, "lc-messages", 'M', "", "STRING")

	// Silently ingnore unknown options
	if err := cli.Getopt(os.Args, nil); err != nil {
		log.Printf("Unsupported options in %+v: %s", os.Args, err.Error())
	}

	if aShowHelp {
		fmt.Fprintf(os.Stderr, "\n%s\n\n\t%s\n\n", tooltip, verStr)
		cli.PrintUsage(os.Stderr)
		os.Exit(0)
	}

	if aShowVer {
		fmt.Fprintf(os.Stderr, "\n%s\n", verStr)
		os.Exit(0)
	}

	// Read configuration
	cfg, err := config.Load(aConfigName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to load configuration from %s: %s\n", aConfigName, err.Error())
		os.Exit(1)
	}
	if aDebug {
		cfg.GUI.Debug = aDebug
	}
	util.NewLogWriter(title, 0, cfg.GUI.Debug)

	log.Println("Serving...")

	// Save default state for this run - go-assuan's simple design is prone to initialization loop, Go does not like it and workaround looks ugly.
	// It should be implemented differently rather than copying what original C does with command maps. Some day, maybe...
	pinentry.DefaultSettings.Timeout = time.Duration(aTimeout) * time.Second
	pinentry.DefaultSettings.Opts.Grab = !aNoGrab
	pinentry.DefaultSettings.Opts.ParentWID = fmt.Sprintf("0x%08X", aParent)

	cbs := &callbacksState{cfg: cfg}
	if err := pinentry.Serve(pinentry.Callbacks{GetPIN: cbs.GetPIN, Confirm: cbs.Confirm, Msg: cbs.Msg}, verStr); err != nil {
		log.Printf("Pinentry Serve returned error: %s", err.Error())
		os.Exit(1)
	}
}
