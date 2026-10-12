package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"gitone/internal/diff"
	"gitone/internal/tui/theme"
)

const (
	// заголовок и пустая строка под ним
	headerHeight = 2
	footerHeight = 1
)

// header показывает путь открытого файла и счётчики строк, как заголовок
// critique: «examples.ts +6-1».
func header(width int, file diff.File, colors theme.Theme) string {
	title := fileTitle(file)

	if file.Added > 0 || file.Removed > 0 {
		added := lipgloss.NewStyle().Foreground(colors.DiffAdded).Render(fmt.Sprintf("+%d", file.Added))
		removed := lipgloss.NewStyle().Foreground(colors.DiffRemoved).Render(fmt.Sprintf("-%d", file.Removed))
		title += " " + added + removed
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
}

// fileTitle это имя файла для заголовка и списка: у переименования оба пути,
// у удалённого старый.
func fileTitle(file diff.File) string {
	switch {
	case file.Status == diff.StatusRenamed || file.Status == diff.StatusCopied:
		return file.OldPath + " → " + file.NewPath
	case file.NewPath != "":
		return file.NewPath
	case file.OldPath != "":
		return file.OldPath
	}

	return "git output"
}

// statusLabel пустой у изменённого файла, это обычный случай. У остальных
// подпись различает записи с одним путём: файл, ставший симлинком, приходит
// удалённым и добавленным.
func statusLabel(status diff.Status) string {
	if status == diff.StatusModified {
		return ""
	}

	return string(status)
}

// footer показывает подсказки клавиш. fileCounter вида «(2/5)» встаёт у
// подсказки списка файлов: это единственное место, где видно, сколько
// файлов в диффе.
func footer(width int, colors theme.Theme, fileCounter string) string {
	muted := lipgloss.NewStyle().Foreground(colors.TextMuted)

	var hints []string
	for _, binding := range bindings() {
		if binding.label == "" {
			continue
		}

		label := binding.label
		if binding.action == actionFiles && fileCounter != "" {
			label += " " + fileCounter
		}
		hints = append(hints, displayKey(binding.keys[0])+" "+muted.Render(label))
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, strings.Join(hints, "   "))
}
