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
