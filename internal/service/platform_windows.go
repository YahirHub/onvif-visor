//go:build windows

package service

import (
	"errors"
	"io/fs"
)

var errWindowsServiceUnsupported = errors.New("--install currently supports Unix-like service managers: systemd, OpenRC, runit and SysV init")

func platformPaths() paths                    { return paths{} }
func preparePlatformDirs(paths) error         { return errWindowsServiceUnsupported }
func applyFileMode(string, fs.FileMode) error { return nil }
func replaceFile(string, string) error        { return errWindowsServiceUnsupported }
func cliEntryPath(paths) string               { return "" }
func installCLIEntry(paths) error             { return errWindowsServiceUnsupported }
func removeCLIEntry(paths) error              { return nil }
func EnsureElevated([]string) (bool, error)   { return false, errWindowsServiceUnsupported }
func detectManager() (manager, error)         { return nil, errWindowsServiceUnsupported }
func managerByName(string) (manager, error)   { return nil, errWindowsServiceUnsupported }
