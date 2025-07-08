//go:build linux || darwin

package main

func checkCanSymlink() bool {
	return true
}
