package syntax

import (
	"bytes"

	ts "github.com/odvcencio/gotreesitter"
)

// Код однофайловых компонентов (#140). Грамматика разметки видит в <script>,
// <style> и выражениях шаблона только текст. Такой узел разбирает своя
// грамматика, и её цвета целиком заменяют цвета разметки под узлом: строковый
// цвет значения директивы не остаётся под именами переменных.

// embedders это языки разметки, внутри которых лежит код. По хунку не понять,
// из какого он блока, поэтому им нужен файл целиком.
var embedders = map[string]bool{"vue": true, "svelte": true, "astro": true}

// Embeds говорит, что внутри файла лежит код на других языках и подсветке
// нужен файл целиком, а не хунк.
func (l Lexer) Embeds() bool {
	return l.entry != nil && embedders[l.entry.Name]
}

// place это где в дереве разметки стоит узел: тип родителя, внутри ли он
// директивы vue, после ли as в {#each} Svelte и атрибут lang у script или
// style, внутри которого стоит узел.
type place struct {
	parent    string
	directive bool
	binding   bool
	lang      string
}

// embeddedFile это имя, по которому Detect выберет язык кода в узле kind, или
// пустая строка, если в узле разметка. Выражения шаблона разбираются как
// TypeScript: он принимает и JavaScript.
func embeddedFile(markup, kind string, at place) string {
	switch {
	case kind == "raw_text" && at.parent == "script_element":
		return scriptFile(markup, at.lang)
	case kind == "raw_text" && at.parent == "style_element":
		if at.lang == "scss" {
			return "embedded.scss"
		}
		return "embedded.css"
	case markup == "vue" && kind == "raw_text" && at.parent == "interpolation",
		markup == "vue" && kind == "attribute_value" && at.directive,
		markup == "svelte" && kind == "svelte_raw_text",
		markup == "astro" && (kind == "frontmatter_js_block" || kind == "attribute_js_expr"):
		return "embedded.ts"
	}
	return ""
}

// scriptFile это язык <script> по атрибуту lang. У Astro скрипт всегда
// TypeScript.
func scriptFile(markup, lang string) string {
	switch {
	case markup == "astro" || lang == "ts":
		return "embedded.ts"
	case lang == "tsx":
		return "embedded.tsx"
	}
	return "embedded.js"
}

// inject перекрашивает код внутри разметки его собственной грамматикой.
func (l Lexer) inject(root *ts.Node, language *ts.Language, source []byte, kinds []Kind) {
	var walk func(node *ts.Node, at place)
	walk = func(node *ts.Node, at place) {
		kind := node.Type(language)
		if file := embeddedFile(l.entry.Name, kind, at); file != "" {
			start, end := int(node.StartByte()), int(node.EndByte())
			if at.binding {
				from, to := eachKey(source[start:end])
				start, end = start+from, start+to
			}
			if start < end {
				if inner := Detect(file).paint(source[start:end]); inner != nil {
					copy(kinds[start:end], inner)
				}
			}
			return
		}

		switch kind {
		case "script_element", "style_element":
			at.lang = langAttribute(node, language, source)
		case "directive_attribute":
			at.directive = true
		}
		at.parent = kind
		for index := range node.ChildCount() {
			child := node.Child(index)
			walk(child, at)
			if kind == "each_start" && child.Type(language) == "as" {
				at.binding = true
			}
		}
	}
	walk(root, place{})
}

// eachKey это границы ключа в тексте после as в {#each}: из «item, i
// (item.id)» получается «item.id». Привязка и индекс до скобок кодом не
// разбираются: «item (item.id)» как TypeScript это вызов функции item. Без
// ключа границы пустые. Скобки внутри строк не учитываются: строка в привязке
// бывает только значением по умолчанию в деструктуризации.
func eachKey(text []byte) (from, to int) {
	text = bytes.TrimRight(text, " \t\r\n")
	depth, open := 0, 0
	for at, char := range text {
		switch char {
		case '(', '[', '{':
			if depth == 0 && char == '(' {
				open = at + 1
			}
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 && char == ')' && at == len(text)-1 {
				return open, at
			}
		}
	}
	return 0, 0
}

// langAttribute это значение атрибута lang в открывающем теге элемента:
// <script lang="ts"> и <script lang=ts> дают «ts». Без атрибута или без
// значения пустая строка.
func langAttribute(element *ts.Node, language *ts.Language, source []byte) string {
	text := func(node *ts.Node) string { return string(source[node.StartByte():node.EndByte()]) }
	child := func(node *ts.Node, kind string) *ts.Node {
		for index := range node.ChildCount() {
			if found := node.Child(index); found.Type(language) == kind {
				return found
			}
		}
		return nil
	}

	tag := child(element, "start_tag")
	if tag == nil {
		return ""
	}
	for index := range tag.ChildCount() {
		attribute := tag.Child(index)
		if attribute.Type(language) != "attribute" {
			continue
		}
		if name := child(attribute, "attribute_name"); name == nil || text(name) != "lang" {
			continue
		}
		value := child(attribute, "attribute_value")
		if quoted := child(attribute, "quoted_attribute_value"); quoted != nil {
			value = child(quoted, "attribute_value")
		}
		if value != nil {
			return text(value)
		}
	}
	return ""
}
