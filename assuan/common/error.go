package common

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

const (
	errSrcMask  = 127
	errSrcShift = 24
	errCodeMask = 65535
)

// ErrorCode - error code as defined by Assuan protocol.
type ErrorCode uint16

// ErrorSource - error source as defined by Assuan protocol.
type ErrorSource uint8

// Error is used to present errors returned by server.
type Error struct {
	Src     ErrorSource
	Code    ErrorCode
	SrcName string
	Message string
}

func (e Error) Error() string {
	return e.SrcName + ": " + e.Message
}

// WriteError converts arbitrary error object to protocol error with Assuan Write Error code.
func WriteError(err error) *Error {
	return &Error{
		Src: ErrSrcAssuan, Code: ErrAssWriteError,
		SrcName: "assuan", Message: err.Error(),
	}
}

// WriteError converts arbitrary error object to protocol error with Assuan Read Error code.
func ReadError(err error) *Error {
	return &Error{
		Src: ErrSrcAssuan, Code: ErrAssReadError,
		SrcName: "assuan", Message: err.Error(),
	}
}

var errParamsRegex = regexp.MustCompile(`^(\d{1,10}) ([\w ]+)(?:<([\w ]+)>)?$`)

func mapSource(src string) string {
	// Used for protocol-level errors
	if strings.ToLower(src) == "user defined source 1" {
		return "Assuan"
	}
	return src
}

func DecodeErrCmd(params string) error {
	// Errors are presented in following format:
	//  ERR CODE      Description         <Source name>
	//  ERR 536871187 Unknown IPC command <User defined source 1>
	//
	// Where CODE consists of source code and error code and few reserved bits:
	//  1000000 0000000 0000000100010011
	//  SOURCE  RESRVD  CODE

	// Parse parameters string.
	groups := errParamsRegex.FindStringSubmatch(params)
	if groups == nil {
		return errors.New("malformed ERR arguments")
	}
	codeStr, desc := groups[1], strings.TrimSpace(groups[2])
	src := "unknown source"
	if len(groups) == 4 {
		src = mapSource(groups[3])
	}
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		return errors.New("malformed ERR arguments (code)")
	}

	srcCode, errCode := SplitErrCode(code)

	return Error{Src: srcCode, Code: errCode, SrcName: src, Message: desc}
}

func SplitErrCode(code int) (ErrorSource, ErrorCode) {
	return ErrorSource(code >> errSrcShift), ErrorCode(code & errCodeMask)
}

// MakeErrCode converts (source, code) pair to format used by Assuan.
func MakeErrCode(source ErrorSource, code ErrorCode) int {
	return int(source)&errSrcMask<<errSrcShift | int(code)&errCodeMask
}
