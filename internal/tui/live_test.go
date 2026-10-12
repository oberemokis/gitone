package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/keymap"
	"gitone/internal/layout"
	"gitone/internal/tui"
	"gitone/internal/tui/theme"
)

// runAll выполняет команду и её части, если она пакетная. Части, которые
// блокируются без сигнала, отбрасываются по времени.
func runAll(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	results := make(chan tea.Msg, 8)
	go func() {
		msg := cmd()
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			results <- msg
			return
		}
		for _, sub := range batch {
			go func(sub tea.Cmd) { results <- sub() }(sub)
		}
	}()

	var msgs []tea.Msg
	for {
		select {
		case msg := <-results:
			msgs = append(msgs, msg)
		case <-time.After(500 * time.Millisecond):
			return msgs
		}
	}
}

// Изменение файла на диске и сигнал из канала: экран обновляется без r.
func TestChangeSignalReloadsWithoutKey(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	changes := make(chan struct{}, 1)
	files, err := tui.Load(repo, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	opts := tui.Options{
		Dir: repo, Config: config.Default(), Keys: keymap.Default(), Theme: theme.Dark(),
		Layout: layout.Focus(), Changes: changes,
	}
	var model tea.Model = tui.New(opts, files)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	write(t, repo, "a.txt", "three\n")
	changes <- struct{}{}

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

	opts := tui.Options{Config: config.Default(), Keys: keymap.Default(), Theme: theme.Dark(), Changes: changes}
	var model tea.Model = tui.New(opts, nil)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	for _, msg := range runAll(model.Init()) {
		model, _ = model.Update(msg)
	}

	if footer := screenOf(model)[len(screenOf(model))-1]; !strings.Contains(footer, "слежение остановлено") {
		t.Fatalf("в подвале %q, ожидалось сообщение об остановке", footer)
	}
}
