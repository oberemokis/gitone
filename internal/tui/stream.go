package tui

import (
	"slices"

	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/i18n"
)

// Лента всех файлов (#373) живёт в diffview: он держит страницы файлов и
// прокрутку, и текущим в ленте становится файл, чья строка наверху. Корень
// включает ленту клавишей a, отдаёт ей перечитанный дифф и после каждого
// сообщения берёт из неё текущий файл: на него действуют space, x, e, h, i и
// заголовок экрана.

// source это откуда дифф: репозиторий и аргументы git diff. Флаги вида
// (пробелы, контекст) показывают тот же дифф.
type source struct {
	repo git.Repo
	args []string
}

func (s source) same(other source) bool {
	return s.repo == other.repo && slices.Equal(s.args, other.args)
}

// followStream раскрашивает видимые файлы ленты и берёт из неё текущий файл.
func (m Model) followStream() Model {
	if m.view.Streaming() {
		m.view = m.view.Prepare()
		m.current = m.view.Current()
	}
	return m
}

// toggleAllFiles включает и выключает ленту. Выделение живёт в одном файле и
// снимается до переключения.
func (m Model) toggleAllFiles() Model {
	if m.view.Selecting() {
		m = m.stopSelect()
	}
	if m.view.Streaming() {
		m.view = m.view.ShowOne()
		return m
	}

	m.view = m.view.ShowAll(m.files, titlesOf(m.files, m.text), m.current)
	return m
}

// backFile это файл, который открывают [ и левая стрелка заголовка:
// предыдущий, а в ленте из середины файла сам этот файл, как [[ в vim.
func (m Model) backFile() int {
	if m.view.AtFileStart() {
		return m.current - 1
	}
	return m.current
}

// reloadStream ставит перечитанный дифф в ленту. Открытый файл kept остаётся
// наверху на той же строке. Пропавший файл того же диффа сменяет следующий за
// ним, а у последнего предыдущий. Другой дифф (лог, stash, b, другой
// worktree) открывается с начала ленты, даже если открытый файл есть и в нём:
// его строка в другом диффе это другое место.
func (m Model) reloadStream(files []diff.File, kept int, sameDiff bool) Model {
	keep := kept
	switch {
	case len(files) == 0:
		keep = -1
	case !sameDiff:
		keep = 0
	case kept < 0:
		keep = m.neighbour(files)
	}

	m.view = m.view.ReloadAll(files, titlesOf(files, m.text), keep, kept >= 0 && sameDiff)
	m.files, m.current = files, max(keep, 0)
	return m
}

// neighbour это место в files соседа пропавшего открытого файла: первого
// следующего за ним, который остался, а без такого ближайшего предыдущего.
func (m Model) neighbour(files []diff.File) int {
	if m.current >= len(m.files) {
		return 0
	}

	for _, file := range m.files[m.current+1:] {
		if index := m.indexIn(files, file); index >= 0 {
			return index
		}
	}
	for _, file := range slices.Backward(m.files[:m.current]) {
		if index := m.indexIn(files, file); index >= 0 {
			return index
		}
	}
	return 0
}

// titlesOf это подписи файлов для заголовков ленты.
func titlesOf(files []diff.File, text i18n.Dictionary) []string {
	titles := make([]string, len(files))
	for index, file := range files {
		titles[index] = fileTitle(file, text)
	}
	return titles
}
