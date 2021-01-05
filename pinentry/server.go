package pinentry

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"github.com/rupor-github/win-gpg-agent/assuan/common"
	"github.com/rupor-github/win-gpg-agent/assuan/server"
	"github.com/rupor-github/win-gpg-agent/wincred"
)

var version = "undefined"
var DefaultSettings Settings

func CredentialName(key string) string {
	return "GnuPG:PinGO=" + key
}

type Callbacks struct {
	GetPIN  func(*common.Pipe, *Settings) (string, *common.Error)
	Confirm func(*common.Pipe, *Settings) (bool, *common.Error)
	Msg     func(*common.Pipe, *Settings) *common.Error
}

func setDesc(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).Desc = params
	return nil
}
func setPrompt(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).Prompt = params
	return nil
}
func setRepeat(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).RepeatPrompt = params
	return nil
}
func setRepeatError(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).RepeatError = params
	return nil
}
func setError(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).Error = params
	return nil
}
func setOk(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).OkBtn = params
	return nil
}
func setNotOk(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).NotOkBtn = params
	return nil
}
func setCancel(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).CancelBtn = params
	return nil
}
func setQualityBar(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).QualityBar = params
	return nil
}
func setQualityBarToolTip(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).QualityBarToolTip = params
	return nil
}
func setGenPINLabel(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).GenPINLabel = params
	return nil
}
func setGenPINToolTip(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).GenPINToolTip = params
	return nil
}
func setTitle(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).Title = params
	return nil
}
func setTimeout(_ *common.Pipe, state interface{}, params string) error {
	i, err := strconv.Atoi(params)
	if err != nil {
		return &common.Error{
			Src: common.ErrSrcPinentry, Code: common.ErrAssInvValue,
			SrcName: "pinentry", Message: "invalid timeout value",
		}
	}
	state.(*Settings).Timeout = time.Duration(i) * time.Second
	return nil
}

func clearPassphrase(_ *common.Pipe, state interface{}, params string) error {
	key := strings.Trim(params, " ")
	cred, err := wincred.GetGenericCredential(CredentialName(key))
	if err != nil && !errors.Is(err, windows.ERROR_NOT_FOUND) {
		log.Printf("GetGenericCredential cannot access vault: %s", err.Error())
		return &common.Error{
			Src: common.ErrSrcPinentry, Code: common.ErrAssGeneral,
			SrcName: "pinentry", Message: "CLEARPASSPHRASE cannot access vault",
		}
	}
	if cred != nil {
		if err := cred.Delete(); err != nil {
			return &common.Error{
				Src: common.ErrSrcPinentry, Code: common.ErrAssInvValue,
				SrcName: "pinentry", Message: "CLEARPASSPHRASE cannot delete credential",
			}
		}
	}
	return nil
}

func getInfo(pipe *common.Pipe, state interface{}, params string) error {
	var res string
	switch strings.Trim(params, " ") {
	case "flavor":
		res = "PinGO (w32)"
	case "version":
		res = version
	case "pid":
		// Since gnupg_allow_set_foregound_window() dioes not know what to do with proper process id - inhibit invalid argument error
		res = "-1"
	case "ttyinfo":
		res = "- - -"
	default:
		return &common.Error{
			Src: common.ErrSrcPinentry, Code: common.ErrAssParameter,
			SrcName: "pinentry", Message: fmt.Sprintf("GETINFO unknown parameter value: %s", params),
		}
	}
	if len(res) != 0 {
		if err := pipe.WriteData([]byte(res)); err != nil {
			log.Println("... IO error, dropping session:", err)
			return err
		}
	}
	return nil
}

func setKeyInfo(_ *common.Pipe, state interface{}, params string) error {
	if len(params) == 0 || params == "--clear" {
		state.(*Settings).KeyInfo = ""
	} else {
		state.(*Settings).KeyInfo = params
	}
	return nil
}

func resetState(_ *common.Pipe, state interface{}, _ string) error {
	*(state.(*Settings)) = DefaultSettings
	return nil
}

