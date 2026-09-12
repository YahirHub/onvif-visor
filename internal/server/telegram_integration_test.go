package server

import "testing"

func TestParseTelegramCommand(t *testing.T) {
	tests := []struct {
		input string
		name  string
		arg   string
	}{
		{input: "/login 0421", name: "/login", arg: "0421"},
		{input: "/start@visor_bot 9999", name: "/start", arg: "9999"},
		{input: "/foto Cocina", name: "/foto", arg: "Cocina"},
		{input: "hola", name: "", arg: ""},
	}
	for _, tt := range tests {
		got := parseTelegramCommand(tt.input)
		if got.Name != tt.name || got.Arg != tt.arg {
			t.Fatalf("parseTelegramCommand(%q) = %#v, want name=%q arg=%q", tt.input, got, tt.name, tt.arg)
		}
	}
}

func TestParseTelegramVideoRequest(t *testing.T) {
	tests := []struct {
		input          string
		defaultSeconds int
		camera         string
		seconds        int
		wantErr        bool
	}{
		{input: "Cocina", defaultSeconds: 8, camera: "Cocina", seconds: 8},
		{input: "Cocina 30", defaultSeconds: 8, camera: "Cocina", seconds: 30},
		{input: "Patio trasero 45", defaultSeconds: 5, camera: "Patio trasero", seconds: 45},
		{input: "Cocina 61", defaultSeconds: 5, wantErr: true},
		{input: "Cocina 2", defaultSeconds: 5, wantErr: true},
	}
	for _, tt := range tests {
		got, err := parseTelegramVideoRequest(tt.input, tt.defaultSeconds)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("parseTelegramVideoRequest(%q) expected error, got %#v", tt.input, got)
			}
			continue
		}
		if err != nil || got.CameraQuery != tt.camera || got.Seconds != tt.seconds {
			t.Fatalf("parseTelegramVideoRequest(%q) = %#v err=%v, want camera=%q seconds=%d", tt.input, got, err, tt.camera, tt.seconds)
		}
	}
}

func TestValidFourDigitCode(t *testing.T) {
	for _, valid := range []string{"0000", "1234", "9999"} {
		if !validFourDigitCode(valid) {
			t.Fatalf("expected valid code %q", valid)
		}
	}
	for _, invalid := range []string{"123", "12345", "12a4", ""} {
		if validFourDigitCode(invalid) {
			t.Fatalf("expected invalid code %q", invalid)
		}
	}
}
