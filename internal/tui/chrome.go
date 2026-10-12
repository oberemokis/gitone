package tui

import (
	"cmp"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/diff"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
)

const (
	// заголовок и пустая строка под ним
	headerHeight = 2
	footerHeight = 1
)

// header показывает путь открытого файла и счётчики строк, как заголовок
// critique: «examples.ts +6-1».
func header(width int, file diff.File, colors theme.Theme, text i18n.Dictionary) string {
	title := fileTitle(file, text)

	if file.Added > 0 || file.Removed > 0 {
		added := lipgloss.NewStyle().Foreground(colors.DiffAdded).Render(fmt.Sprintf("+%d", file.Added))
		removed := lipgloss.NewStyle().Foreground(colors.DiffRemoved).Render(fmt.Sprintf("-%d", file.Removed))
		title += " " + added + removed
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
}

// fileTitle это имя файла для заголовка и списка: у переименования оба пути,
// у удалённого старый.
func fileTitle(file diff.File, text i18n.Dictionary) string {
	if file.Status == diff.StatusRenamed || file.Status == diff.StatusCopied {
		return file.OldPath + " → " + file.NewPath
	}

	return cmp.Or(file.NewPath, file.OldPath, text.Text("screen.gitOutput", nil))
}

// statusLabel пустой у изменённого файла, это обычный случай. У остальных
// подпись различает записи с одним путём: файл, ставший симлинком, приходит
// удалённым и добавленным.
func statusLabel(status diff.Status, text i18n.Dictionary) string {
	if status == diff.StatusModified {
		return ""
	}

	return text.Text(statusKey(status), nil)
}

func statusKey(status diff.Status) string {
	return "status." + string(status)
}

// footer показывает подсказки клавиш. fileCounter вида «(2/5)» встаёт у
// подсказки списка файлов: это единственное место, где видно, сколько
// файлов в диффе.
// Подпись показывает первую клавишу действия, так переназначение видно сразу.
// Действие без клавиш в подвал не выводится.
func footer(width int, colors theme.Theme, text i18n.Dictionary, fileCounter string, bindings []keymap.Binding) string {
	muted := lipgloss.NewStyle().Foreground(colors.TextMuted)

	var hints []string
	for _, binding := range bindings {
		if !binding.Footer || len(binding.Keys) == 0 {
			continue
		}

		label := text.Text(footerKey(binding.Action), nil)
		if binding.Action == keymap.FilesPick && fileCounter != "" {
			label += " " + fileCounter
		}
		hints = append(hints, displayKey(binding.Keys[0])+" "+muted.Render(label))
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, strings.Join(hints, "   "))
}

// statusFooter заменяет подсказки одной строкой. Текст ошибки git бывает
// многострочным и длиннее экрана, поэтому идёт первая строка, обрезанная по
// ширине: иначе подвал выйдет за края и сломает раскладку.
func statusFooter(width int, style lipgloss.Style, text string) string {
	first, _, _ := strings.Cut(text, "\n")
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, style.Render(ansi.Truncate(first, width, "…")))
}
