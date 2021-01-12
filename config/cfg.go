// Package config abstracts all program configuration.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	ucfg "go.uber.org/config"

	"github.com/rupor-github/win-gpg-agent/util"
)

// GPGConfig structs wraps configuration values for GnuPG.
type GPGConfig struct {
	Path   string   `yaml:"install_path,omitempty"`
	Home   string   `yaml:"homedir,omitempty"`
	Config string   `yaml:"gpg_agent_conf,omitempty"`
	Args   []string `yaml:"gpg_agent_args,omitempty"`
}

var defaultGPGConfig = `
gpg:
  install_path: "${ProgramFiles(x86)}\\gnupg"
  homedir: "${APPDATA}\\gnupg"
`

// CLPConfig wraps configuration values for gclpr.
type CLPConfig struct {
	Port int      `yaml:"port,omitempty"`
	LE   string   `yaml:"line_endings,omitempty"`
	Keys []string `yaml:"public_keys,omitempty"`
}

// GUIConfig wraps configuration values for agent-gui, pinentry and sorelay.
type GUIConfig struct {
	Debug             bool            `yaml:"debug,omitempty"`
	SetEnv            bool            `yaml:"setenv,omitempty"`
	IgnoreSessionLock bool            `yaml:"ignore_session_lock,omitempty"`
	PipeName          string          `yaml:"pipe_name,omitempty"`
	Home              string          `yaml:"homedir,omitempty"`
	Deadline          time.Duration   `yaml:"deadline,omitempty"`
	PinDlg            util.DlgDetails `yaml:"pin_dialog,omitempty"`
	Clp               CLPConfig       `yaml:"gclpr,omitempty"`
}

var defaultGUIConfig = `
gui:
  debug: false
  setenv: true
  ignore_session_lock: false
  deadline: 1m
  pipe_name: %s
  homedir: "${LOCALAPPDATA}\\gnupg"
  gclpr:
    port: 2850
  pin_dialog:
    delay: 300ms
    name: Windows Security
    class: Credential Dialog Xaml Host
`

// Config keeps all configuration values.
type Config struct {
	GUI GUIConfig
	GPG GPGConfig
}

// Load prepares configuration structures using all available sources.
func Load(fnames ...string) (*Config, error) {

	configSources := []ucfg.YAMLOption{
		ucfg.Expand(os.LookupEnv),
		ucfg.Source(strings.NewReader(fmt.Sprintf(defaultGUIConfig, util.SSHAgentPipeName))),
		ucfg.Source(strings.NewReader(defaultGPGConfig)),
	}
	for _, fname := range fnames {
		if len(fname) != 0 && util.FileExists(fname) {
			configSources = append(configSources, ucfg.File(fname))
		}
	}
	provider, err := ucfg.NewYAML(configSources...)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := provider.Get("gui").Populate(&cfg.GUI); err != nil {
		return nil, err
	}
	if err := provider.Get("gpg").Populate(&cfg.GPG); err != nil {
		return nil, err
	}
	return &cfg, nil
}
