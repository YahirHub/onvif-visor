//go:build !windows

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const managerCommandTimeout = 30 * time.Second

func detectManager() (manager, error) {
	if commandExists("systemctl") && dirExists("/run/systemd/system") {
		return systemdManager{}, nil
	}
	if commandExists("rc-service") && commandExists("rc-update") {
		return openRCManager{}, nil
	}
	if commandExists("sv") && runitServiceRoot() != "" {
		return runitManager{}, nil
	}
	if dirExists("/etc/init.d") && (commandExists("service") || commandExists("start-stop-daemon")) {
		return sysVManager{}, nil
	}
	return nil, errors.New("no supported service manager detected; supported managers: systemd, OpenRC, runit and SysV init")
}

func managerByName(name string) (manager, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "systemd":
		return systemdManager{}, nil
	case "openrc":
		return openRCManager{}, nil
	case "runit":
		return runitManager{}, nil
	case "sysv":
		return sysVManager{}, nil
	default:
		return nil, fmt.Errorf("unsupported installed service manager %q", name)
	}
}

type systemdManager struct{}

func (systemdManager) Name() string { return "systemd" }
func (systemdManager) StopIfInstalled() error {
	_, err := runCommand("systemctl", "stop", Name+".service")
	return ignoreMissingService(err)
}
func (systemdManager) Install(p paths) error {
	path := "/etc/systemd/system/" + Name + ".service"
	if err := os.WriteFile(path, []byte(renderSystemdUnit(p)), 0o644); err != nil {
		return err
	}
	if _, err := runCommand("systemctl", "daemon-reload"); err != nil {
		return err
	}
	_, err := runCommand("systemctl", "enable", "--now", Name+".service")
	return err
}
func (systemdManager) Remove(_ paths) error {
	_, _ = runCommand("systemctl", "stop", Name+".service")
	_, _ = runCommand("systemctl", "disable", Name+".service")
	if err := os.Remove("/etc/systemd/system/" + Name + ".service"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err := runCommand("systemctl", "daemon-reload")
	return err
}
func (systemdManager) Control(action Action) (string, error) {
	if action == ActionStatus {
		return runCommand("systemctl", "--no-pager", "--full", "status", Name+".service")
	}
	return runCommand("systemctl", string(action), Name+".service")
}
func renderSystemdUnit(p paths) string {
	return fmt.Sprintf(`[Unit]
Description=ONVIF Visor camera monitoring service
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=%s
ExecStart=%s service-run
Restart=on-failure
RestartSec=2s
KillMode=control-group
TimeoutStopSec=20s
UMask=0077
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
`, p.InstallDir, p.Binary)
}

type openRCManager struct{}

func (openRCManager) Name() string { return "openrc" }
func (openRCManager) StopIfInstalled() error {
	if !fileExists("/etc/init.d/" + Name) {
		return nil
	}
	_, err := runCommand("rc-service", Name, "stop")
	if err == nil {
		return nil
	}
	status, statusErr := runCommand("rc-service", Name, "status")
	combined := strings.ToLower(status)
	if statusErr != nil {
		combined += " " + strings.ToLower(statusErr.Error())
	}
	if strings.Contains(combined, "stopped") || strings.Contains(combined, "inactive") {
		return nil
	}
	return ignoreMissingService(err)
}
func (openRCManager) Install(p paths) error {
	path := "/etc/init.d/" + Name
	if err := os.WriteFile(path, []byte(renderOpenRCScript(p, commandExists("supervise-daemon"))), 0o755); err != nil {
		return err
	}
	if _, err := runCommand("rc-update", "add", Name, "default"); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
		return err
	}
	_, err := runCommand("rc-service", Name, "start")
	return err
}
func (openRCManager) Remove(_ paths) error {
	_, _ = runCommand("rc-service", Name, "stop")
	_, _ = runCommand("rc-update", "del", Name, "default")
	err := os.Remove("/etc/init.d/" + Name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func (openRCManager) Control(action Action) (string, error) {
	return runCommand("rc-service", Name, string(action))
}
func renderOpenRCScript(p paths, supervised bool) string {
	supervisor := ""
	background := "command_background=\"yes\"\npidfile=\"/run/onvif-visor.pid\"\n"
	if supervised {
		supervisor = "supervisor=\"supervise-daemon\"\nrespawn_delay=2\n"
		background = ""
	}
	return fmt.Sprintf(`#!/sbin/openrc-run
name="ONVIF Visor"
description="ONVIF camera monitoring service"
command="%s"
command_args="service-run"
directory="%s"
%s%s
depend() {
    need net
}
`, p.Binary, p.InstallDir, supervisor, background)
}

type runitManager struct{}

func (runitManager) Name() string { return "runit" }
func (runitManager) StopIfInstalled() error {
	_, err := runCommand("sv", "down", Name)
	return ignoreMissingService(err)
}
func (runitManager) Install(p paths) error {
	root := runitServiceRoot()
	if root == "" {
		return errors.New("runit service root not found (/etc/service or /var/service)")
	}
	serviceDir := filepath.Join("/etc/sv", Name)
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "run"), []byte(renderRunitScript(p)), 0o755); err != nil {
		return err
	}
	link := filepath.Join(root, Name)
	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("refusing to replace non-symlink runit entry %s", link)
		}
		_ = os.Remove(link)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Symlink(serviceDir, link); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := runCommand("sv", "up", Name)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) || !strings.Contains(strings.ToLower(err.Error()), "supervise/ok") {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}
