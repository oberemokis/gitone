package tui

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
)

const (
	// заголовок с пустой строкой над и под ним: вплотную к краю экрана он
	// теряется
	headerHeight = 3
	footerHeight = 1
)

// header показывает путь открытого файла и счётчики строк, как заголовок
// critique: «examples.ts +6 -1», и номер файла в диффе «(2/5)»: другого места,
// где видно число файлов, на экране нет.
func header(width int, file diff.File, counter string, colors theme.Theme, text i18n.Dictionary) string {
	title := fileTitle(file, text)

	if file.Added > 0 || file.Removed > 0 {
		added := lipgloss.NewStyle().Foreground(colors.DiffAdded).Render(fmt.Sprintf("+%d", file.Added))
		removed := lipgloss.NewStyle().Foreground(colors.DiffRemoved).Render(fmt.Sprintf("-%d", file.Removed))
		title += " " + added + " " + removed
	}
	title += " " + lipgloss.NewStyle().Foreground(colors.TextMuted).Render(counter)

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

// keyHints собирает подсказки клавиш для правого края подвала. Подпись
// показывает первую клавишу действия, так переназначение видно сразу.
// Действие без клавиш в подвал не выводится.
func keyHints(colors theme.Theme, text i18n.Dictionary, bindings []keymap.Binding) string {
	muted := lipgloss.NewStyle().Foreground(colors.TextMuted)

	var hints []string
	for _, binding := range bindings {
		if !binding.Footer || len(binding.Keys) == 0 {
			continue
		}

		hints = append(hints, displayKey(binding.Keys[0])+" "+muted.Render(text.Text(footerKey(binding.Action), nil)))
	}

	return strings.Join(hints, "   ")
}

// locationLabel это ветка и приглушённое имя worktree для левого края.
func locationLabel(location git.Location, colors theme.Theme) string {
	if location.Branch == "" {
		return ""
	}

	return location.Branch + "  " + lipgloss.NewStyle().Foreground(colors.TextMuted).Render(location.Worktree)
}

// spinnerFrames это спиннер из точек Брайля, как у opencode и lazygit.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinnerStep это шаг спиннера: глаз ещё видит вращение, а перерисовок мало.
const spinnerStep = 80 * time.Millisecond

type spinMsg struct{}

func spinTick() tea.Cmd {
	return tea.Tick(spinnerStep, func(time.Time) tea.Msg { return spinMsg{} })
}

// spinner это кадр frame и надпись loading цветом добавленных строк темы.
func spinner(frame int, colors theme.Theme, text i18n.Dictionary) string {
	symbol := spinnerFrames[frame%len(spinnerFrames)]
	return lipgloss.NewStyle().Foreground(colors.DiffAdded).Render(symbol + " " + text.Text("screen.loading", nil))
}

// footerPadding это поле подвала у краёв экрана.
const footerPadding = 1

// footer раскладывает подвал на три части. Правый край главный: подсказки
// видны всегда. Центр стоит посередине экрана, а если не помещается, уходит
// левее, но правый край не задевает. Левый край уступает центру и обрезается
// первым. Текст ошибки git бывает длиннее экрана, обрезка держит подвал в одну
// строку.
func footer(width int, left, center, right string) string {
	inner := width - 2*footerPadding
	if inner <= 0 {
		return strings.Repeat(" ", max(width, 0))
	}

	right = ansi.Truncate(right, inner, "…")
	rightWidth := ansi.StringWidth(right)
	// между частями хотя бы один пробел
	room := max(inner-rightWidth-1, 0)

	center = ansi.Truncate(center, room, "…")
	centerWidth := ansi.StringWidth(center)
	centerStart := min((inner-centerWidth)/2, room-centerWidth)

	leftRoom := room
	if centerWidth > 0 {
		leftRoom = max(centerStart-1, 0)
	}
	left = ansi.Truncate(left, leftRoom, "…")
	leftWidth := ansi.StringWidth(left)

	var line strings.Builder
	line.WriteString(left)
	if centerWidth > 0 {
		line.WriteString(strings.Repeat(" ", centerStart-leftWidth))
		line.WriteString(center)
		leftWidth = centerStart + centerWidth
	}
	line.WriteString(strings.Repeat(" ", inner-leftWidth-rightWidth))
	line.WriteString(right)

	padding := strings.Repeat(" ", footerPadding)
	return padding + line.String() + padding
}
