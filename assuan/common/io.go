package common

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
)

const (
	// MaxLineLen is a maximum length of line in Assuan protocol, including
	// space after command and LF.
	MaxLineLen = 1000
)

// ReadWriter ties arbitrary io.Reader and io.Writer to get a struct that
// satisfies io.ReadWriter requirements.
type ReadWriter struct {
	io.Reader
	io.Writer
}

// Pipe is a wrapper for Assuan command stream.
type Pipe struct {
	scnr *bufio.Scanner
	r    io.Reader
	w    io.Writer
}

// New crreates and initializes Pipe using biderectional stream.
func New(stream io.ReadWriter) Pipe {
	p := Pipe{bufio.NewScanner(stream), stream, stream}
	p.scnr.Buffer(make([]byte, 0, MaxLineLen), MaxLineLen)
	return p
}

// NewPipe crreates and initializes Pipe using 2 streams.
func NewPipe(in io.Reader, out io.Writer) Pipe {
	p := Pipe{bufio.NewScanner(in), in, out}
	p.scnr.Buffer(make([]byte, 0, MaxLineLen), MaxLineLen)
	return p
}

// Close closes Pipe.
func (p *Pipe) Close() error {
	// Reserved for future use, no-op now.
	return nil
}

// RestrictInputLen controls how lines longer than MaxLineLen should be handled.
// By default they will be discarded and error will be returned. You can disable
// this behavior using RestrictInputLen(false) if implementation you are working
// with violates this restriction.
//
// Note that even with b=false line length will be restricted to
// bufio.MaxScanTokenSize (64 KiB).
//
// This function MUST be called before any I/O, otherwise it will panic.
func (p *Pipe) RestrictInputLen(restrict bool) {
	if restrict {
		p.scnr.Buffer(make([]byte, 0, MaxLineLen), MaxLineLen)
	} else {
		p.scnr.Buffer([]byte{}, bufio.MaxScanTokenSize)
	}
}

// ReadLine reads raw request/response in following format: command <parameters>
//
// Empty lines and lines starting with # are ignored as specified by protocol.
// Additionally, status information is silently discarded for now.
func (p *Pipe) ReadLine() (cmd string, params string, err error) {
	var line string
	for {
		if ok := p.scnr.Scan(); !ok {
			err := p.scnr.Err()
			if err == nil {
				err = io.EOF
			}
			return "", "", err
		}
		line = p.scnr.Text()

		// We got something that looks like a message. Let's parse it.
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "S ") && len(strings.TrimSpace(line)) != 0 {
			break
		}
	}

	// Part before first whitespace is a command. Everything after first whitespace is parameters.
	parts := strings.SplitN(line, " ", 2)

	// If there is no parameters... (huh!?)
	if len(parts) == 1 {
		return strings.ToUpper(parts[0]), "", nil
	}

	log.Println("<", parts[0])

	params, err = UnescapeParameters(parts[1])
	if err != nil {
		return "", "", err
	}

	// Command is "normalized" to upper case since peer can send
	// commands in any case.
	return strings.ToUpper(parts[0]), params, nil
}

// WriteLine writes request/response to pipe.
// Contents of params is escaped according to requirements of Assuan protocol.
func (p *Pipe) WriteLine(cmd string, params string) error {
	if len(cmd)+len(params)+2 > MaxLineLen {
		log.Println("Refusing to send - command too long")
		// 2 is for whitespace after command and LF
		return errors.New("command or parameters are too log")
	}

	log.Println(">", cmd)

	var line []byte
	if params != "" {
		line = []byte(strings.ToUpper(cmd) + " " + EscapeParameters(params) + "\n")
	} else {
		line = []byte(strings.ToUpper(cmd) + "\n")
	}
	_, err := p.w.Write(line)
	return err
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

// WriteData sends passed byte slice using one or more D commands.
// Note: Error may occur even after some data is written so it's better
// to just CAN transaction after WriteData error.
func (p *Pipe) WriteData(input []byte) error {
	encoded := []byte(EscapeParameters(string(input)))
	chunkLen := MaxLineLen - 3 // 3 is for 'D ' and line feed.
	for i := 0; i < len(encoded); i += chunkLen {
		chunk := encoded[i:min(i+chunkLen, len(encoded))]
		chunk = append([]byte{'D', ' '}, chunk...)
		chunk = append(chunk, '\n')

		if _, err := p.w.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

// WriteDataReader is similar to WriteData but sends data from input Reader
// until EOF.
func (p *Pipe) WriteDataReader(input io.Reader) error {
	chunkLen := MaxLineLen - 3 // 3 is for 'D ' and line feed.
	buf := make([]byte, chunkLen)

	for {
		n, err := input.Read(buf)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		chunk := []byte(EscapeParameters(string(buf[:n])))
		chunk = append([]byte{'D', ' '}, chunk...)
		chunk = append(chunk, '\n')
		if _, err := p.w.Write(chunk); err != nil {
			return err
		}
	}
}

// ReadData reads sequence of D commands and joins data together.
func (p *Pipe) ReadData() (data []byte, err error) {
	for {
		cmd, chunk, err := p.ReadLine()
		if err != nil {
			return nil, err
		}

		if cmd == "END" {
			return data, nil
		}

		if cmd == "CAN" {
			return nil, Error{Src: ErrSrcAssuan, Code: ErrUnexpected, SrcName: "assuan", Message: "IPC call has been cancelled"}
		}

		if cmd != "D" {
			return nil, Error{Src: ErrSrcAssuan, Code: ErrUnexpected, SrcName: "assuan", Message: "unexpected IPC command"}
		}

		unescaped, err := UnescapeParameters(chunk)
		if err != nil {
			return nil, err
		}

		data = append(data, []byte(unescaped)...)
	}
}

// WriteComment is special case of WriteLine. "Command" is # and text is parameter.
func (p *Pipe) WriteComment(text string) error {
	return p.WriteLine("#", text)
}

// WriteError is a special case of WriteLine. It writes command.
func (p *Pipe) WriteError(err Error) error {
	return p.WriteLine("ERR", fmt.Sprintf("%d %s <%s>", MakeErrCode(err.Src, err.Code), err.Message, err.SrcName))
}
