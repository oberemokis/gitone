package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// vueComponent это компонент, где скрипт длиннее контекста хунка: правка в
// его середине не захватывает тег <script>.
func vueComponent(answer string) string {
	return strings.Join([]string{
		"<template>",
		"  <p>{{ title }}</p>",
		"</template>",
		`<script setup lang="ts">`,
		"import { ref } from 'vue'",
		"const title = ref('x')",
		"const first = 1",
		"const second = 2",
		"const third = 3",
		"const answer = " + answer,
		"const fourth = 4",
		"const fifth = 5",
		"const sixth = 6",
		"</script>",
	}, "\n") + "\n"
}

// openSized открывает дифф каталога и доводит до конца команды после первого
// размера экрана, как Bubble Tea: среди них чтение версий файла (#140).
func openSized(t *testing.T, repo string, width, height int) tea.Model {
	t.Helper()

	model, cmd := open(t, options(repo, layout.Focus())).Update(tea.WindowSizeMsg{Width: width, Height: height})
	return settle(model, cmd, 3)
}

// keywordColor это SGR цвета ключевого слова темы dark.
var keywordColor = colorCode(38, theme.Dark().SyntaxKeyword)

// constColors это последний SGR перед каждым «const» в строке экрана, где без
// цветов написано text. Строка ищется без кодов: подсветка рвёт текст ими.
func constColors(t *testing.T, model tea.Model, text string) []string {
	t.Helper()

	row := rowOf(screenOf(model), text)
	if row < 0 {
		t.Fatalf("на экране нет %q:\n%s", text, screenText(model))
	}
	line := strings.Split(model.View().Content, "\n")[row]

	var colors []string
	for at := strings.Index(line, "const"); at >= 0; {
		codes := sgr.FindAllString(line[:at], -1)
		colors = append(colors, codes[len(codes)-1])
		next := strings.Index(line[at+1:], "const")
		if next < 0 {
			break
		}
		at += 1 + next
	}
	return colors
}

// Хунк из середины <script lang="ts"> красится как TypeScript: тега в хунке
// нет, блок берётся из файла целиком. Правка не в индексе: новая версия
// читается из рабочего дерева. В split обе версии в одной строке экрана.
func TestVueScriptHunkHighlightedFromFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "App.vue", vueComponent("41"))
	commitAll(t, repo)
	write(t, repo, "App.vue", vueComponent("42"))

	colors := constColors(t, openSized(t, repo, 140, 30), "answer = 41")
	if len(colors) != 2 {
		t.Fatalf("в строке правки %d «const», ожидалось 2 (split)", len(colors))
	}
	for side, code := range colors {
		if !strings.Contains(code, keywordColor) {
			t.Errorf("const в версии %d не цветом ключевого слова: %q", side, code)
		}
	}
}

// scriptComponent это компонент, где хунк целиком с тегом <script lang="ts">:
// и без версий файла видно, что код в нём TypeScript.
func scriptComponent(answer string) string {
	return strings.Join([]string{
		`<script setup lang="ts">`,
		"const answer = " + answer,
		"</script>",
	}, "\n") + "\n"
}

// Новую версию не прочитать: дифф показан с подсветкой по хункам, на обеих
// сторонах, а чтение не повторяется на каждое сообщение.
func TestUnreadableVersionKeepsHunkHighlight(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "App.vue", scriptComponent("41"))
	commitAll(t, repo)
	write(t, repo, "App.vue", scriptComponent("42"))

	model := open(t, options(repo, layout.Focus()))
	if err := os.Chmod(filepath.Join(repo, "App.vue"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(repo, "App.vue"), 0o644) })

	// единственная команда после размера экрана это чтение версий
	model, cmd := model.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	for _, msg := range runAll(cmd) {
		var next tea.Cmd
		if model, next = model.Update(msg); next != nil {
			t.Fatal("после неудачного чтения версий корень снова просит их прочитать")
		}
	}
	requireKeywordConsts(t, model, "answer = 41")
}

