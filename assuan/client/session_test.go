//nolint:errcheck
package client_test

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"testing"

	assuan "win-gpg-agent/assuan/client"
	"win-gpg-agent/assuan/common"
)

func ExampleSession() {
	// Connect to dirmngr.
	conn, _ := net.Dial("unix", ".gnupg/S.dirmngr")
	ses, _ := assuan.Init(conn)
	defer ses.Close()

	// Search for my key on default keyserver.
	data, _ := ses.SimpleCmd("KS_SEARCH", "foxcpp")
	fmt.Println(string(data))
	// data []byte = "info:1:1%0Apub:2499BEB8B47B0235009A5F0AEE8384B0561A25AF:..."

	// More complex transaction: send key to keyserver.
	ses.Transact("KS_PUT", "", map[string]interface{}{
		"KEYBLOCK":      []byte{},
		"KEYBLOCK_INFO": []byte{},
	})
}

func TestInitClose(t *testing.T) {
	srvResp := strings.NewReader(`OK Pleased to meet you
`)
	clReq := bytes.Buffer{}

	ses, err := assuan.Init(common.ReadWriter{Reader: srvResp, Writer: &clReq})
	if err != nil {
		t.Log("Unexpected error on client.Init:", err)
		t.FailNow()
	}

	ses.Close() // Should send BYE.

	if !strings.HasSuffix(clReq.String(), "BYE\n") {
		t.Error("sesion.Close didn't sent BYE")
	}
}

func TestSession_SimpleCmd(t *testing.T) {
	srvResp := strings.NewReader(`OK Pleased to meet you
OK`)
	clReq := bytes.Buffer{}

	ses, err := assuan.Init(common.ReadWriter{Reader: srvResp, Writer: &clReq})
	if err != nil {
		t.Log("Unexpected error on client.Init:", err)
		t.FailNow()
	}

	data, err := ses.SimpleCmd("TESTCMD", "PARAMS_123")
	if err != nil {
		t.Log("Unexpected error on client.SimpleCmd:", err)
	}
	if len(data) != 0 {
		t.Log("Unexpected data received:", data)
	}
}

func TestSession_SimpleCmdData(t *testing.T) {
	srvResp := strings.NewReader(`OK Pleased to meet you
D ABCDEF
OK`)
	clReq := bytes.Buffer{}

	ses, err := assuan.Init(common.ReadWriter{Reader: srvResp, Writer: &clReq})
	if err != nil {
		t.Log("Unexpected error on client.Init:", err)
		t.FailNow()
	}
	defer ses.Close()

	data, err := ses.SimpleCmd("TESTCMD", "PARAMS_123")
	if err != nil {
		t.Log("Unexpected error on client.SimpleCmd:", err)
	}
	if string(data) != string("ABCDEF") {
		t.Log("Wrong data received:", string(data))
	}
}

type DummmyMarhshaller struct {
	s string
}

func (dm DummmyMarhshaller) MarshalText() ([]byte, error) {
	return []byte(dm.s), nil
}

func TestSession_Transact(t *testing.T) {
	srvResp := strings.NewReader(`OK Pleased to meet you
INQUIRE foo
INQUIRE bar
INQUIRE baz
OK
`)
	clReq := bytes.Buffer{}
	ses, err := assuan.Init(common.ReadWriter{Reader: srvResp, Writer: &clReq})
	if err != nil {
		t.Log("Unexpected error on client.Init:", err)
		t.FailNow()
	}
	defer ses.Close()

	_, err = ses.Transact("CMD", "params", map[string]interface{}{
		"foo": []byte("FOO"),
		"bar": bytes.NewReader([]byte("BAR")),
		"baz": DummmyMarhshaller{s: "BAZ"},
	})
	if err != nil {
		t.Log("Unexpected error on client.Transact:", err)
		t.FailNow()
	}

	expectedOutput := `CMD params
D FOO
END
D BAR
END
D BAZ
END
`

	if clReq.String() != expectedOutput {
		t.Error("Client sent different output:")
		//t.Error("Expected:", "'"+expectedOutput+"'")
		//t.Error("Got:", "'"+clReq.String()+"'")
		t.Error("Expected:", []byte(expectedOutput))
		t.Error("Got:", clReq.Bytes())
	}
}
