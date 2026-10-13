package tui_test

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/i18n"
	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// kindTheme красит каждый вид токенов своим цветом, от #0b0b0b у ключевого
// слова до #141414 у тега: в ANSI 38;2;11;11;11 … 38;2;20;20;20. Так по цвету
// перед токеном видно, какой вид ему выдали.
func kindTheme() theme.Theme {
	palette := theme.Dark()
	palette.SyntaxKeyword = lipgloss.Color("#0b0b0b")
	palette.SyntaxString = lipgloss.Color("#0c0c0c")
	palette.SyntaxComment = lipgloss.Color("#0d0d0d")
	palette.SyntaxNumber = lipgloss.Color("#0e0e0e")
	palette.SyntaxFunction = lipgloss.Color("#0f0f0f")
	palette.SyntaxType = lipgloss.Color("#101010")
	palette.SyntaxConstant = lipgloss.Color("#111111")
	palette.SyntaxProperty = lipgloss.Color("#121212")
	palette.SyntaxOperator = lipgloss.Color("#131313")
	palette.SyntaxTag = lipgloss.Color("#141414")
	return palette
}

const (
	keywordFg  = "38;2;11;11;11"
	stringFg   = "38;2;12;12;12"
	commentFg  = "38;2;13;13;13"
	numberFg   = "38;2;14;14;14"
	functionFg = "38;2;15;15;15"
	typeFg     = "38;2;16;16;16"
	constantFg = "38;2;17;17;17"
	propertyFg = "38;2;18;18;18"
	operatorFg = "38;2;19;19;19"
	tagFg      = "38;2;20;20;20"
)

// Каждый вид токена красится своим цветом темы: в строке экрана перед токеном
// стоит цвет его вида, а не соседнего.
func TestSyntaxKindsUseOwnColors(t *testing.T) {
	goSource := strings.Join([]string{
		"package main",
		"",
		"// Load читает дифф",
		"func (m Model) View() string {",
		`	x := fmt.Sprintf("%d", m.count, 1337)`,
		"	return readDiff(x), nil",
		"}",
	}, "\n") + "\n"
	vueSource := "<template>\n  <div>hi</div>\n</template>\n"

	tests := []struct {
		name, path, old, new, marker, token, want string
	}{
		{name: "комментарий", path: "main.go", old: "package main\n", new: goSource, marker: "// Load", token: "// Load", want: commentFg},
		{name: "ключевое слово", path: "main.go", old: "package main\n", new: goSource, marker: "Model", token: "func", want: keywordFg},
		{name: "тип", path: "main.go", old: "package main\n", new: goSource, marker: "string", token: "string", want: typeFg},
		{name: "строка", path: "main.go", old: "package main\n", new: goSource, marker: `"%d"`, token: `"%d"`, want: stringFg},
		{name: "число", path: "main.go", old: "package main\n", new: goSource, marker: "1337", token: "1337", want: numberFg},
		{name: "оператор", path: "main.go", old: "package main\n", new: goSource, marker: ":=", token: ":=", want: operatorFg},
		{name: "функция", path: "main.go", old: "package main\n", new: goSource, marker: "readDiff", token: "readDiff", want: functionFg},
		{name: "константа", path: "main.go", old: "package main\n", new: goSource, marker: "nil", token: "nil", want: constantFg},
		{name: "свойство", path: "main.go", old: "package main\n", new: goSource, marker: "count", token: "count", want: propertyFg},
		{name: "тег", path: "App.vue", old: "x\n", new: vueSource, marker: "div", token: "div", want: tagFg},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := rawWith(t, tc.path, tc.old, tc.new, kindTheme())

			if got := styleBefore(t, raw, tc.marker, tc.token); !strings.Contains(got, tc.want) {
				t.Errorf("перед %q стоит %q, want цвет %s", tc.token, got, tc.want)
			}
		})
	}
}