// reopenedVueRepo это репозиторий, где правка в середине компонента (тега
// <script> в хунке нет) и ещё текстовый файл: Vue идёт первым, потому что
// сортировка по пути ставит «A» перед «b».
func reopenedVueRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "App.vue", vueComponent("41"))
	write(t, repo, "b.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "App.vue", vueComponent("42"))
	write(t, repo, "b.txt", "two\n")
	return repo
}

// requireKeywordConsts проверяет, что на обеих сторонах строки с text const
// идёт цветом ключевого слова.
func requireKeywordConsts(t *testing.T, model tea.Model, text string) {
	t.Helper()

	colors := constColors(t, model, text)
	if len(colors) != 2 {
		t.Fatalf("в строке правки %d «const», ожидалось 2 (split)", len(colors))
	}
	for side, code := range colors {
		if !strings.Contains(code, keywordColor) {
			t.Errorf("const в версии %d не цветом ключевого слова: %q", side, code)
		}
	}
}

// По r файл читается заново и красится по версиям снова: подсветка по хункам,
// которой дифф заменил её, не остаётся навсегда.
func TestReloadKeepsVersionHighlight(t *testing.T) {
	model := pressReload(t, openSized(t, reopenedVueRepo(t), 140, 30))

	requireKeywordConsts(t, model, "answer = 41")
}

// Другой файл и обратно: файл раскрашен по версиям, а не остался с подсветкой
// по хункам из-за ключа версий, который помнился с прошлого раза.
func TestSwitchingFilesKeepsVersionHighlight(t *testing.T) {
	model := openSized(t, reopenedVueRepo(t), 140, 30)

	model, _ = model.Update(key(']'))
	model, _ = model.Update(key('['))

	requireKeywordConsts(t, model, "answer = 41")
}

// Версия файла нумеруется с единицы, как хунки: комментарии над правкой и под
// ней не должны красить её строку, если подсветка сдвинута на строку.
func TestVersionHighlightNumbersLinesFromOne(t *testing.T) {
	commented := func(answer string) string {
		line := "const answer = " + answer + "\n"
		return strings.Replace(vueComponent(answer), line, "// before\n"+line+"// after\n", 1)
	}
	repo := newRepo(t)
	write(t, repo, "App.vue", commented("41"))
	commitAll(t, repo)
	write(t, repo, "App.vue", commented("42"))

	requireKeywordConsts(t, openSized(t, repo, 140, 30), "answer = 41")
}

// Подсветка по версиям знает строку целиком, а дифф режет строку длиннее 4095
// рун. Комментарий на 5000 букв кончается за обрезанным текстом, а слово let
// после него начинается там: отрезки обрезают по длине строки диффа, иначе экран
// падает на `slice bounds out of range`. Цвета на месте.
func TestVersionHighlightOfLineCutByDiff(t *testing.T) {
	long := func(answer string) string {
		line := "const answer = " + answer + "\n"
		wide := "const answer = " + answer + "; /* " + strings.Repeat("x", 5000) + " */ let beyond = 1\n"
		return strings.Replace(vueComponent(answer), line, wide, 1)
	}
	repo := newRepo(t)
	write(t, repo, "App.vue", long("41"))
	commitAll(t, repo)
	write(t, repo, "App.vue", long("42"))

	requireKeywordConsts(t, openSized(t, repo, 140, 30), "answer = 41")
}

// Версия режется так же, как строка диффа: комментарий кончается 4096-й руной,
// её заменяет «…», и отрезок его подсветки не рвёт «…» пополам. Перенос по умолчанию показывает строку до конца (#394).
func TestVersionLineCutLikeDiffLine(t *testing.T) {
	long := func(answer string) string {
		line := "const answer = " + answer + "\n"
		// 19 рун до комментария, 4073 буквы и две пары знаков в нём
		wide := "const answer = " + answer + "; /*" + strings.Repeat("x", 4073) + "*/ let beyond = 1\n"
		return strings.Replace(vueComponent(answer), line, wide, 1)
	}
	repo := newRepo(t)
	write(t, repo, "App.vue", long("41"))
	commitAll(t, repo)
	write(t, repo, "App.vue", long("42"))

	model := openSized(t, repo, 200, 150)

	content := model.View().Content
	if !utf8.ValidString(content) || strings.ContainsRune(content, utf8.RuneError) {
		t.Fatalf("на экране разрезанная руна:\n%s", screenText(model))
	}
	if !strings.Contains(screenText(model), "x*…") {
		t.Fatalf("строка не кончается «…»:\n%s", screenText(model))
	}
	// комментарий в обрезанной строке не закрыт: следующие строки всё равно код
	requireKeywordConsts(t, model, "fourth = 4")
}

