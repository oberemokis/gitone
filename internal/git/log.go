package git

import (
	"strconv"
	"strings"
)

// LogEntry это строка лога. Root значит, что у коммита нет родителя: его дифф
// через <коммит>^! не строится.
type LogEntry struct {
	Hash    string
	Subject string
	Author  string
	When    string
	Root    bool
}

// Log возвращает limit последних коммитов текущей ветки от новых к старым.
// В репозитории без коммитов список пуст без ошибки. Формат задан явно,
// разделитель не встречается в темах и именах.
func Log(dir string, limit int) ([]LogEntry, error) {
	root, err := topLevel(dir)
	if err != nil {
		return nil, failed("log", err)
	}

	if _, err := run(call{dir: root}, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return nil, nil
	}

	out, err := run(call{dir: root}, "log", "-n", strconv.Itoa(limit), "--format=%H%x1f%P%x1f%s%x1f%an%x1f%cr")
	if err != nil {
		return nil, failed("log", err)
	}

	var commits []LogEntry
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, "\x1f")
		if len(fields) != 5 {
			continue
		}

		commits = append(commits, LogEntry{
			Hash:    fields[0],
			Root:    fields[1] == "",
			Subject: fields[2],
			Author:  fields[3],
			When:    fields[4],
		})
	}

	return commits, nil
}
