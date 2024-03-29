// go:build windows

package wincred

import (
	"encoding/binary"
	"reflect"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// uf16PtrToString creates a Go string from a pointer to a UTF16 encoded zero-terminated string.
func utf16PtrToString(wstr *uint16) string {
	return windows.UTF16PtrToString(wstr)
}

// utf16ToByte creates a byte array from a given UTF 16 char array.
func utf16ToByte(wstr []uint16) (result []byte) {
	result = make([]byte, len(wstr)*2)
	for i := range wstr {
		binary.LittleEndian.PutUint16(result[(i*2):(i*2)+2], wstr[i])
	}
	return
}

// utf16FromString creates a UTF16 char array from a string.
func utf16FromString(str string) []uint16 {
	res, err := windows.UTF16FromString(str)
	if err != nil {
		return []uint16{}
	}
	return res
}

// goBytes copies the given C byte array to a Go byte array (see `C.GoBytes`).
// This function avoids having cgo as dependency.
func goBytes(src uintptr, len uint32) []byte {
	if src == uintptr(0) {
		return []byte{}
	}
	rv := make([]byte, len)
	copy(rv, *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: src,
		Len:  int(len),
		Cap:  int(len),
	})))
	return rv
}

// Convert the given CREDENTIAL struct to a more usable structure.
func sysToCredential(cred *sysCREDENTIAL) (result *Credential) {
	if cred == nil {
		return nil
	}
	result = new(Credential)
	result.Comment = utf16PtrToString(cred.Comment)
	result.TargetName = utf16PtrToString(cred.TargetName)
	result.TargetAlias = utf16PtrToString(cred.TargetAlias)
	result.UserName = utf16PtrToString(cred.UserName)
	result.LastWritten = time.Unix(0, cred.LastWritten.Nanoseconds())
	result.Persist = CredentialPersistence(cred.Persist)
	result.CredentialBlob = goBytes(cred.CredentialBlob, cred.CredentialBlobSize)
	result.Attributes = make([]CredentialAttribute, cred.AttributeCount)
	attrSlice := *(*[]sysCREDENTIAL_ATTRIBUTE)(unsafe.Pointer(&reflect.SliceHeader{
		Data: cred.Attributes,
		Len:  int(cred.AttributeCount),
		Cap:  int(cred.AttributeCount),
	}))
	for i, attr := range attrSlice {
		resultAttr := &result.Attributes[i]
		resultAttr.Keyword = utf16PtrToString(attr.Keyword)
		resultAttr.Value = goBytes(attr.Value, attr.ValueSize)
	}
	return result
}

// Convert the given Credential object back to a CREDENTIAL struct, which can be used for calling the Windows APIs.
func sysFromCredential(cred *Credential) (result *sysCREDENTIAL) {
	if cred == nil {
		return nil
	}
	result = new(sysCREDENTIAL)
	result.Flags = 0
	result.Type = 0
	result.TargetName, _ = windows.UTF16PtrFromString(cred.TargetName)
	result.Comment, _ = windows.UTF16PtrFromString(cred.Comment)
	result.LastWritten = windows.NsecToFiletime(cred.LastWritten.UnixNano())
	result.CredentialBlobSize = uint32(len(cred.CredentialBlob))
	if len(cred.CredentialBlob) > 0 {
		result.CredentialBlob = uintptr(unsafe.Pointer(&cred.CredentialBlob[0]))
	} else {
		result.CredentialBlob = 0
	}
	result.Persist = uint32(cred.Persist)
	result.AttributeCount = uint32(len(cred.Attributes))
	attributes := make([]sysCREDENTIAL_ATTRIBUTE, len(cred.Attributes))
	if len(attributes) > 0 {
		result.Attributes = uintptr(unsafe.Pointer(&attributes[0]))
	} else {
		result.Attributes = 0
	}
	for i := range cred.Attributes {
		inAttr := &cred.Attributes[i]
		outAttr := &attributes[i]
		outAttr.Keyword, _ = windows.UTF16PtrFromString(inAttr.Keyword)
		outAttr.Flags = 0
		outAttr.ValueSize = uint32(len(inAttr.Value))
		if len(inAttr.Value) > 0 {
			outAttr.Value = uintptr(unsafe.Pointer(&inAttr.Value[0]))
		} else {
			outAttr.Value = 0
		}
	}
	result.TargetAlias, _ = windows.UTF16PtrFromString(cred.TargetAlias)
	result.UserName, _ = windows.UTF16PtrFromString(cred.UserName)

	return
}
