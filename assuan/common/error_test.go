package common_test

import (
	"testing"

	"win-gpg-agent/assuan/common"
)

func TestSplitErrCode(t *testing.T) {
	src, code := common.SplitErrCode(536871187)
	if src != common.ErrSrcUser1 {
		t.Errorf("Error source mismatch: wanted %d, got %d", common.ErrSrcUser1, src)
	}
	if code != common.ErrAssUnknownCmd {
		t.Errorf("Error code mismatch: wanted %d, got %d", common.ErrUnknownCommand, code)
	}
}

func TestMakeErrCode(t *testing.T) {
	combinedCode := common.MakeErrCode(common.ErrSrcUser1, common.ErrAssUnknownCmd)
	if combinedCode != 536871187 {
		t.Errorf("Combined error code mismatch: wanted %d, got %d", 536871187, combinedCode)
	}
}

func TestDecodeErrCmd(t *testing.T) {
	errI := common.DecodeErrCmd("536871187 Unknown IPC command <User defined source 1>")
	err, ok := errI.(common.Error)
	if !ok {
		t.Error("Non-common.Error error returned:", err)
		t.FailNow()
	}

	if err.Src != common.ErrSrcUser1 {
		t.Errorf("Error source mismatch: wanted %d, got %d", common.ErrSrcUser1, err.Src)
	}
	if err.Code != common.ErrAssUnknownCmd {
		t.Errorf("Error code mismatch: wanted %d, got %d", common.ErrAssUnknownCmd, err.Code)
	}
	if err.Message != "Unknown IPC command" {
		t.Errorf("Error message mismatch: wanted '%s', got '%s'", "Unknown IPC command", err.Message)
	}
}
