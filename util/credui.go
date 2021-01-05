//+build windows

package util

import (
	"errors"
	"log"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

var (
	modCredUI                       = windows.NewLazySystemDLL("credui.dll")
	pPromptForWindowsCredentials    = modCredUI.NewProc("CredUIPromptForWindowsCredentialsW")
	pCredPackAuthenticationBuffer   = modCredUI.NewProc("CredPackAuthenticationBufferW")
	pCredUnPackAuthenticationBuffer = modCredUI.NewProc("CredUnPackAuthenticationBufferW")
)

type DlgDetails struct {
	Delay    time.Duration `yaml:"delay,omitempty"`
	WndName  string        `yaml:"name,omitempty"`
	WndClass string        `yaml:"class,omitempty"`
}

func prepareAuthBuf(user string) (buf *uint8, size uint32) {

	const CRED_PACK_GENERIC_CREDENTIALS = 0x4

	if len(user) == 0 {
		return
	}

	var pszUserName = windows.StringToUTF16Ptr(user)
	var pass uint16
	pszPassword := &pass

	r1, _, err := pCredPackAuthenticationBuffer.Call(
		uintptr(CRED_PACK_GENERIC_CREDENTIALS), // DWORD  dwFlags,
		uintptr(unsafe.Pointer(pszUserName)),   // LPWSTR pszUserName,
		uintptr(unsafe.Pointer(pszPassword)),   // LPWSTR pszPassword,
		0,                                      // PBYTE  pPackedCredentials,
		uintptr(unsafe.Pointer(&size)),         // DWORD  *pcbPackedCredentials
	)
	if r1 == 0 && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		log.Printf("CredPackAuthenticationBufferW (cred = NULL) LastErr: %s , ret: %d", err.Error(), r1)
		return nil, 0
	}

	b := make([]uint8, size)
	buf = &b[0]

	r1, _, err = pCredPackAuthenticationBuffer.Call(
		uintptr(CRED_PACK_GENERIC_CREDENTIALS), // DWORD  dwFlags,
		uintptr(unsafe.Pointer(pszUserName)),   // LPWSTR pszUserName,
		uintptr(unsafe.Pointer(pszPassword)),   // LPWSTR pszPassword,
		uintptr(unsafe.Pointer(buf)),           // PBYTE  pPackedCredentials,
		uintptr(unsafe.Pointer(&size)),         // DWORD  *pcbPackedCredentials
	)
	if r1 == 0 {
		log.Printf("CredPackAuthenticationBufferW LastErr: %s , ret: %x", err.Error(), r1)
		return nil, 0
	}
	return
}

// quick and dirty
func cleanLabel(str string) string {
	space := regexp.MustCompile(`\s+`)
	cleaner := strings.NewReplacer("\n", " ", "\r", " ")
	return space.ReplaceAllString(cleaner.Replace(str), " ")
}

type credUIInfo struct {
	Size                     uint32
	HWnd                     windows.Handle
	MessageText, CaptionText *uint16
	HBmpBanner               windows.Handle
}

const (
	CREDUI_MAX_CAPTION_LENGTH        = 128
	CREDUI_MAX_MESSAGE_LENGTH        = 32767
	CREDUI_MAX_USERNAME_LENGTH       = 256
	CREDUI_MAX_PASSWORD_LENGTH       = 256
	CREDUI_MAX_GENERIC_TARGET_LENGTH = 32767
	CREDUI_MAX_DOMAIN_TARGET_LENGTH  = 256 + 1 + 80

	CREDUIWIN_GENERIC      = 0x1  // Return the user name and password in plain text.
	CREDUIWIN_CHECKBOX     = 0x2  // The Save check box is displayed in the dialog box.
	CREDUIWIN_IN_CRED_ONLY = 0x20 // Only the credentials specified by the pvInAuthBuffer parameter for the authentication package specified by the pulAuthPackage parameter should be enumerated.
)

