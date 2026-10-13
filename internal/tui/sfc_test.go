package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
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

// keywordColor это SGR цвета ключевого слова темы dark (#ff7b72).
const keywordColor = "38;2;255;123;114"

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

// Новую версию не прочитать: дифф показан с подсветкой по хункам, а чтение не
// повторяется на каждое сообщение.
func TestUnreadableVersionKeepsHunkHighlight(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "App.vue", vueComponent("41"))
	commitAll(t, repo)
	write(t, repo, "App.vue", vueComponent("42"))

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
	if !strings.Contains(screenText(model), "answer = 42") {
		t.Fatalf("дифф не показан:\n%s", screenText(model))
	}
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
