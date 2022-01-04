package util

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"time"
	"unsafe"

	"github.com/lxn/win"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows"
)

const (
	xAgentClassName          = "NSSSH:AGENTWND"
	xAgentInstanceClassName  = "STATIC"
	xAgentInstanceWindowName = "_SINGLE_INSTANCE::XAGENT"

	letterBytes = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func XAgentCookieString(n int) string {

	var b = make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

type window struct {
	class string
	wh    win.HWND
}

func (w *window) Close() (err error) {
	if w == nil {
		return nil
	}
	if w.wh != 0 {
		if !win.DestroyWindow(w.wh) {
			return fmt.Errorf("unable to DestroyWindow %08x of class %s", w.wh, w.class)
		}
	}
	defer func() {
		if !win.UnregisterClass(windows.StringToUTF16Ptr(w.class)) {
			err = multierr.Append(err, fmt.Errorf("unable to UnregisterClass %s", w.class))
		}
	}()
	return nil
}

type XAgentAdvertiser struct {
	cookieWnd   *window
	instanceWnd *window
}

func createWindow(class, name string) (*window, error) {

	wc := win.WNDCLASSEX{
		HInstance:     win.GetModuleHandle(nil),
		LpszClassName: windows.StringToUTF16Ptr(class),
		LpfnWndProc:   windows.NewCallback(win.DefWindowProc),
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))

	if a := win.RegisterClassEx(&wc); a == 0 {
		return nil, fmt.Errorf("unable to RegisterClassEx for %s", class)
	}

	wnd := &window{class: class}
	if wnd.wh = win.CreateWindowEx(0, wc.LpszClassName, windows.StringToUTF16Ptr(name), 0, 0, 0, 0, 0, 0, 0, 0, nil); wnd.wh == 0 {
		return nil, multierr.Append(fmt.Errorf("unable to CreateWindowEx for %s", name), wnd.Close())
	}
	return wnd, nil
}

// AdvertiseXAgent() sets up server side part of XAgent protocol.
func AdvertiseXAgent(cookie string, port int) (xa *XAgentAdvertiser, err error) {
	xa = &XAgentAdvertiser{}
	if xa.cookieWnd, err = createWindow(xAgentClassName, cookie); err != nil {
		return nil, err
	}
	if xa.instanceWnd, err = createWindow(xAgentInstanceClassName, xAgentInstanceWindowName); err != nil {
		return nil, multierr.Append(err, xa.cookieWnd.Close())
	}
	win.SetWindowLong(xa.cookieWnd.wh, -21, int32(port))
	return xa, nil
}

// Close implements io.Closer.
func (xa *XAgentAdvertiser) Close() error {
	if xa != nil {
		return multierr.Append(xa.instanceWnd.Close(), xa.cookieWnd.Close())
	}
	return nil
}

//XAgentPerformHandshake exchanges handshake data with client.
func XAgentPerformHandshake(conn io.ReadWriter, cookie string) error {

	var length [4]byte
	if _, err := io.ReadFull(conn, length[:]); err != nil {
		return err
	}

	l := binary.BigEndian.Uint32(length[:]) + 4
	if l > (16 << 20) {
		return fmt.Errorf("xagent request too large: %d", l)
	}

	bufIn := make([]byte, l)
	if _, err := io.ReadFull(conn, bufIn); err != nil {
		return err
	}

	var req struct {
		Flag   uint32 `sshtype:"99"`
		Length uint32
		Cookie []byte `ssh:"rest"`
	}

	if err := ssh.Unmarshal(bufIn, &req); err != nil {
		return err
	}

	if int(req.Length) != len(cookie) {
		return fmt.Errorf("xagent invalid cookie length")
	}
	if int(req.Length) < len(req.Cookie) {
		return fmt.Errorf("xagent invalid message length")
	}
	if string(req.Cookie) != cookie {
		return fmt.Errorf("xagent invalid cookie")
	}

	var resp struct {
		Flag uint32 `sshtype:"99"`
	}
	resp.Flag = req.Flag

	msg := ssh.Marshal(&resp)

	bufOut := bytes.NewBuffer(nil)
	err := binary.Write(bufOut, binary.BigEndian, uint32(len(msg)))
	if err != nil {
		return err
	}
	_, err = bufOut.Write(msg)
	if err != nil {
		return err
	}

	if _, err := conn.Write(bufOut.Bytes()); err != nil {
		return err
	}
	return nil
}
