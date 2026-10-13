//go:build unix

package git_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"gitone/internal/git"
)

// hangScript это программа, на которой git зависает: стоит вместо upload-pack
// или receive-pack, сообщает о запуске через fifo started (открытие fifo ждёт
// читателя, так тест узнаёт о запуске без опроса) и ждёт на stdin слова от
// git, которого не будет. По TERM пишет файл killed и выходит. Своих детей
// скрипт не порождает: сигнал, пришедший во время fork и exec ребёнка, терялся
// бы. Скрипт это внук git pull и сын git push, до него сигнал доходит, только
// если отмена бьёт по всей группе. Иначе Pull ждёт его, пока не выйдет таймаут
// ожидания, а файла killed нет.
func hangScript(started, killed string) string {
	return fmt.Sprintf(`#!/bin/sh
trap 'echo > "%s"; exit 0' TERM
echo > "%s"
read line
echo > "%s.eof"
`, killed, started, killed)
}

// Отмена убивает git вместе с детьми, а не только его самого: git pull не
// пересылает сигнал своему fetch, и тот дотянул бы до конца в фоне.
func TestCancelStopsGitAndItsChildren(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, origin, local, script string)
		run     func(ctx context.Context, repo string) error
	}{
		{
			name: "pull",
			prepare: func(t *testing.T, origin, local, script string) {
				publish(t, origin, "b.txt", "two\n")
				runGit(t, local, "config", "remote.origin.uploadpack", script)
			},
			run: func(ctx context.Context, repo string) error {
				_, err := git.Pull(ctx, repo)
				return err
			},
		},
		{
			name: "push",
			prepare: func(t *testing.T, _, local, script string) {
				write(t, local, "b.txt", "two\n")
				commitAll(t, local)
				runGit(t, local, "config", "remote.origin.receivepack", script)
			},
			run: func(ctx context.Context, repo string) error {
				_, err := git.Push(ctx, repo)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origin, local := remotePair(t)
			dir := t.TempDir()
			started, killed := filepath.Join(dir, "started"), filepath.Join(dir, "killed")
			if err := syscall.Mkfifo(started, 0o600); err != nil {
				t.Fatal(err)
			}
			script := filepath.Join(dir, "hang.sh")
			if err := os.WriteFile(script, []byte(hangScript(started, killed)), 0o755); err != nil {
				t.Fatal(err)
			}
			tc.prepare(t, origin, local, script)
			before := revision(t, local, "HEAD")
			originBefore := revision(t, origin, "main")

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			done := make(chan error, 1)
			go func() { done <- tc.run(ctx, local) }()

			running := make(chan struct{})
			go func() {
				if fifo, err := os.Open(started); err == nil {
					defer fifo.Close()
					fifo.Read(make([]byte, 1))
					close(running)
				}
			}()
			select {
			case <-running:
			case err := <-done:
				t.Fatalf("команда закончилась до запуска зависшей программы: %v", err)
			}

			cancel()
			err := <-done

			if !errors.Is(err, context.Canceled) {
				t.Errorf("ошибка %v, ожидалась отмена context.Canceled", err)
			}
			if !strings.HasPrefix(err.Error(), tc.name+": ") {
				t.Errorf("ошибка %q, ожидалось имя операции %q в начале", err, tc.name)
			}
			if _, statErr := os.Stat(killed); statErr != nil {
				t.Errorf("зависшая программа не получила TERM: %v", statErr)
			}
			if _, statErr := os.Stat(filepath.Join(local, ".git", "index.lock")); !errors.Is(statErr, fs.ErrNotExist) {
				t.Errorf("остался index.lock: %v", statErr)
			}
			if got := revision(t, local, "HEAD"); got != before {
				t.Errorf("HEAD %s, ожидался прежний %s", got, before)
			}
			if got := revision(t, origin, "main"); got != originBefore {
				t.Errorf("main на origin %s, ожидался прежний %s", got, originBefore)
			}
		})
	}
}
