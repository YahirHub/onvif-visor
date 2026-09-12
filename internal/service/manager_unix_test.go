//go:build !windows

package service

import (
	"strings"
	"testing"
)

func TestRenderSystemdUnitUsesInstalledBinary(t *testing.T) {
	p := platformPaths()
	unit := renderSystemdUnit(p)
	for _, want := range []string{
		"WorkingDirectory=/opt/onvif-visor",
		"ExecStart=/opt/onvif-visor/onvif-visor service-run",
		"Restart=on-failure",
		"UMask=0077",
		"LimitNOFILE=65536",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("systemd unit missing %q:\n%s", want, unit)
		}
	}
}

func TestRenderOpenRCScriptModes(t *testing.T) {
	p := platformPaths()
	supervised := renderOpenRCScript(p, true)
	if !strings.Contains(supervised, `supervisor="supervise-daemon"`) || strings.Contains(supervised, "command_background") {
		t.Fatalf("unexpected supervised OpenRC script:\n%s", supervised)
	}
	background := renderOpenRCScript(p, false)
	if !strings.Contains(background, `command_background="yes"`) || !strings.Contains(background, `pidfile="/run/onvif-visor.pid"`) {
		t.Fatalf("unexpected fallback OpenRC script:\n%s", background)
	}
}

func TestRenderRunitAndSysVScripts(t *testing.T) {
	p := platformPaths()
	runit := renderRunitScript(p)
	if !strings.Contains(runit, "service-run") {
		t.Fatalf("runit script missing service-run:\n%s", runit)
	}
	sysv := renderSysVScript(p)
	for _, want := range []string{"start-stop-daemon", "service-run", "TERM/20/KILL/5", "Default-Start:"} {
		if !strings.Contains(sysv, want) {
			t.Fatalf("SysV script missing %q:\n%s", want, sysv)
		}
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("/tmp/a'b")
	if got != `'/tmp/a'\''b'` {
		t.Fatalf("shellQuote = %q", got)
	}
}