// PromptForWindowsCredentials calls Windows CredUI.dll to pupup "standard" Windows security dialog using provided description, prompt and a flag,
// indicating that user could make a choice to save the result in Windows Credential manager. It returns canceled flag (indicating error or user's
// refusal to complete operation) and when false string with entered password/pin and flag indicating that user checked "Remember me" checkbox.
func PromptForWindowsCredentials(details DlgDetails, errorMessage, description, prompt string, save bool) (bool, string, bool) {

	// NOTE: since pinentry is being started from arbitrary "background" process after long chain of executions timing may vary and often
	// passphrase dialog would not come into foreground (as it should) - instead meaningless icon will flash on taskbar. To fight it we
	// are going to wait a bit and then attempt to bring window into foreground ourselves. This is bad - timing cannot be predicted, there
	// may be multiple instances of pinentry running multiple dialogs, caption could be in different languages etc.
	go func() {
		<-time.After(details.Delay)
		if hwnd := win.FindWindow(windows.StringToUTF16Ptr(details.WndClass), windows.StringToUTF16Ptr(details.WndName)); hwnd != 0 {
			win.SetForegroundWindow(hwnd)
		}
	}()

	const (
		CREDUIWIN_DEFAULT = CREDUIWIN_GENERIC + CREDUIWIN_IN_CRED_ONLY
	)

	caption := "Pinentry (go)"

	description = cleanLabel(description)
	if len(errorMessage) > 0 {
		errorMessage = cleanLabel(errorMessage)
		description = errorMessage + "\n\n" + description
	}
	if len(description) > CREDUI_MAX_MESSAGE_LENGTH {
		description = description[0:CREDUI_MAX_MESSAGE_LENGTH]
	}

	prompt = cleanLabel(prompt)
	if len(prompt) > CREDUI_MAX_USERNAME_LENGTH {
		prompt = prompt[0:CREDUI_MAX_USERNAME_LENGTH]
	}

	uiInfo := credUIInfo{
		MessageText: windows.StringToUTF16Ptr(description),
		CaptionText: windows.StringToUTF16Ptr(caption),
	}
	uiInfo.Size = uint32(unsafe.Sizeof(uiInfo))

	var (
		inBuf, sizeOfInBuf = prepareAuthBuf(prompt)
		outBuf             *uint8
		sizeOfOutBuf       uint32
		authPackage        uint32
		saveFlag           int32
		dwFlags            = uint32(CREDUIWIN_DEFAULT)
	)

	if save {
		dwFlags += CREDUIWIN_CHECKBOX
		saveFlag = 1
	}

	r1, _, err := pPromptForWindowsCredentials.Call(
		uintptr(unsafe.Pointer(&uiInfo)),
		0,                                      // DWORD   dwAuthError,
		uintptr(unsafe.Pointer(&authPackage)),  // ULONG   *pulAuthPackage,
		uintptr(unsafe.Pointer(inBuf)),         // LPCVOID pvInAuthBuffer,
		uintptr(sizeOfInBuf),                   // ULONG   ulInAuthBufferSize,
		uintptr(unsafe.Pointer(&outBuf)),       // LPVOID  *ppvOutAuthBuffer,
		uintptr(unsafe.Pointer(&sizeOfOutBuf)), // ULONG   *pulOutAuthBufferSize,
		uintptr(unsafe.Pointer(&saveFlag)),     // BOOL    *pfSave,
		uintptr(dwFlags),                       // DWORD   dwFlags
	)

	// ERROR_CANCELED is the only other option
	if r1 != 0 {
		log.Printf("CredUIPromptForWindowsCredentialsW LastErr: %s, ret: %d", err.Error(), r1)
		return true, "", false
	}

	// Let's unpack the result

	var (
		szUserName       = make([]uint16, CREDUI_MAX_USERNAME_LENGTH+1)
		cchMaxUserName   = uint32(CREDUI_MAX_USERNAME_LENGTH)
		szDomainName     = make([]uint16, CREDUI_MAX_DOMAIN_TARGET_LENGTH+1)
		cchMaxDomainName = uint32(CREDUI_MAX_DOMAIN_TARGET_LENGTH)
		szPassword       = make([]uint16, CREDUI_MAX_PASSWORD_LENGTH+1)
		cchMaxPassword   = uint32(CREDUI_MAX_PASSWORD_LENGTH)
	)

	r1, _, err = pCredUnPackAuthenticationBuffer.Call(
		0,                                        // DWORD  dwFlags,
		uintptr(unsafe.Pointer(outBuf)),          // PVOID  pAuthBuffer,
		uintptr(sizeOfOutBuf),                    // DWORD  cbAuthBuffer,
		uintptr(unsafe.Pointer(&szUserName[0])),  // LPWSTR pszUserName,
		uintptr(unsafe.Pointer(&cchMaxUserName)), // DWORD  *pcchMaxUserName,
		uintptr(unsafe.Pointer(&szDomainName[0])),  // LPWSTR pszDomainName,
		uintptr(unsafe.Pointer(&cchMaxDomainName)), // DWORD  *pcchMaxDomainName,
		uintptr(unsafe.Pointer(&szPassword[0])),    // LPWSTR pszPassword,
		uintptr(unsafe.Pointer(&cchMaxPassword)),   // DWORD  *pcchMaxPassword
	)

	if r1 == 0 {
		log.Printf("CredUnPackAuthenticationBufferW LastErr: %s, ret: %d", err.Error(), r1)
		return true, "", false
	}

	res := windows.UTF16ToString(szPassword)
	windows.CoTaskMemFree(unsafe.Pointer(outBuf))

	return false, res, saveFlag != 0
}

func PromptForConfirmaion(_ DlgDetails, description, prompt string, onebutton bool) bool {

	caption := "Pinentry (go)"

	description, prompt = cleanLabel(description), cleanLabel(prompt)
	if len(prompt) > 0 {
		description = description + "\n\n" + prompt
	}

	var flags uint32
	if onebutton {
		flags = MB_OK + MB_ICONASTERISK + MB_SETFOREGROUND
	} else {
		flags = MB_YESNO + MB_ICONQUESTION + MB_SETFOREGROUND
	}

	ret := MessageBox(caption, description, uintptr(flags))
	return ret == IDYES || ret == IDOK
}
