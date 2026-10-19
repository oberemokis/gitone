package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/i18n"
)

// runMainEnv включает в тестовом бинарнике настоящий main: дочерний процесс
// запускает его, как пользователь запускает gitone, и тест читает stderr и код.
const runMainEnv = "GITONE_TEST_RUN_MAIN"

func TestMain(m *testing.M) {
	if os.Getenv(runMainEnv) == "1" {
		main()
		return
	}

	os.Exit(m.Run())
}

// Ошибка git, которая печатается до запуска интерфейса, идёт в терминал
// пользователя мимо границы показа в tui: в stderr значок вместо ESC,
// последовательность не красит терминал. Многострочный текст git остаётся
// многострочным. git подставной: настоящий и сам заменяет управляющие символы в
// своих отказах, а текст хука или старой версии нет.
func TestMainPrintsGitErrorWithoutControlCharacters(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf 'fatal: \\033[31mboom\\033[0m\\nhint: one\\rtwo\\n' >&2\nexit 128\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0])
	command.Dir = t.TempDir()
	command.Env = append(os.Environ(), runMainEnv+"=1", "PATH="+bin, "HOME="+command.Dir, "XDG_CONFIG_HOME="+command.Dir)
	var stderr bytes.Buffer
	command.Stderr = &stderr

	err := command.Run()

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("gitone: %v, want выход с кодом 1", err)
	}
	// подставной git отказывает уже на поиске корня репозитория (git.Open)
	if want := "rev-parse: fatal: ␛[31mboom␛[0m\nhint: one␍two\n"; !strings.HasSuffix(stderr.String(), want) {
		t.Errorf("stderr = %q, want окончание %q", stderr.String(), want)
	}
}

func TestAddNotice(t *testing.T) {
	text := i18n.Custom("xx", map[string]string{"json.extraText": "лишний текст"})

	tests := []struct {
		name   string
		notice string
		errs   []error
		want   string
	}{
		{name: "нет ошибок", notice: "", errs: []error{nil, nil}, want: ""},
		{name: "nil пропускается", notice: "", errs: []error{nil, errors.New("тема"), nil}, want: "тема"},
		{name: "к тексту через точку с запятой", notice: "config.json", errs: []error{errors.New("раскладка")}, want: "config.json; раскладка"},
		{name: "текст без ошибок остаётся", notice: "config.json", errs: []error{nil}, want: "config.json"},
		{name: "своя ошибка на языке словаря", errs: []error{fmt.Errorf("config.json: %w", i18n.New("json.extraText", nil))}, want: "config.json: лишний текст"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := addNotice(text, tc.notice, tc.errs...); got != tc.want {
				t.Errorf("addNotice(%q, %v) = %q, want %q", tc.notice, tc.errs, got, tc.want)
			}
		})
	}
}
