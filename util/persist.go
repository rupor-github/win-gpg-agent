package util

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/sys/windows/registry"
)

var keyName = fmt.Sprintf(`Software\win-gpg-agent\%s`, WinAgentName)

// GetIntOption reads named integer value from registry. If value does not exist or there is a problem - default is returned.
func GetIntOption(name string, def uint64) uint64 {

	k, exist, err := registry.CreateKey(registry.CURRENT_USER, keyName, registry.QUERY_VALUE|registry.READ|registry.WRITE)
	if err != nil {
		log.Printf("Unable to CreateKey %s: %v", keyName, err)
		return def
	}
	defer k.Close()

	if !exist {
		return def
	}

	val, _, err := k.GetIntegerValue(name)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf(`Unable to GetIntegerValue %s\%s: %v`, keyName, name, err)
		}
		return def
	}
	return val
}

// SetIntOption stores named integer value to registry. Key must exist. Errors are ignored.
func SetIntOption(name string, val uint64) {

	k, err := registry.OpenKey(registry.CURRENT_USER, keyName, registry.QUERY_VALUE|registry.READ|registry.WRITE)
	if err != nil {
		log.Printf("Unable to OpenKey %s: %v", keyName, err)
		return
	}
	defer k.Close()

	if err := k.SetQWordValue(name, val); err != nil {
		log.Printf(`Unable to SetQWordValue %s\%s %x: %v`, keyName, name, val, err)
		return
	}
}