// Удалённая строка красится по старой стороне хунка, добавленная по новой: у
// соседних строк другой вид (комментарии), и подсветка, сдвинутая на строку в
// любую сторону, дала бы слову func не цвет ключевого слова.
func TestUnifiedColorsEachLineByItsOwnSide(t *testing.T) {
	old := "package main\n\n// head\nfunc oldName() {}\n// tail\n"
	new := "package main\n\n// head\nfunc newName() {}\n// tail\n"

	raw := rawWith(t, "main.go", old, new, kindTheme())

	for _, marker := range []string{"oldName", "newName"} {
		if got := styleBefore(t, raw, marker, "func"); !strings.Contains(got, keywordFg) {
			t.Errorf("func в строке с %s: %q, want цвет ключевого слова %s", marker, got, keywordFg)
		}
	}
}

// Сторона хунка разбирается целиком, поэтому многострочный комментарий красится
// цветом комментария во всех трёх строках, а не только в первой: по строке
// «second» и «third */» tree-sitter комментария не увидел бы.
func TestMultilineCommentIsColoredOnEveryLine(t *testing.T) {
	source := func(name string) string {
		return "package main\n\n/* first\n   second\n   third */\nfunc " + name + "() {}\n"
	}

	raw := rawWith(t, "main.go", source("old"), source("fresh"), kindTheme())

	for _, token := range []string{"first", "second", "third"} {
		if got := styleBefore(t, raw, token, token); !strings.Contains(got, commentFg) {
			t.Errorf("перед %q стоит %q, want цвет комментария %s", token, got, commentFg)
		}
	}
}

// В split левая половина красится подсветкой старой стороны, правая новой:
// слева ключевое слово, справа комментарий.
func TestSplitColorsHalvesBySide(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "main.go", "package main\n\n// head\nfunc oldName() {}\n// tail\n")
	commitAll(t, repo)
	write(t, repo, "main.go", "package main\n\n// head\n// newName\n// tail\n")

	opts := options(repo, layout.Focus())
	opts.Theme = kindTheme()
	opts.Config.Diff.Mode = "split"
	raw := resize(open(t, opts), 120, 20).View().Content

	if got := styleBefore(t, raw, "oldName", "func"); !strings.Contains(got, keywordFg) {
		t.Errorf("func слева: %q, want цвет ключевого слова %s", got, keywordFg)
	}
	if got := styleBefore(t, raw, "newName", "// newName"); !strings.Contains(got, commentFg) {
		t.Errorf("комментарий справа: %q, want цвет комментария %s", got, commentFg)
	}
}

// bgTheme даёт строкам отличимые фоны: добавленная #060606 (48;2;6;6;6),
// удалённая #080808 (48;2;8;8;8).
func bgTheme() theme.Theme {
	palette := theme.Dark()
	palette.DiffAddedBg = lipgloss.Color("#060606")
	palette.DiffRemovedBg = lipgloss.Color("#080808")
	return palette
}

// Фон строки идёт до правого края панели, а не обрывается на конце текста.
func TestCodePaddingKeepsLineBackground(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "keep\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "keep\nnew line\n")

	opts := options(repo, layout.Focus())
	opts.Theme = bgTheme()
	opts.Config.Diff.Mode = "unified"
	raw := resize(open(t, opts), 100, 20).View().Content

	cells := cellBackgrounds(rawLine(t, raw, "new line"))
	if len(cells) != 100 {
		t.Fatalf("в строке %d клеток, want 100", len(cells))
	}
	// пять клеток слева это номер и знак на своём фоне
	if got := slices.Compact(slices.Clone(cells[5:])); !slices.Equal(got, []string{"6;6;6"}) {
		t.Errorf("фон кода и добивки %q, want один фон строки 6;6;6", got)
	}
}

