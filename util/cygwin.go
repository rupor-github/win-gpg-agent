package util

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"golang.org/x/sys/windows"
)

// CygwinNonceString converts binary nonce to printable string in net order.
func CygwinNonceString(nonce [16]byte) string {
	var buf [35]byte
	dst := buf[:]
	for i := 0; i < 4; i++ {
		b := nonce[i*4 : i*4+4]
		hex.Encode(dst[i*9:i*9+8], []byte{b[3], b[2], b[1], b[0]})
		if i != 3 {
			dst[9*i+8] = '-'
		}
	}
	return string(buf[:])
}

// CygwinCreateSocketFile creates CygWin socket file with proper content and attributes.
func CygwinCreateSocketFile(fname string, port int) (nonce [16]byte, err error) {
	if _, err = rand.Read(nonce[:]); err != nil {
		return
	}
	if err = ioutil.WriteFile(fname, []byte(fmt.Sprintf("!<socket >%d s %s", port, CygwinNonceString(nonce))), 0600); err != nil {
		return
	}
	var cpath *uint16
	if cpath, err = windows.UTF16PtrFromString(fname); err != nil {
		return
	}
	err = windows.SetFileAttributes(cpath, windows.FILE_ATTRIBUTE_SYSTEM|windows.FILE_ATTRIBUTE_READONLY)
	return
}

// CygwinPerformHandshake exchanges handshake data.
func CygwinPerformHandshake(conn io.ReadWriter, nonce [16]byte) error {

	var nonceR [16]byte
	if _, err := conn.Read(nonceR[:]); err != nil {
		return err
	}
	if !bytes.Equal(nonce[:], nonceR[:]) {
		return fmt.Errorf("invalid nonce received - expecting %x but got %x", nonce[:], nonceR[:])
	}
	if _, err := conn.Write(nonce[:]); err != nil {
		return err
	}

	// read client pid:uid:gid
	buf := make([]byte, 12)
	if _, err := conn.Read(buf); err != nil {
		return err
	}

	// Send back our info, making sure that gid:uid are the same as received
	binary.LittleEndian.PutUint32(buf, uint32(os.Getpid()))
	if _, err := conn.Write(buf); err != nil {
		return err
	}
	return nil
}
