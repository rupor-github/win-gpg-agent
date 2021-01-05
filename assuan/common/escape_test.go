package common

import "testing"

func TestEscapeParams(t *testing.T) {
	if EscapeParameters("\r\n%") != "%0D%0A%25" {
		t.Error("\\r\\n% should be escaped to %0D%0A%25")
	}
	if EscapeParameters("foobar\\") != "foobar%5C" {
		t.Error("foobar\\ should be escaped to foobar%5C")
	}
}

func TestUnescapeParams(t *testing.T) {
	res, err := UnescapeParameters("%0D%0A%25%5C")
	if err != nil {
		t.Error("unescape %0D%0A%25%5C:", err)
		t.FailNow()
	}
	if res != "\r\n%\\" {
		t.Error("%0D%0A%25%5C should be de-escaped to \\r\\n%\\")
	}

	// https://github.com/foxcpp/go-assuan/pull/1
	res, err = UnescapeParameters("+++")
	if err != nil {
		t.Error("unescape +++:", err)
		t.FailNow()
	}
	if res != "+++" {
		t.Error("common.UnescapeParameters removes + from output")
	}
}
