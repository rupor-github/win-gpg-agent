package pinentry

import (
	"fmt"
	"time"

	"win-gpg-agent/assuan/common"
)

type Options struct {
	Grab                bool
	AllowExtPasswdCache bool
	Display             string
	TTYType             string
	TTYName             string
	TTYAlert            string
	LCCtype             string
	LCMessages          string
	Owner               string
	TouchFile           string
	ParentWID           string
	InvisibleChar       string
}

func (o *Options) String() string {
	return fmt.Sprintf(`{
  Grab:                [%t],
  AllowExtPasswdCache: [%t],
  Display:             [%s],
  TTYType:             [%s],
  TTYName:             [%s],
  TTYAlert:            [%s],
  LCCtype:             [%s],
  LCMessages:          [%s],
  Owner:               [%s],
  TouchFile:           [%s],
  ParentWID:           [%s],
  InvisibleChar:       [%s]
}`,
		o.Grab,
		o.AllowExtPasswdCache,
		o.Display,
		o.TTYType,
		o.TTYName,
		o.TTYAlert,
		o.LCCtype,
		o.LCMessages,
		o.Owner,
		o.TouchFile,
		o.ParentWID,
		o.InvisibleChar,
	)
}

// Settings struct contains options for pinentry prompt.
type Settings struct {
	// Some commands now allow argument passing
	CmdArgs string
	// Detailed description of request.
	Desc string
	// Text right before textbox.
	Prompt string
	// Error to show. Reset after GetPin.
	Error string
	// Text on OK button.
	OkBtn string
	// Text on NOT OK button.
	// Broken in GnuPG's pinentry (2.2.5).
	NotOkBtn string
	// Text on CANCEL button.
	CancelBtn string
	// Window title.
	Title string
	// Prompt timeout. Any user interaction disables timeout.
	Timeout time.Duration
	// Text right before repeat textbox.
	// Repeat textbox is hidden after GetPin.
	RepeatPrompt string
	// Error text to be shown if passwords do not match.
	RepeatError string
	// Text before password quality bar.
	QualityBar, QualityBarToolTip string
	// label and tooltip to be used for a generate action.
	GenPINLabel, GenPINToolTip string
	// To identify a key for caching - empty string mean that the key does not have a stable identifier.
	KeyInfo string
	// Password quality callback.
	PasswordQuality func(string) int

	Opts Options

	// For getInfo
	Ver string
}

func (s *Settings) String() string {
	return fmt.Sprintf(`{
  Args:         [%s],
  Desc:         [%s],
  Prompt:       [%s],
  Error:        [%s],
  OkBtn:        [%s],
  NotOkBtn:     [%s],
  CancelBtn:    [%s],
  Title:        [%s],
  Timeout:      [%s],
  RepeatPrompt: [%s],
  RepeatError:  [%s],
  QualityBar:   [%s],
  QualityBarTT: [%s],
  GenPINLabel:  [%s],
  GenPINTT:     [%s],
  KeyInfo:      [%s],
  Opts:
%s
}`,
		s.CmdArgs,
		common.EscapeParameters(s.Desc),
		s.Prompt,
		s.Error,
		s.OkBtn,
		s.NotOkBtn,
		s.CancelBtn,
		s.Title,
		s.Timeout,
		s.RepeatPrompt, s.RepeatError,
		s.QualityBar, s.QualityBarToolTip,
		s.GenPINLabel, s.GenPINToolTip,
		s.KeyInfo,
		s.Opts.String(),
	)
}
