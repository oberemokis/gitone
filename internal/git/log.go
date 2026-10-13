package git

import (
	"strconv"
	"strings"
)

// LogEntry это строка лога. Root значит, что у коммита нет родителя: его дифф
// через <коммит>^! не строится. Graph это рисунок графа слева от коммита, как
// в git log --graph. Строка без Hash это только соединение между коммитами.
type LogEntry struct {
	Graph   string
	Hash    string
	Subject string
	Author  string
	When    string
	Root    bool
}

// Log возвращает limit последних коммитов текущей ветки от новых к старым
// вместе с графом, между ними строки соединения графа. В репозитории без
// коммитов список пуст без ошибки. Формат задан явно: нулевой байт отделяет
// рисунок графа от полей и поля друг от друга. Имя автора и тема это
// произвольный текст, и \x1f или другой печатный разделитель в них возможен, а
// NUL git в этих полях не допускает. log.showSignature из конфига пользователя
// вставил бы между графом и форматом строку проверки подписи.
func Log(dir string, limit int) ([]LogEntry, error) {
	root, err := topLevel(dir)
	if err != nil {
		return nil, failed("log", err)
	}

	if _, err := run(call{dir: root}, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return nil, nil
	}

	out, err := run(call{dir: root}, "log", "--graph", "--no-color", "--no-show-signature", "-n", strconv.Itoa(limit), "--format=%x00%H%x00%P%x00%an%x00%cr%x00%s")
	if err != nil {
		return nil, failed("log", err)
	}

	var entries []LogEntry
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		graph, commit, found := strings.Cut(line, "\x00")
		graph = strings.TrimRight(graph, " ")
		if !found {
			entries = append(entries, LogEntry{Graph: graph})
			continue
		}

		fields := strings.SplitN(commit, "\x00", 5)
		entries = append(entries, LogEntry{
			Graph:   graph,
			Hash:    fields[0],
			Root:    fields[1] == "",
			Author:  fields[2],
			When:    fields[3],
			Subject: fields[4],
		})
	}

	return entries, nil
}
