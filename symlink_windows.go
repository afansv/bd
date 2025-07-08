//go:build windows

package main

import (
	"golang.org/x/sys/windows/registry"
)

// symlink on windows can be accessed only if dev mode is enabled
func checkCanSymlink() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func(k registry.Key) {
		_ = k.Close()
	}(k)
	val, _, err := k.GetIntegerValue("AllowDevelopmentWithoutDevLicense")
	if err != nil {
		return false
	}
	return val == 1
}
