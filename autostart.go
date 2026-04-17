package main

import (
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const regKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const regValueName = "Speedo"

func isAutoStartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(regValueName)
	return err == nil
}

func setAutoStart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enable {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		// CommandLineToArgvW strips a single layer of double quotes; embedded
		// quotes inside the path must be escaped by doubling them so the launched
		// process receives the original path.
		escaped := strings.ReplaceAll(exe, `"`, `""`)
		return k.SetStringValue(regValueName, `"`+escaped+`"`)
	}
	return k.DeleteValue(regValueName)
}
