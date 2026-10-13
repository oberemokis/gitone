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
// рисунок графа от полей. Тема это произвольный текст и может содержать
// разделитель полей, поэтому стоит в формате последней и режется по первым
// четырём разделителям. log.showSignature из конфига пользователя вставил бы
// между графом и форматом строку проверки подписи.
func Log(dir string, limit int) ([]LogEntry, error) {
	root, err := topLevel(dir)
	if err != nil {
		return nil, failed("log", err)
	}

	if _, err := run(call{dir: root}, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return nil, nil
	}

	out, err := run(call{dir: root}, "log", "--graph", "--no-color", "--no-show-signature", "-n", strconv.Itoa(limit), "--format=%x00%H%x1f%P%x1f%an%x1f%cr%x1f%s")
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

		fields := strings.SplitN(commit, "\x1f", 5)
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