func (runitManager) Remove(_ paths) error {
	_, _ = runCommand("sv", "down", Name)
	root := runitServiceRoot()
	if root != "" {
		link := filepath.Join(root, Name)
		if info, err := os.Lstat(link); err == nil && info.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(link)
		}
	}
	return os.RemoveAll(filepath.Join("/etc/sv", Name))
}
func (runitManager) Control(action Action) (string, error) {
	verb := string(action)
	if action == ActionStart {
		verb = "up"
	}
	if action == ActionStop {
		verb = "down"
	}
	return runCommand("sv", verb, Name)
}
func renderRunitScript(p paths) string {
	return fmt.Sprintf("#!/bin/sh\ncd %s || exit 1\nexec %s service-run\n", shellQuote(p.InstallDir), shellQuote(p.Binary))
}
func runitServiceRoot() string {
	for _, root := range []string{"/etc/service", "/var/service"} {
		if dirExists(root) {
			return root
		}
	}
	return ""
}

type sysVManager struct{}

func (sysVManager) Name() string { return "sysv" }
func (sysVManager) StopIfInstalled() error {
	if !fileExists("/etc/init.d/" + Name) {
		return nil
	}
	_, err := runSysV("stop")
	return ignoreMissingService(err)
}
func (sysVManager) Install(p paths) error {
	if !commandExists("start-stop-daemon") {
		return errors.New("SysV fallback requires start-stop-daemon")
	}
	path := "/etc/init.d/" + Name
	if err := os.WriteFile(path, []byte(renderSysVScript(p)), 0o755); err != nil {
		return err
	}
	if commandExists("update-rc.d") {
		if _, err := runCommand("update-rc.d", Name, "defaults"); err != nil {
			return err
		}
	} else if commandExists("chkconfig") {
		if _, err := runCommand("chkconfig", "--add", Name); err != nil {
			return err
		}
	}
	_, err := runSysV("start")
	return err
}
func (sysVManager) Remove(_ paths) error {
	_, _ = runSysV("stop")
	if commandExists("update-rc.d") {
		_, _ = runCommand("update-rc.d", "-f", Name, "remove")
	}
	if commandExists("chkconfig") {
		_, _ = runCommand("chkconfig", "--del", Name)
	}
	err := os.Remove("/etc/init.d/" + Name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func (sysVManager) Control(action Action) (string, error) { return runSysV(string(action)) }
func runSysV(action string) (string, error) {
	if commandExists("service") {
		return runCommand("service", Name, action)
	}
	return runCommand("/etc/init.d/"+Name, action)
}
func renderSysVScript(p paths) string {
	return fmt.Sprintf(`#!/bin/sh
### BEGIN INIT INFO
# Provides:          onvif-visor
# Required-Start:    $network $remote_fs
# Required-Stop:     $network $remote_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: ONVIF Visor
### END INIT INFO
DAEMON=%s
DIR=%s
PIDFILE=/run/onvif-visor.pid
LOGFILE=%s/service.log
case "$1" in
  start)
    mkdir -p "%s"
    start-stop-daemon --start --quiet --oknodo --background --make-pidfile --pidfile "$PIDFILE" --chdir "$DIR" --startas /bin/sh -- -c "exec '$DAEMON' service-run >>'$LOGFILE' 2>&1"
    ;;
  stop)
    start-stop-daemon --stop --quiet --oknodo --retry TERM/20/KILL/5 --pidfile "$PIDFILE" --remove-pidfile
    ;;
  restart)
    "$0" stop
    "$0" start
    ;;
  status)
    if start-stop-daemon --status --pidfile "$PIDFILE"; then echo "ONVIF Visor: running"; exit 0; fi
    echo "ONVIF Visor: stopped"; exit 3
    ;;
  *) echo "Usage: $0 {start|stop|restart|status}"; exit 2 ;;
esac
`, p.Binary, p.InstallDir, p.LogDir, p.LogDir)
}

func runCommand(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), managerCommandTimeout)
	defer cancel()
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
		text := strings.TrimSpace(output.String())
		if text == "" {
			return "", fmt.Errorf("%s timed out after %s", name, managerCommandTimeout)
		}
		return text, fmt.Errorf("%s timed out after %s: %s", name, managerCommandTimeout, text)
	}
	text := strings.TrimSpace(output.String())
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		return text, fmt.Errorf("%s: %w: %s", name, err, text)
	}
	return text, nil
}

func ignoreMissingService(err error) error {
	if err == nil {
		return nil
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{"not found", "not-found", "does not exist", "not loaded", "unknown service", "no such file", "not supervised"} {
		if strings.Contains(text, marker) {
			return nil
		}
	}
	return err
}
func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }
func dirExists(path string) bool     { info, err := os.Stat(path); return err == nil && info.IsDir() }
func fileExists(path string) bool    { info, err := os.Stat(path); return err == nil && !info.IsDir() }
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
