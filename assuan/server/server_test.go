package server

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	"win-gpg-agent/assuan/common"
)

func TestInquire(t *testing.T) {
	sample := `D FOO
END
D BAR
END
D BAZ
END
`
	pipe := common.NewPipe(strings.NewReader(sample), ioutil.Discard)
	data, err := Inquire(&pipe, []string{"foo", "bar", "baz"})
	if err != nil {
		t.Error("Unexpected Inquire error:", err)
		t.FailNow()
	}

	if string(data["foo"]) != "FOO" {
		t.Error("Missing or incorrect data read:", data)
		t.FailNow()
	}
	if string(data["bar"]) != "BAR" {
		t.Error("Missing or incorrect data read:", data)
		t.FailNow()
	}
	if string(data["baz"]) != "BAZ" {
		t.Error("Missing or incorrect data read:", data)
		t.FailNow()
	}
}

func TestHandleCmd(t *testing.T) {
	t.Run("BYE cmd", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		if err := handleCmd(&pipe, "BYE", "", ProtoInfo{}, nil); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		if buf.String() != "OK\n" {
			t.Error("Response to BYE is not OK:", buf.String())
		}
	})
	t.Run("RESET cmd (default handler)", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		state := interface{}("foobar")

		if err := handleCmd(&pipe, "RESET", "", ProtoInfo{}, &state); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		if buf.String() != "OK\n" {
			t.Error("Response to RESET is not OK:", buf.String())
		}
	})
	t.Run("HELP cmd", helpTest)
	t.Run("OPTION cmd", optionsTest)
	t.Run("custom cmd", customCmdTest)
}

func helpTest(t *testing.T) {
	t.Run("commands list", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		if err := handleCmd(&pipe, "HELP", "", ProtoInfo{}, nil); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		for _, line := range strings.Split(buf.String(), "\n") {
			if line == "" {
				// empty lines are ok in assuan actually
				continue
			}
			if !strings.HasPrefix(line, "#") && line != "OK" {
				t.Error("Response contains non-comment lines other than OK:", "'"+line+"'")
				t.Error(buf.String())
				t.FailNow()
			}
		}
	})
	t.Run("help for non-existent cmd", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		if err := handleCmd(&pipe, "HELP", "CCMD", ProtoInfo{}, nil); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		if !strings.HasPrefix(buf.String(), "ERR") {
			t.Error("HELP command not failed")
			t.Error(buf.String())
		}
	})
	t.Run("help for cmd", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)
		proto := ProtoInfo{}

		proto.Help = make(map[string][]string)
		proto.Help["CCMD"] = []string{"help string"}
		if err := handleCmd(&pipe, "HELP", "CCMD", proto, nil); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		if buf.String() != "# help string\nOK\n" {
			t.Error("Mismatched output:")
			t.Error(buf.String())
		}
	})
}

func customCmdTest(t *testing.T) {
	t.Run("unknown cmd", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		if err := handleCmd(&pipe, "CCMD", "test", ProtoInfo{}, nil); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		if !strings.HasPrefix(buf.String(), "ERR") {
			t.Error("CCMD command not failed")
			t.Error(buf.String())
		}
	})
	t.Run("common.Error from cmd handler", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		proto := ProtoInfo{}
		proto.Handlers = make(map[string]CommandHandler)
		proto.Handlers["CCMD"] = func(_ *common.Pipe, _ interface{}, _ string) error {
			return &common.Error{
				Src: common.ErrSrcAssuan, Code: common.ErrAssUnknownCmd,
				SrcName: "assuan", Message: "TEST ERROR",
			}
		}

		if err := handleCmd(&pipe, "CCMD", "", proto, nil); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		if !strings.HasPrefix(buf.String(), "ERR") {
			t.Error("OPTION command not failed")
			t.Error(buf.String())
		}
	})
}

func optionsTest(t *testing.T) {
	t.Run("no OPTION support", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		if err := handleCmd(&pipe, "OPTION", "a 2", ProtoInfo{}, nil); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		if !strings.HasPrefix(buf.String(), "ERR") {
			t.Error("OPTION command not failed")
			t.Error(buf.String())
		}
	})
	t.Run("common.Error from SetOption", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		proto := ProtoInfo{}
		proto.SetOption = func(_ interface{}, _, _ string) error {
			return &common.Error{
				Src: common.ErrSrcAssuan, Code: common.ErrAssUnknownCmd,
				SrcName: "assuan", Message: "TEST ERROR",
			}
		}

		if err := handleCmd(&pipe, "OPTION", "a 2", proto, nil); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		if !strings.HasPrefix(buf.String(), "ERR") {
			t.Error("OPTION command not failed")
			t.Error(buf.String())
		}
	})
	t.Run("correct value passed", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		key, val := "", ""

		proto := ProtoInfo{}
		proto.SetOption = func(_ interface{}, k, v string) error {
			key, val = k, v
			return nil
		}

		if err := handleCmd(&pipe, "OPTION", "a 2", proto, nil); err != nil {
			t.Error("Unexpected handleCmd error:", err)
			t.FailNow()
		}
		if strings.HasPrefix(buf.String(), "ERR") {
			t.Error("OPTION command failed")
			t.Error(buf.String())
		}

		if key != "a" || val != "2" {
			t.Errorf("Mismatched key-value: wanted %s/%s, got %s/%s", "a", "2", key, val)
		}
	})
}
