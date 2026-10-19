package tui

import (
	"cmp"
	"fmt"
	"image/color"
	"slices"
	"strconv"
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
	"gitone/internal/tui/zone"
)

const (
	// заголовок с пустой строкой над и под ним: вплотную к краю экрана он
	// теряется
	headerHeight = 3
	// подвал с пустой строкой над и под ним, как заголовок (#103)
	footerHeight = 3
)

// header показывает путь открытого файла и счётчики строк, как заголовок
// critique: «examples.ts +6 -1», и номер файла в диффе «(2/5)»: другого места,
// где видно число файлов, на экране нет. У краёв стрелки к соседним файлам
// (#120), клик по ним открывает файл, как [ и ]. У края списка стрелка
// приглушена. Рядом со стрелками первые клавиши действий (#195): слева от ‹
// «предыдущий файл», справа от › «следующий», приглушённые, как подпись
// подсказки в подвале. Клавиша и стрелка одно место клика. Действие без
// клавиш оставляет стрелку без подписи. Место отдаётся имени: клавиши
// пропадают первыми, когда имя со счётчиками перестаёт помещаться. Потом
// имя обрезается «…», стрелки и счётчики остаются видны. За счётчиком
// приглушённо стоит вид диффа flags (#217), пока он не по умолчанию: иначе
// спрятанные пробелы легко принять за отсутствие правок.
func header(width int, files []diff.File, current int, flags string, colors theme.Theme, keys keymap.Keymap, text i18n.Dictionary) string {
	file := files[current]
	muted := lipgloss.NewStyle().Foreground(colors.TextMuted)

	tail := ""
	if file.Added > 0 || file.Removed > 0 {
		added := lipgloss.NewStyle().Foreground(colors.DiffAdded).Render(fmt.Sprintf("+%d", file.Added))
		removed := lipgloss.NewStyle().Foreground(colors.DiffRemoved).Render(fmt.Sprintf("-%d", file.Removed))
		tail += " " + added + " " + removed
	}
	tail += " " + muted.Render(fmt.Sprintf("(%d/%d)", current+1, len(files)))
	if flags != "" {
		tail += " " + muted.Render(flags)
	}

	// между стрелками: поля подвала и по пробелу у каждой стрелки
	inner := max(width-2*footerPadding-4, 0)
	previous, next := firstKey(keys, keymap.FilesPrevious), firstKey(keys, keymap.FilesNext)

	// клавиши остаются, пока имя целиком помещается и с ними
	name := fileTitle(file, text)
	if room := inner - keyRoom(previous) - keyRoom(next); ansi.StringWidth(name+tail) <= room {
		inner = room
	} else {
		previous, next = "", ""
	}
	name = ansi.Truncate(name, max(inner-ansi.StringWidth(tail), 1), "…")
	title := ansi.Truncate(name+tail, inner, "…")

	// клавиша стоит снаружи стрелки: «[ ‹» слева и «› ]» справа
	arrow := func(symbol, key string, index int, keyFirst bool) string {
		edge := index < 0 || index >= len(files)
		if edge {
			symbol = muted.Render(symbol)
		}

		parts := []string{symbol}
		if key != "" {
			parts = append(parts, muted.Render(key))
		}
		if keyFirst {
			slices.Reverse(parts)
		}

		button := strings.Join(parts, " ")
		if edge {
			return button
		}
		return zone.Mark(fileTarget+strconv.Itoa(index), button)
	}

	padding := strings.Repeat(" ", footerPadding)
	return padding + arrow("‹", previous, current-1, true) + " " + lipgloss.PlaceHorizontal(inner, lipgloss.Center, title) + " " + arrow("›", next, current+1, false) + padding
}

// keyRoom это место клавиши рядом со стрелкой вместе с пробелом между ними,
// у действия без клавиш ноль.
func keyRoom(key string) int {
	if key == "" {
		return 0
	}

	return ansi.StringWidth(key) + 1
}

// fileTitle это имя файла для заголовка и списка: у переименования оба пути,
// у удалённого старый. Пути в File сырые, имя из чужого репозитория может
// нести escape-коды и таб, поэтому на экран оно идёт через visible: таб
// раскрывается до измерения и обрезки, иначе строка панели и заголовка выходила
// бы шире экрана.
func fileTitle(file diff.File, text i18n.Dictionary) string {
	if file.Status == diff.StatusRenamed || file.Status == diff.StatusCopied {
		return visible(file.OldPath + " → " + file.NewPath)
	}

	return visible(cmp.Or(file.NewPath, file.OldPath, text.Text("screen.gitOutput", nil)))
}

// tabSpaces это таб в тексте на экране, как в diffview: Render раскрывает его
// в четыре пробела, а ansi.StringWidth считает нулевым.
const tabSpaces = "    "