func setOpt(state interface{}, key string, val string) error {
	opts := state.(*Settings)

	if key == "no-grab" {
		opts.Opts.Grab = false
		return nil
	}
	if key == "grab" {
		opts.Opts.Grab = true
		return nil
	}
	if key == "ttytype" {
		opts.Opts.TTYType = val
		return nil
	}
	if key == "ttyname" {
		opts.Opts.TTYName = val
		return nil
	}
	if key == "ttyalert" {
		opts.Opts.TTYAlert = val
		return nil
	}
	if key == "lc-ctype" {
		opts.Opts.LCCtype = val
		return nil
	}
	if key == "lc-messages" {
		opts.Opts.LCMessages = val
		return nil
	}
	if key == "owner" {
		opts.Opts.Owner = val
		return nil
	}
	if key == "touch-file" {
		opts.Opts.TouchFile = val
		return nil
	}
	if key == "parent-wid" {
		opts.Opts.ParentWID = val
		return nil
	}
	if key == "invisible-char" {
		opts.Opts.InvisibleChar = val
		return nil
	}
	if key == "allow-external-password-cache" {
		opts.Opts.AllowExtPasswdCache = true
		return nil
	}

	if strings.HasPrefix(key, "default-") {
		return nil
	}

	return &common.Error{
		Src: common.ErrSrcPinentry, Code: common.ErrUnknownOption,
		SrcName: "pinentry", Message: "unknown option: " + key,
	}
}

var Info = server.ProtoInfo{
	Greeting: "PinGO (w32)",
	Handlers: map[string]server.CommandHandler{
		"SETDESC":          setDesc,
		"SETPROMPT":        setPrompt,
		"SETREPEAT":        setRepeat,
		"SETREPEATERROR":   setRepeatError,
		"SETERROR":         setError,
		"SETOK":            setOk,
		"SETNOTOK":         setNotOk,
		"SETCANCEL":        setCancel,
		"SETQUALITYBAR":    setQualityBar,
		"SETQUALITYBAR_TT": setQualityBarToolTip,
		"SETGENPIN":        setGenPINLabel,
		"SETGENPIN_TT":     setGenPINToolTip,
		"SETTITLE":         setTitle,
		"SETTIMEOUT":       setTimeout,
		"CLEARPASSPHRASE":  clearPassphrase,
		"GETINFO":          getInfo,
		"SETKEYINFO":       setKeyInfo,
		"RESET":            resetState,
	},
	Help: map[string][]string{}, // TODO
	GetDefaultState: func() interface{} {
		var s = DefaultSettings
		return &s
	},
	SetOption: setOpt,
}

func Serve(callbacks Callbacks, ver string) error {
	info := Info

	if len(version) != 0 {
		version = ver
	}

	info.Handlers["GETPIN"] = func(pipe *common.Pipe, state interface{}, params string) error {
		if callbacks.GetPIN == nil {
			log.Println("GETPIN requested but not supported")
			return &common.Error{
				Src: common.ErrSrcPinentry, Code: common.ErrNotImplemented,
				SrcName: "pinentry", Message: "GETPIN op is not supported",
			}
		}

		state.(*Settings).CmdArgs = params
		log.Printf("GETPIN state:\n%s", state.(*Settings).String())
		pass, err := callbacks.GetPIN(pipe, state.(*Settings))
		if err != nil {
			return err
		}

		if err := pipe.WriteData([]byte(pass)); err != nil {
			return nil
		}
		return nil
	}
	info.Handlers["CONFIRM"] = func(pipe *common.Pipe, state interface{}, params string) error {
		if callbacks.Confirm == nil {
			log.Println("CONFIRM requested but not supported")
			return &common.Error{
				Src: common.ErrSrcPinentry, Code: common.ErrNotImplemented,
				SrcName: "pinentry", Message: "CONFIRM op is not supported",
			}
		}

		state.(*Settings).CmdArgs = params
		log.Printf("CONFIRM state:\n%s", state.(*Settings).String())
		v, err := callbacks.Confirm(pipe, state.(*Settings))
		if err != nil {
			return err
		}

		if !v {
			return &common.Error{
				Src: common.ErrSrcPinentry, Code: common.ErrCanceled,
				SrcName: "pinentry", Message: "operation canceled",
			}
		}
		return nil
	}
	info.Handlers["MESSAGE"] = func(pipe *common.Pipe, state interface{}, params string) error {
		if callbacks.Msg == nil {
			log.Println("MESSAGE requested but not supported")
			return &common.Error{
				Src: common.ErrSrcPinentry, Code: common.ErrNotImplemented,
				SrcName: "pinentry", Message: "MESSAGE op is not supported",
			}
		}
		state.(*Settings).CmdArgs = params
		log.Printf("MESSAGE state:\n%s", state.(*Settings).String())
		return callbacks.Msg(pipe, state.(*Settings))
	}

	err := server.ServeStdin(info)
	return err
}
