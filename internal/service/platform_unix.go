//go:build !windows

package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

func platformPaths() paths {
	installDir := "/opt/onvif-visor"
	return paths{
		InstallDir:    installDir,
		Binary:        filepath.Join(installDir, "onvif-visor"),
		DataDir:       filepath.Join(installDir, "data"),
		RecordingsDir: filepath.Join(installDir, "recordings"),
		ModelsDir:     filepath.Join(installDir, "models"),
		Metadata:      filepath.Join(installDir, "install.json"),
		LogDir:        "/var/log/onvif-visor",
	}
}

func preparePlatformDirs(p paths) error {
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{
		{p.InstallDir, 0o755},
		{p.DataDir, 0o700},
		{p.RecordingsDir, 0o700},
		{p.ModelsDir, 0o755},
		{p.LogDir, 0o700},
	} {
		if err := os.MkdirAll(item.path, item.mode); err != nil {
			return fmt.Errorf("create %s: %w", item.path, err)
		}
		if err := os.Chmod(item.path, item.mode); err != nil {
			return fmt.Errorf("chmod %s: %w", item.path, err)
		}
	}
	return nil
}

func applyFileMode(path string, mode fs.FileMode) error { return os.Chmod(path, mode) }
func replaceFile(source, destination string) error      { return os.Rename(source, destination) }
func cliEntryPath(_ paths) string                       { return "/usr/local/bin/onvif-visor" }

func installCLIEntry(p paths) error {
	entry := cliEntryPath(p)
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(entry); err == nil {
		if info.IsDir() {
			return fmt.Errorf("refusing to replace directory %s", entry)
		}
		if err := os.Remove(entry); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp := entry + ".new"
	_ = os.Remove(tmp)
	if err := os.Symlink(p.Binary, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, entry); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func removeCLIEntry(p paths) error {
	entry := cliEntryPath(p)
	info, err := os.Lstat(entry)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	target, err := os.Readlink(entry)
	if err != nil {
		return err
	}
	if filepath.Clean(target) != filepath.Clean(p.Binary) {
		return nil
	}
	return os.Remove(entry)
}

func EnsureElevated(args []string) (bool, error) {
	if os.Geteuid() == 0 {
		return false, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("resolve executable for privilege escalation: %w", err)
	}
	for _, candidate := range []string{"sudo", "doas", "pkexec"} {
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		commandArgs := append([]string{executable}, args...)
		if candidate == "sudo" || candidate == "doas" {
			commandArgs = append([]string{"--", executable}, args...)
		}
		cmd := exec.Command(path, commandArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return true, fmt.Errorf("privilege escalation with %s failed: %w", candidate, err)
		}
		return true, nil
	}
	return false, errors.New("administrator privileges are required; install sudo, doas or pkexec, or rerun ONVIF Visor as root")
}
