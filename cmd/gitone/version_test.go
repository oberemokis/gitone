package main

import "testing"

func TestIsVersionFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "без аргументов", args: []string{"gitone"}, want: false},
		{name: "--version", args: []string{"gitone", "--version"}, want: true},
		{name: "-v", args: []string{"gitone", "-v"}, want: true},
		// аргументы git идут в git diff как есть и флагом версии не считаются
		{name: "аргумент git", args: []string{"gitone", "--staged"}, want: false},
		{name: "флаг не один", args: []string{"gitone", "--version", "main"}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isVersionFlag(tc.args); got != tc.want {
				t.Errorf("isVersionFlag(%q) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

// Формат «gitone X.Y.Z» это контракт с `update` прошлых версий: тест держит его
// литералом, а не выводит из кода.
func TestVersionLineIsContractWithOlderUpdaters(t *testing.T) {
	if got := versionLine("1.2.3"); got != "gitone 1.2.3" {
		t.Errorf("versionLine(%q) = %q, want %q", "1.2.3", got, "gitone 1.2.3")
	}
}