// colorBefore это последний SGR перед первым text в строке экрана с кодами.
func colorBefore(t *testing.T, line, text string) string {
	t.Helper()

	at := strings.Index(line, text)
	if at < 0 {
		t.Fatalf("в строке нет %q: %q", text, line)
	}
	codes := sgr.FindAllString(line[:at], -1)
	return codes[len(codes)-1]
}

// Управляющий символ в строке файла экран показывает значком из трёх байт, а
// в версии из git он один байт: подсветка по версиям должна ложиться на то же
// место, что подсветка по хункам. Цвет ключевого слова на const справа от
// значка, а не на соседних буквах, и ни одной руны, разрезанной пополам.
func TestVersionHighlightFollowsPrintableText(t *testing.T) {
	withControl := func(answer string) string {
		return strings.Replace(vueComponent(answer), "const answer", "/*\v*/ const answer", 1)
	}
	repo := newRepo(t)
	write(t, repo, "App.vue", withControl("41"))
	commitAll(t, repo)
	write(t, repo, "App.vue", withControl("42"))

	model := openSized(t, repo, 140, 30)

	row := rowOf(screenOf(model), "answer = 41")
	if row < 0 || !strings.Contains(screenOf(model)[row], "/*␋*/ const answer = 41") {
		t.Fatalf("на экране нет строки со значком ␋:\n%s", screenText(model))
	}
	for side, code := range constColors(t, model, "answer = 41") {
		if !strings.Contains(code, keywordColor) {
			t.Errorf("const в версии %d не цветом ключевого слова: %q", side, code)
		}
	}
	line := strings.Split(model.View().Content, "\n")[row]
	if strings.Contains(colorBefore(t, line, "answer"), keywordColor) {
		t.Errorf("answer цветом ключевого слова: %q", line)
	}
	if strings.ContainsRune(line, '�') {
		t.Errorf("в строке есть разрезанная руна: %q", line)
	}
}

// Файл с CRLF и комментарием на несколько строк: в версии из git у строк есть
// `\r`, в диффе его нет. Отрезок многострочного комментария доходит до конца
// строки версии, поэтому его обрезают по длине строки диффа, иначе экран падает
// на `slice bounds out of range`. Цвета на месте, значков ␍ нет.
func TestVersionHighlightWithCRLFAndMultilineComment(t *testing.T) {
	crlf := func(answer string) string {
		text := strings.Replace(vueComponent(answer), "const answer", "/* first\n   second */\nconst answer", 1)
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	repo := newRepo(t)
	write(t, repo, "App.vue", crlf("41"))
	commitAll(t, repo)
	write(t, repo, "App.vue", crlf("42"))

	model := openSized(t, repo, 140, 30)

	if !strings.Contains(screenText(model), "answer = 42") {
		t.Fatalf("дифф не показан:\n%s", screenText(model))
	}
	for side, code := range constColors(t, model, "answer = 41") {
		if !strings.Contains(code, keywordColor) {
			t.Errorf("const в версии %d не цветом ключевого слова: %q", side, code)
		}
	}
	if strings.Contains(screenText(model), "␍") {
		t.Errorf("в обычном CRLF-файле появился значок ␍:\n%s", screenText(model))
	}
}

// Новый компонент: старой версии нет, новая красится по файлу целиком.
func TestNewVueFileHighlightedFromFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "App.vue", vueComponent("42"))

	colors := constColors(t, openSized(t, repo, 140, 40), "answer = 42")
	if len(colors) == 0 || !strings.Contains(colors[0], keywordColor) {
		t.Errorf("const в новом файле не цветом ключевого слова: %q", colors)
	}
}
