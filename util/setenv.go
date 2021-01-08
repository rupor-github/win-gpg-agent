package util

import (
	"log"
	"os"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	wslName = "WSLENV"
)

func notifySystem() {
	var (
		mod             = windows.NewLazySystemDLL("user32")
		proc            = mod.NewProc("SendMessageTimeoutW")
		wmSETTINGCHANGE = uint32(0x001A)
		smtoABORTIFHUNG = uint32(0x0002)
		smtoNORMAL      = uint32(0x0000)
	)

	start := time.Now()
	log.Printf("Broadcasting environment change. From %s", start)

	_, _, _ = proc.Call(uintptr(windows.InvalidHandle),
		uintptr(wmSETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Environment"))),
		uintptr(smtoNORMAL|smtoABORTIFHUNG),
		uintptr(1000),
		0)

	log.Printf("Broadcasting environment change. To   %s, Elapsed %s", time.Now(), time.Since(start))
}

// PrepareUserEnvironmentVariable modifies user environment. if wslenv is true - its name is added to WSLENV/up list for path translation.
func PrepareUserEnvironmentVariable(name, value string, wslenv, translate bool) error {

	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.READ|registry.WRITE)
	if err != nil {
		return err
	}
	defer k.Close()

	if err := k.SetStringValue(name, value); err != nil {
		return err
	}
	log.Printf("Set '%s=%s'", name, value)
	defer notifySystem()

	if !wslenv {
		return nil
	}

	val, _, err := k.GetStringValue(wslName)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	log.Printf("Was '%s=%s'", wslName, val)

	parts := strings.Split(val, ":")
	vals := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		if !strings.HasPrefix(part, name) {
			vals = append(vals, part)
		}
	}
	name += "/u"
	if translate {
		name += "p"
	}
	vals = append(vals, name)
	val = strings.Join(vals, ":")

	if err := k.SetStringValue(wslName, val); err != nil {
		return err
	}
	log.Printf("Set '%s=%s'", wslName, val)

	return nil
}

// CleanUserEnvironmentVariable will reverse settings done by PrepareUserEnvironmentVariable.
func CleanUserEnvironmentVariable(name string, wslenv bool) error {

	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.READ|registry.WRITE)
	if err != nil {
		return err
	}
	defer k.Close()

	if err := k.DeleteValue(name); err != nil {
		return err
	}
	log.Printf("Del '%s'", name)
	defer notifySystem()

	if !wslenv {
		return nil
	}

	val, _, err := k.GetStringValue(wslName)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	log.Printf("Was '%s=%s'", wslName, val)

	parts := strings.Split(val, ":")
	vals := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		if !strings.HasPrefix(part, name) {
			vals = append(vals, part)
		}
	}
	val = strings.Join(vals, ":")

	if len(val) == 0 {
		if err := k.DeleteValue(wslName); err != nil {
			return err
		}
		log.Printf("Del '%s'", wslName)
	} else {
		if err := k.SetStringValue(wslName, val); err != nil {
			return err
		}
		log.Printf("Set '%s=%s'", wslName, val)
	}
	return nil
}
