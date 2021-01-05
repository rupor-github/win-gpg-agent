package common_test

import (
	"bytes"
	"strings"
	"testing"

	"win-gpg-agent/assuan/common"
)

func TestPipe_ReadLine(t *testing.T) {
	t.Run("simple line with escaped chars: CMD par%41ms", func(t *testing.T) {
		sample := "CMD par%41ms\n"
		rdr := strings.NewReader(sample)
		pipe := common.NewPipe(rdr, nil)
		defer pipe.Close()

		cmd, params, err := pipe.ReadLine()

		if err != nil {
			t.Error("Unexpected error on pipe.ReadLine:", err)
		}
		if cmd != "CMD" {
			t.Errorf("Command mismatch: wanted %s, got %s", "CMD", cmd)
		}
		if params != "parAms" {
			t.Errorf("Params mismatch: wanted %s, got %s", "parAms", params)
		}
	})
	t.Run("too long line", func(t *testing.T) {
		sample := "CMD " + strings.Repeat("F", common.MaxLineLen+20)
		rdr := strings.NewReader(sample)
		pipe := common.NewPipe(rdr, nil)
		defer pipe.Close()

		_, _, err := pipe.ReadLine()

		if err == nil {
			t.Error("pipe.ReadLine should fail, but succeed")
		}
	})
	t.Run("comments", func(t *testing.T) {
		sample := `# asd asd df d fd fd f
# as d sf d fd f df d 
CMD F!_)`
		rdr := strings.NewReader(sample)
		pipe := common.NewPipe(rdr, nil)
		defer pipe.Close()

		cmd, params, err := pipe.ReadLine()
		if err != nil {
			t.Error("Unexpected error on pipe.ReadLine:", err)
		}
		if cmd != "CMD" {
			t.Errorf("Command mismatch: wanted %s, got %s", "CMD", cmd)
		}
		if params != "F!_)" {
			t.Errorf("Params mismatch: wanted %s, got %s", "F!_)", params)
		}
	})
}

func TestPipe_WriteLine(t *testing.T) {
	t.Run("simple write: CMD par\\rams", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)
		defer pipe.Close()

		err := pipe.WriteLine("CMD", "par\rams")

		if err != nil {
			t.Error("Unexpected error on pipe.WriteLine:", err)
			t.FailNow()
		}
		if buf.String() != "CMD par%0Dams\n" {
			t.Errorf("pipe.WriteLine wrote incorrect line: '%s'", buf.String())
		}
	})
	t.Run("too long line", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)

		err := pipe.WriteLine("CMD", strings.Repeat("F", common.MaxLineLen+20))

		if err == nil {
			t.Error("pipe.WriteLine didn't refused to write too long line")
		}
	})
}

func TestPipe_WriteData(t *testing.T) {
	t.Run("simple write: \\rBC", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)
		defer pipe.Close()

		err := pipe.WriteData([]byte("\rBC"))

		if err != nil {
			t.Error("Unexpected error on pipe.WriteData:", err)
			t.FailNow()
		}
		if buf.String() != "D %0DBC\n" {
			t.Errorf("pipe.WriteLine wrote incorrect line: '%s'", buf.String())
		}
	})
	t.Run("line wrapping", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)
		defer pipe.Close()

		data := []byte(strings.Repeat("F", common.MaxLineLen*7))

		err := pipe.WriteData(data)

		if err != nil {
			t.Error("Unexpected error on pipe.WriteData:", err)
		}
		splitten := strings.Split(buf.String(), "\n")
		for _, part := range splitten {
			if len(part)+1 > common.MaxLineLen {
				t.Error("pipe.WriteData wrote line bigger than MaxLineLen")
				t.FailNow()
			}
		}
	})
	t.Run("from io.Reader", func(t *testing.T) {
		buf := bytes.Buffer{}
		pipe := common.NewPipe(nil, &buf)
		defer pipe.Close()

		data := []byte("ABCDEF")

		err := pipe.WriteDataReader(bytes.NewReader(data))

		if err != nil {
			t.Error("Unexpected error on pipe.WriteData:", err)
			t.FailNow()
		}
		if buf.String() != "D ABCDEF\n" {
			t.Errorf("pipe.WriteData wrote wrong line: '%s'", buf.String())
		}
	})
	// TODO: Test that wrapping is done correctly and no data is corrupted.
}

func TestPipe_ReadData(t *testing.T) {
	t.Run("simple read", func(t *testing.T) {
		sample := `D ABCDEF
END`
		pipe := common.NewPipe(strings.NewReader(sample), nil)
		defer pipe.Close()

		data, err := pipe.ReadData()

		if err != nil {
			t.Error("Unexpected error on pipe.ReadData:", err)
			t.FailNow()
		}
		if string(data) != "ABCDEF" {
			t.Error("pipe.ReadData read incorrect data:", string(data))
		}
	})
	t.Run("wrapped", func(t *testing.T) {
		sample := `D ABCDEF
D ABCDEF
END
`
		pipe := common.NewPipe(strings.NewReader(sample), nil)
		defer pipe.Close()

		data, err := pipe.ReadData()

		if err != nil {
			t.Error("Unexpected error on pipe.ReadData:", err)
			t.FailNow()
		}
		if string(data) != "ABCDEFABCDEF" {
			t.Error("pipe.ReadData read incorrect data:", string(data))
		}
	})
	t.Run("escaped", func(t *testing.T) {
		sample := `D %41BCDEF
END
`
		pipe := common.NewPipe(strings.NewReader(sample), nil)
		defer pipe.Close()

		data, err := pipe.ReadData()

		if err != nil {
			t.Error("Unexpected error on pipe.ReadData:", err)
			t.FailNow()
		}
		if string(data) != "ABCDEF" {
			t.Error("pipe.ReadData read incorrect data:", string(data))
		}
	})
}
