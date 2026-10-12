package main

import "testing"

func TestIsVersionFlag(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"gitone"}, want: false},
		{args: []string{"gitone", "--version"}, want: true},
		{args: []string{"gitone", "-v"}, want: true},
		// аргументы git идут в git diff как есть и флагом версии не считаются
		{args: []string{"gitone", "--staged"}, want: false},
		{args: []string{"gitone", "--version", "main"}, want: false},
	}

	for _, tc := range tests {
		if got := isVersionFlag(tc.args); got != tc.want {
			t.Fatalf("%v: %v, ожидалось %v", tc.args, got, tc.want)
		}
	}
}

func TestVersionLine(t *testing.T) {
	if got := versionLine("1.2.3"); got != "gitone 1.2.3" {
		t.Fatalf("строка версии %q", got)
	}
}
