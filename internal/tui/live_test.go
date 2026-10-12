package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
	"gitone/internal/tui"
)

// Изменение файла на диске и сигнал из канала: экран обновляется без r.
func TestChangeSignalReloadsWithoutKey(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	changes := make(chan struct{}, 1)
	opts := options(repo, layout.Focus())
	opts.Changes = changes
	model := resize(open(t, opts), 100, 20)

	write(t, repo, "a.txt", "three\n")
	changes <- struct{}{}
	// сигнал последний: закрытый канал отпускает ожидание следующего
	close(changes)

	for _, signal := range runAll(model.Init()) {
		var next tea.Cmd
		model, next = model.Update(signal)
		for _, reply := range runAll(next) {
			model, _ = model.Update(reply)
		}
	}

	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "three") {
		t.Fatalf("после сигнала экран не обновился:\n%s", body)
	}
}

// Закрытый канал слежения сообщает о себе в подвале.
func TestStoppedWatchShowsNotice(t *testing.T) {
	changes := make(chan struct{})
	close(changes)

	opts := options("", layout.Focus())
	opts.Changes = changes
	model := resize(tui.New(opts, nil), 100, 20)

	for _, msg := range runAll(model.Init()) {
		model, _ = model.Update(msg)
	}

	if footer := screenOf(model)[len(screenOf(model))-1]; !strings.Contains(footer, "watching stopped") {
		t.Fatalf("в подвале %q, ожидалось сообщение об остановке", footer)
	}
}
