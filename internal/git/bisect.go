package git

import (
	"regexp"
	"strconv"
	"strings"
)

// BisectStep это итог отметки в bisect (#245). Found значит, что первый
// плохой коммит найден: Commit и Subject называют его. Иначе Commit это
// коммит, на который git перешёл для проверки, и Left сколько коммитов
// осталось проверить; пустой Commit значит, что git ждёт вторую отметку.
type BisectStep struct {
	Found   bool
	Commit  string
	Subject string
	Left    int
}

var (
	// «Bisecting: 3 revisions left to test after this (roughly 2 steps)»
	bisectLeft = regexp.MustCompile(`(?m)^Bisecting: (\d+) revisions? left`)
	// «[<хэш>] <тема>» после строки Bisecting
	bisectAt = regexp.MustCompile(`(?m)^\[([0-9a-f]+)\] (.*)$`)
	// «<хэш> is the first bad commit», новый git берёт bad в кавычки
	bisectFound = regexp.MustCompile(`(?m)^([0-9a-f]+) is the first '?bad'? commit$`)
)

// BisectMark отмечает коммит хорошим или плохим, как b в логе lazygit. Первая
// отметка начинает bisect. Когда отмечены обе стороны, git переходит на
// следующий коммит для проверки: грязное дерево он не трогает и отказывает
// той же причиной, что checkout.
func BisectMark(dir, hash string, good bool) (BisectStep, error) {
	root, err := topLevel(dir)
	if err != nil {
		return BisectStep{}, failed("bisect", err)
	}
	if !bisecting(root) {
		if _, err := run(call{dir: root}, "bisect", "start"); err != nil {
			return BisectStep{}, failed("bisect", headline(err))
		}
	}

	term := "bad"
	if good {
		term = "good"
	}
	out, err := run(call{dir: root}, "bisect", term, hash)
	if err != nil {
		return BisectStep{}, failed("bisect", headline(err))
	}
	return bisectStep(root, out), nil
}

// bisectStep разбирает вывод git bisect good и bad.
func bisectStep(root, out string) BisectStep {
	if match := bisectFound.FindStringSubmatch(out); match != nil {
		subject, _ := run(call{dir: root}, "log", "-1", "--format=%s", match[1])
		return BisectStep{Found: true, Commit: match[1], Subject: strings.TrimSpace(subject)}
	}

	var step BisectStep
	if match := bisectLeft.FindStringSubmatch(out); match != nil {
		step.Left, _ = strconv.Atoi(match[1])
	}
	if match := bisectAt.FindStringSubmatch(out); match != nil {
		step.Commit, step.Subject = match[1], match[2]
	}
	return step
}

// BisectReset заканчивает bisect: HEAD возвращается туда, где был до старта.
func BisectReset(dir string) error {
	root, err := topLevel(dir)
	if err != nil {
		return failed("bisect", err)
	}
	_, err = run(call{dir: root}, "bisect", "reset")
	return failed("bisect", headline(err))
}

// bisecting говорит, что идёт bisect: git держит его начало в BISECT_START.
func bisecting(root string) bool {
	return gitPathExists(root, "BISECT_START")
}