// visible готовит поле из git к показу: управляющие символы становятся
// значками (diff.Printable), таб раскрывается до измерения и обрезки (#153).
// Без этого тема коммита или имя ветки с ESC красили бы экран, а `\r` и `\n`
// ломали строку (#177). Так проходят все поля из git, когда становятся
// ячейкой окна или текстом подвала. Значение в git остаётся сырым: имя ветки
// нужно git как есть.
func visible(field string) string {
	return strings.ReplaceAll(diff.Printable(field), "\t", tabSpaces)
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
// Действие без клавиш в подвал не выводится. Клик по подсказке нажимает её
// клавишу (#119).
func keyHints(colors theme.Theme, text i18n.Dictionary, bindings []keymap.Binding) string {
	var hints []string
	for _, binding := range bindings {
		if !binding.Footer || len(binding.Keys) == 0 {
			continue
		}

		hint := keyHint(lipgloss.NewStyle(), colors, displayKey(binding.Keys[0]), text.Text(footerKey(binding.Action), nil))
		hints = append(hints, pressable(binding.Action, hint))
	}

	return strings.Join(hints, "   ")
}

// emptyActions это действия, о которых говорит экран без изменений: окна, про
// которые подвал молчит. Ветки, справку и выход он показывает сам.
var emptyActions = []keymap.Action{
	keymap.FilesPick, keymap.AppLog, keymap.AppWorktrees, keymap.StashList,
}

// emptyHints собирает под «No changes» ряд клавиш шириной не больше width так
// же, как подвал: первая клавиша действия, переназначение видно сразу,
// действие без клавиш в ряд не идёт. Не поместившиеся подсказки уходят с
// конца, всё то же есть в справке. Если не помещается ни одна, ряд пустой.
// Клик по подсказке нажимает клавишу (#119).
func emptyHints(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, width int) string {
	var hints []string
	for _, action := range emptyActions {
		if key := firstKey(keys, action); key != "" {
			hints = append(hints, pressable(action, keyHint(lipgloss.NewStyle(), colors, key, text.Text(footerKey(action), nil))))
		}
	}

	for ; len(hints) > 0; hints = hints[:len(hints)-1] {
		if row := strings.Join(hints, "   "); lipgloss.Width(row) <= width {
			return row
		}
	}

	return ""
}

// locationLabel это ветка и worktree для левого края, каждое со своей
// клавишей окна, как подсказки справа: «B main   W gitone» (#108). Клавиша
// берётся из keymap, отключённое окно оставляет одно имя. Клик по паре
// открывает окно, как её клавиша (#119). Ветка идёт цветом accent, worktree
// цветом своей роли, клавиши цветом текста. Имена жирные (#208): рядом с
// подсказками справа они иначе сливаются с ними, а клавиши остаются обычными.
// Незавершённая операция (#243) стоит третьей парой цветом ошибки со своей
// клавишей меню: иначе о ней узнают только по отказу git, как в lazygit.
func locationLabel(location git.Location, colors theme.Theme, keys keymap.Keymap, text i18n.Dictionary) string {
	if location.Branch == "" {
		return ""
	}

	pair := func(action keymap.Action, nameColor color.Color, name string) string {
		styled := lipgloss.NewStyle().Foreground(nameColor).Bold(true).Render(name)
		if key := firstKey(keys, action); key != "" {
			return pressable(action, key+" "+styled)
		}
		return styled
	}

	label := pair(keymap.AppBranches, colors.Accent, visible(location.Branch)) + "   " + pair(keymap.AppWorktrees, colors.Worktree, visible(location.Worktree))
	if location.Operation != "" {
		label += "   " + pair(keymap.GitOperation, colors.Error, text.Text("operation."+location.Operation, nil))
	}
	return label
}

// spinnerFrames это спиннер из точек Брайля, как у opencode и lazygit.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinnerStep это шаг спиннера: глаз ещё видит вращение, а перерисовок мало.
const spinnerStep = 80 * time.Millisecond

// spinMsg это шаг спиннера загрузки номер load. Цепочка шагов принадлежит своей
// загрузке: шаг, пришедший после того, как началась следующая, отбрасывается, и
// две цепочки не крутят спиннер вдвое быстрее.
type spinMsg struct {
	load int
}

func (msg spinMsg) apply(m Model) (tea.Model, tea.Cmd) {
	if !m.loading || msg.load != m.load {
		return m, nil
	}
	m.spin++
	return m, spinTick(m.load)
}

func spinTick(load int) tea.Cmd {
	return tea.Tick(spinnerStep, func(time.Time) tea.Msg { return spinMsg{load: load} })
}

// spinner это кадр frame и надпись label приглушённым цветом: загрузка, pull и
// push идут фоном и не должны спорить с веткой и подсказками.
func spinner(frame int, colors theme.Theme, label string) string {
	symbol := spinnerFrames[frame%len(spinnerFrames)]
	return lipgloss.NewStyle().Foreground(colors.TextMuted).Render(symbol + " " + label)
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