// У половины split без строки нет ни текста, ни фона: видно, с какой стороны
// строки нет. Удалённая строка без пары красит только левую половину.
func TestEmptySplitHalfHasNoBackground(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "keep\ngone\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "keep\n")

	opts := options(repo, layout.Focus())
	opts.Theme = bgTheme()
	opts.Config.Diff.Mode = "split"
	raw := resize(open(t, opts), 100, 20).View().Content

	cells := cellBackgrounds(rawLine(t, raw, "gone"))
	if len(cells) != 100 {
		t.Fatalf("в строке %d клеток, want 100", len(cells))
	}
	if got := slices.Compact(slices.Clone(cells[5:50])); !slices.Equal(got, []string{"8;8;8"}) {
		t.Errorf("фон левой половины после номера %q, want 8;8;8", got)
	}
	if got := slices.Compact(slices.Clone(cells[50:])); !slices.Equal(got, []string{""}) {
		t.Errorf("фон пустой правой половины %q, want без фона", got)
	}
}

// Сдвиг вбок пропускает куски строки целиком: слово «GAMMA» с фоном слова
// идёт отдельным куском, и колонки следующего за ним кода считаются от
// ширины всех пропущенных.
func TestShiftSkipsWholePieces(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "alpha beta gamma delta epsilon zeta eta theta iota\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "alpha beta GAMMA delta epsilon zeta eta theta iota\n")

	model := openView(t, repo, "unified", 25, noWrap)
	model, _ = model.Update(right)
	model, _ = model.Update(right)

	// код с колонки 16: пропущены «alpha beta » (11 колонок) и «GAMMA» (5)
	want := []string{" 1 -  delta epsilon zeta", " 1 +  delta epsilon zeta"}
	if got := bodyRows(model, len(want)); !slices.Equal(got, want) {
		t.Errorf("строки диффа %q, want %q", got, want)
	}
}

// Сырой блок git (разбор его не узнал) идёт цветом текста, как код, а не
// приглушённым цветом служебных надписей.
func TestRawBlockIsNotMuted(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "y\n")

	custom := theme.Dark()
	custom.TextMuted = lipgloss.Color("#030303")
	opts := options(repo, layout.Focus())
	opts.Theme = custom
	opts.Args = []string{"--stat"}
	raw := resize(open(t, opts), 100, 20).View().Content

	if got := styleBefore(t, raw, "| 2 +-", "a.txt"); strings.Contains(got, mutedFg) {
		t.Errorf("перед началом строки статистики %q, want без приглушённого цвета %s", got, mutedFg)
	}
}

// Служебная надпись над файлом (бинарный файл) приглушена.
func TestNoticeLineIsMuted(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.bin", "x\x00y\n")
	commitAll(t, repo)
	write(t, repo, "a.bin", "z\x00w\n")

	custom := theme.Dark()
	custom.TextMuted = lipgloss.Color("#030303")
	opts := options(repo, layout.Focus())
	opts.Theme = custom
	raw := resize(open(t, opts), 100, 20).View().Content

	if got := styleBefore(t, raw, "Binary", "Binary"); !strings.Contains(got, mutedFg) {
		t.Errorf("перед служебной строкой %q, want приглушённый цвет %s", got, mutedFg)
	}
}

// Таб в служебной надписи становится четырьмя пробелами до обрезки и добивки,
// как в коде: Lip Gloss раскрыл бы его сам при отрисовке, и строка вылезла бы
// за панель на три колонки.
func TestNoticeExpandsTabs(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.bin", "x\x00y\n")
	commitAll(t, repo)
	write(t, repo, "a.bin", "z\x00w\n")

	opts := options(repo, layout.Focus())
	opts.Text = i18n.Custom("tabbed", map[string]string{"diff.binary": "bin\tfile"})
	screen := screenOf(resize(open(t, opts), 100, 20))

	row := rowOf(screen, "bin    file")
	if row < 0 {
		t.Fatalf("на экране нет «bin    file»:\n%s", strings.Join(screen, "\n"))
	}
	if width := ansi.StringWidth(screen[row]); width != 100 {
		t.Errorf("строка с табом шириной %d колонок, want 100", width)
	}
}
