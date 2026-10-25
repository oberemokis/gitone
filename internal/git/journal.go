package git

import (
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// JournalEntry это запись журнала команд (#247): когда, с какими аргументами
// и с каким кодом выхода git отработал. Exit 0 значит успех.
type JournalEntry struct {
	When time.Time
	Args []string
	Exit int
}

// journalSize это сколько последних записей помнит журнал.
// ponytail: число зашито, настройкой станет, если 200 кому-то не хватит.
const journalSize = 200

// journal это общий журнал всех вызовов run: run идёт из команд Bubble Tea в
// разных горутинах, поэтому записи под мьютексом. Это третье исключение из
// правила «нет общих изменяемых структур» (docs/architecture.md): журнал
// пишет единая точка запуска git, а провести его через каждую функцию пакета
// значило бы поменять их все.
var journal struct {
	sync.Mutex
	entries []JournalEntry
}

// Journal возвращает записи журнала от новых к старым.
func Journal() []JournalEntry {
	journal.Lock()
	defer journal.Unlock()

	entries := slices.Clone(journal.entries)
	slices.Reverse(entries)
	return entries
}

// remember пишет вызов в журнал. Чтение не пишется: опрос раз в две секунды
// читает дифф, индекс и ветку, и за минуту вытеснил бы из журнала всё, что
// сделал пользователь. Адрес с логином или паролем маскируется, окружение в
// журнал не попадает вовсе.
func remember(args []string, exit int) {
	if readOnly(args) {
		return
	}

	entry := JournalEntry{When: time.Now(), Args: make([]string, len(args)), Exit: exit}
	for index, arg := range args {
		entry.Args[index] = credentials.ReplaceAllString(arg, "://***@")
	}

	journal.Lock()
	defer journal.Unlock()
	journal.entries = append(journal.entries, entry)
	if extra := len(journal.entries) - journalSize; extra > 0 {
		journal.entries = slices.Delete(journal.entries, 0, extra)
	}
}

// credentials это логин и пароль в адресе: https://user:token@host.
var credentials = regexp.MustCompile(`://[^/@\s]+@`)

// readOnly говорит, что вызов ничего не меняет в репозитории. Подкоманды со
// смешанным смыслом (stash, remote, config, submodule, worktree) читают только
// в формах, которые здесь перечислены; перечислено то, что пакет запускает.
// check-ignore отвечает кодом 1, когда путь не игнорируется: это ответ, а не
// сбой (#402).
func readOnly(args []string) bool {
	if len(args) == 0 {
		return true
	}

	switch rest := args[1:]; args[0] {
	case "rev-parse", "diff", "ls-files", "ls-tree", "for-each-ref", "log", "merge-base", "merge-tree",
		"symbolic-ref", "cat-file", "format-patch", "rev-list", "reflog", "check-ignore":
		return true
	// у hash-object -w пишет объект, а у diff это «без пробелов»
	case "hash-object":
		return !slices.Contains(rest, "-w")
	case "stash":
		return len(rest) > 0 && rest[0] == "list"
	case "worktree":
		return len(rest) > 0 && rest[0] == "list"
	case "submodule":
		return len(rest) > 0 && rest[0] == "status"
	case "remote":
		return len(rest) == 0 || rest[0] == "-v"
	case "config":
		return slices.ContainsFunc(rest, func(arg string) bool { return strings.HasPrefix(arg, "--get") })
	}
	return false
}
