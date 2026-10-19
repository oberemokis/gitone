package diff

import (
	"fmt"
	"strings"
)

// LineRef называет изменённую строку диффа: удалённую по старому номеру,
// добавленную по новому (#212). По номерам выделение на экране находит те же
// строки в диффе, перечитанном с другим числом строк контекста.
type LineRef struct {
	Kind   LineKind
	Number int
}

// Patch собирает из текста диффа одного файла патч только с выбранными
// строками, как `git add -p`: невыбранная добавленная строка выпадает,
// невыбранная удалённая становится контекстом. Такой патч ставит выбранное в
// индекс (`git apply --cached`). reverse собирает патч для обратного
// применения к рабочему дереву (отмена строк, #213): невыбранная добавленная
// строка остаётся контекстом, невыбранная удалённая выпадает. Хунк без
// выбранных строк в патч не идёт, без выбранных строк патч пуст. Строки идут
// как в тексте git, с `\r` и управляющими символами: патч применяет git, а не
// экран.
func Patch(text string, selected map[LineRef]bool, reverse bool) string {
	var patch patchBuilder
	patch.reverse = reverse

	oldLeft, newLeft := 0, 0
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, `\`):
			patch.noNewline(line)
		case oldLeft > 0 || newLeft > 0:
			patch.line(line, selected)
			if !strings.HasPrefix(line, "+") {
				oldLeft--
			}
			if !strings.HasPrefix(line, "-") {
				newLeft--
			}
		case strings.HasPrefix(line, "@@ "):
			match := hunkHeader.FindStringSubmatch(line)
			if match == nil {
				return ""
			}
			patch.flush()
			patch.start(number(match[1], 0), number(match[3], 0))
			oldLeft, newLeft = number(match[2], 1), number(match[4], 1)
		case strings.HasPrefix(line, "diff --git "), strings.HasPrefix(line, "--- "), strings.HasPrefix(line, "+++ "):
			patch.header = append(patch.header, line)
		}
	}
	patch.flush()

	if len(patch.hunks) == 0 {
		return ""
	}
	return strings.Join(append(patch.header, patch.hunks...), "\n") + "\n"
}

// patchBuilder копит хунк патча. oldNumber и newNumber это номера следующей
// строки старой и новой стороны исходного хунка, по ним ищется выбор. shift
// это сдвиг начала хунка на другой стороне от уже собранных хунков.
type patchBuilder struct {
	reverse    bool
	header     []string
	hunks      []string
	lines      []patchLine
	oldStart   int
	newStart   int
	oldNumber  int
	newNumber  int
	oldCount   int
	newCount   int
	chosen     bool
	shift      int
	lastKept   bool
	inProgress bool
}

func (p *patchBuilder) start(oldStart, newStart int) {
	p.oldStart, p.newStart = oldStart, newStart
	p.oldNumber, p.newNumber = oldStart, newStart
	p.lines, p.oldCount, p.newCount, p.chosen = nil, 0, 0, false
	p.inProgress = true
}

// line переводит строку исходного хунка в строку патча.
func (p *patchBuilder) line(line string, selected map[LineRef]bool) {
	body := ""
	if line != "" {
		body = line[1:]
	}

	switch {
	case strings.HasPrefix(line, "+"):
		pick := selected[LineRef{Kind: LineAdded, Number: p.newNumber}]
		p.newNumber++
		switch {
		case pick:
			p.keep("+" + body)
			p.newCount++
			p.chosen = true
		case p.reverse:
			p.keep(" " + body)
			p.oldCount++
			p.newCount++
		default:
			p.lastKept = false
		}
	case strings.HasPrefix(line, "-"):
		pick := selected[LineRef{Kind: LineRemoved, Number: p.oldNumber}]
		p.oldNumber++
		switch {
		case pick:
			p.keep("-" + body)
			p.oldCount++
			p.chosen = true
		case p.reverse:
			p.lastKept = false
		default:
			p.keep(" " + body)
			p.oldCount++
			p.newCount++
		}
	default:
		p.oldNumber++
		p.newNumber++
		p.keep(" " + body)
		p.oldCount++
		p.newCount++
	}
}

// patchLine это строка патча и пометка «нет перевода строки», которая шла за
// ней в диффе.
type patchLine struct {
	text   string
	marker string
}

func (p *patchBuilder) keep(line string) {
	p.lines = append(p.lines, patchLine{text: line})
	p.lastKept = true
}

// noNewline запоминает пометку «нет перевода строки» за строкой, которая
// осталась в патче. Где она встанет, решает render.
func (p *patchBuilder) noNewline(line string) {
	if p.inProgress && p.lastKept {
		p.lines[len(p.lines)-1].marker = line
	}
}

// render пишет строки хунка. Пометка верна только за последней строкой своей
// стороны: если за строкой без перевода на той же стороне патча идут другие
// (невыбранная удалённая стала контекстом, а за ней добавленные), git apply
// склеил бы их. Тогда на этой стороне строка получает перевод, а контекст,
// чьи стороны теперь кончаются по-разному, делится на удалённую и добавленную.
func (p *patchBuilder) render() []string {
	var out []string
	for index, line := range p.lines {
		if line.marker == "" {
			out = append(out, line.text)
			continue
		}

		laterOld, laterNew := false, false
		for _, later := range p.lines[index+1:] {
			laterOld = laterOld || !strings.HasPrefix(later.text, "+")
			laterNew = laterNew || !strings.HasPrefix(later.text, "-")
		}
		body := line.text[1:]
		switch {
		case strings.HasPrefix(line.text, "-") && laterOld, strings.HasPrefix(line.text, "+") && laterNew,
			strings.HasPrefix(line.text, " ") && laterOld && laterNew:
			out = append(out, line.text)
		case strings.HasPrefix(line.text, " ") && laterNew:
			out = append(out, "-"+body, line.marker, "+"+body)
		case strings.HasPrefix(line.text, " ") && laterOld:
			out = append(out, "-"+body, "+"+body, line.marker)
		default:
			out = append(out, line.text, line.marker)
		}
	}
	return out
}

// flush кладёт хунк в патч, если в нём есть выбранные строки. Начало на той
// стороне, к которой патч применяется, остаётся исходным, на другой сдвигается
// на разницу уже собранных хунков.
func (p *patchBuilder) flush() {
	if !p.inProgress || !p.chosen {
		p.inProgress = false
		return
	}

	oldStart, newStart := p.oldStart, p.oldStart+p.shift
	if p.reverse {
		oldStart, newStart = p.newStart-p.shift, p.newStart
	}
	p.shift += p.newCount - p.oldCount

	p.hunks = append(p.hunks, fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, p.oldCount, newStart, p.newCount))
	p.hunks = append(p.hunks, p.render()...)
	p.inProgress = false
}
