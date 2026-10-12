package tui_test

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// askDiscardWithWatch открывает изменённый a.txt со слежением и задаёт вопрос
// discard. Возвращает модель, команду ожидания сигнала и канал слежения.
func askDiscardWithWatch(t *testing.T) (string, tea.Model, tea.Cmd, chan struct{}) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "old\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "new\n")

	changes := make(chan struct{}, 1)
	opts := options(repo, layout.Focus())
	opts.Changes = changes
	model := resize(open(t, opts), 100, 20)
	waiter := model.Init()

	model, _ = model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	return repo, model, waiter, changes
}

// answerYes отвечает y и выполняет то, что из этого вышло.
func answerYes(model tea.Model) tea.Model {
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	return settle(model, cmd, 4)
}

// Остановка слежения затирает вопрос в подвале, и y после неё файл не трогает.
func TestStoppedWatchCancelsDiscardQuestion(t *testing.T) {
	repo, model, waiter, changes := askDiscardWithWatch(t)

	close(changes)
	model, _ = model.Update(waiter())
	answerYes(model)

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "new\n" {
		t.Fatalf("a.txt = %q после y, want правка на месте", got)
	}
}

// Сигнал слежения запускает перечитывание, надпись loading закрывает вопрос, и
// y после этого файл не трогает.
func TestReloadCancelsDiscardQuestion(t *testing.T) {
	repo, model, waiter, changes := askDiscardWithWatch(t)

	changes <- struct{}{}
	model, _ = model.Update(waiter())
	answerYes(model)

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "new\n" {
		t.Fatalf("a.txt = %q после y, want правка на месте", got)
	}
}
