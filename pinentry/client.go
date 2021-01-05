package pinentry

import (
	"io"
	"os/exec"
	"strconv"
	"time"

	assuan "github.com/rupor-github/win-gpg-agent/assuan/client"
	"github.com/rupor-github/win-gpg-agent/assuan/common"
)

type Client struct {
	Session *assuan.Session

	current    Settings
	qualityBar bool
}

// Launch starts pinentry binary found in directories from PATH envvar and creates
// pinentry.Client for interaction with it.
func Launch() (*Client, error) {
	cmd := exec.Command("pinentry")

	c := new(Client)
	var err error
	c.Session, err = assuan.InitCmd(cmd)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Launch starts pinentry binary specified by passed path and creates
// pinentry.Client for interaction with it.
func LaunchCustom(path string) (Client, error) {
	cmd := exec.Command(path)

	c := Client{}
	var err error
	c.Session, err = assuan.InitCmd(cmd)
	if err != nil {
		return Client{}, err
	}
	return c, nil
}

func New(stream io.ReadWriter) (Client, error) {
	c := Client{}
	var err error
	c.Session, err = assuan.Init(stream)
	if err != nil {
		return Client{}, err
	}
	return c, err
}

func (c *Client) Close() error {
	return c.Session.Close()
}

func (c *Client) Reset() error {
	return c.Session.Reset()
}

func (c *Client) SetDesc(text string) error {
	if _, err := c.Session.SimpleCmd("SETDESC", text); err != nil {
		return err
	}
	c.current.Desc = text
	return nil
}

func (c *Client) SetPrompt(text string) error {
	if _, err := c.Session.SimpleCmd("SETPROMPT", text); err != nil {
		return err
	}
	c.current.Prompt = text
	return nil
}

func (c *Client) SetError(text string) error {
	if _, err := c.Session.SimpleCmd("SETERROR", text); err != nil {
		return err
	}
	c.current.Error = text
	return nil
}

func (c *Client) SetOkBtn(text string) error {
	if _, err := c.Session.SimpleCmd("SETOK", text); err != nil {
		return err
	}
	c.current.OkBtn = text
	return nil
}

func (c *Client) SetNotOkBtn(text string) error {
	if _, err := c.Session.SimpleCmd("SETNOTOK", text); err != nil {
		return err
	}
	c.current.NotOkBtn = text
	return nil
}

func (c *Client) SetCancelBtn(text string) error {
	if _, err := c.Session.SimpleCmd("SETCANCEL", text); err != nil {
		return err
	}
	c.current.CancelBtn = text
	return nil
}

func (c *Client) SetTitle(text string) error {
	if _, err := c.Session.SimpleCmd("SETTITLE", text); err != nil {
		return err
	}
	c.current.Title = text
	return nil
}

func (c *Client) SetTimeout(timeout time.Duration) error {
	if _, err := c.Session.SimpleCmd("SETTIMEOUT", strconv.Itoa(int(timeout.Seconds()))); err != nil {
		return err
	}
	c.current.Timeout = timeout
	return nil
}

func (c *Client) SetRepeatPrompt(text string) error {
	if _, err := c.Session.SimpleCmd("SETREPEAT", text); err != nil {
		return err
	}
	c.current.RepeatPrompt = text
	return nil
}

func (c *Client) SetRepeatError(text string) error {
	if _, err := c.Session.SimpleCmd("SETREPEATERROR", text); err != nil {
		return err
	}
	c.current.RepeatError = text
	return nil
}

func (c *Client) SetQualityBar(text string) error {
	if _, err := c.Session.SimpleCmd("SETQUALITYBAR", text); err != nil {
		return err
	}
	c.current.QualityBar = text
	c.qualityBar = true
	return nil
}

func (c *Client) SetPasswdQualityCallback(callback func(string) int) {
	c.current.PasswordQuality = callback
}

func (c *Client) Current() Settings {
	return c.current
}

func (c *Client) Apply(s Settings) error {
	if err := c.SetDesc(s.Desc); err != nil {
		return err
	}
	if err := c.SetPrompt(s.Prompt); err != nil {
		return err
	}
	if err := c.SetError(s.Error); err != nil {
		return err
	}
	if err := c.SetOkBtn(s.OkBtn); err != nil {
		return err
	}
	if err := c.SetNotOkBtn(s.NotOkBtn); err != nil {
		return err
	}
	if err := c.SetCancelBtn(s.CancelBtn); err != nil {
		return err
	}
	if err := c.SetTitle(s.Title); err != nil {
		return err
	}
	if err := c.SetTimeout(s.Timeout); err != nil {
		return err
	}
	if err := c.SetRepeatPrompt(s.RepeatPrompt); err != nil {
		return err
	}
	if err := c.SetRepeatError(s.RepeatError); err != nil {
		return err
	}
	if err := c.SetQualityBar(s.QualityBar); err != nil {
		return err
	}
	c.current.PasswordQuality = s.PasswordQuality
	return nil
}

// GetPIN shows window with password textbox, Cancel and Ok buttons.
// Error is returned if Cancel is pressed.
func (c *Client) GetPIN() (string, error) {
	if c.qualityBar {
		return c.getPINWithQualBar()
	}

	dat, err := c.Session.SimpleCmd("GETPIN", "")
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

func (c *Client) getPINWithQualBar() (string, error) {
	// We will get requests in following form:
	//  INQUIRE QUALITY password-here
	// and we should respond with quality percentage,
	// otherwise pinentry will hang.
	// This is different from usual transaction so we have to use raw I/O.

	defer func() { c.qualityBar = false }()

	pipe := c.Session.Pipe
	if err := pipe.WriteLine("GETPIN", ""); err != nil {
		return "", err
	}
	for {
		cmd, params, err := pipe.ReadLine()
		if err != nil {
			return "", err
		}

		if cmd == "D" {
			// We got password.

			// Take OK from pipe.
			if _, _, err := pipe.ReadLine(); err != nil {
				return "", err
			}

			return params, nil
		}

		if cmd == "INQUIRE" {
			// params[8:] is
			//  QUALITY password-here
			//          ^~~~~~~~~~~~~
			passwd := params[8:]

			if c.current.PasswordQuality == nil {
				if err := pipe.WriteLine("D", "0"); err != nil {
					return "", err
				}
				if err := pipe.WriteLine("END", ""); err != nil {
					return "", err
				}
				continue
			}

			quality := c.current.PasswordQuality(passwd)
			if err := pipe.WriteLine("D", strconv.Itoa(quality)); err != nil {
				return "", err
			}
			if err := pipe.WriteLine("END", ""); err != nil {
				return "", err
			}
		}

		if cmd == "ERR" {
			return "", common.DecodeErrCmd(params)
		}
	}
}

// Confirm shows window with Cancel and Ok buttons but without password
// textbox, error is returned if Cancel is pressed (as usual).
func (c *Client) Confirm() error {
	_, err := c.Session.SimpleCmd("CONFIRM", "")
	return err
}

// Message just shows window with only OK button.
func (c *Client) Message() error {
	_, err := c.Session.SimpleCmd("MESSAGE", "")
	return err
}
