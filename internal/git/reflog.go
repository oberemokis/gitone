package git

import "strings"

// ReflogEntry это запись reflog HEAD (#240): коммит, на который встал HEAD,
// действие git («commit», «checkout», «reset»), его описание и время записи.
// Root значит, что у коммита нет родителя.
type ReflogEntry struct {
	Hash    string
	Action  string
	Subject string
	When    string
	Root    bool
}

// Reflog возвращает записи reflog HEAD от новых к старым. Время записи, а не
// коммита, git отдаёт только в селекторе: HEAD@{5 minutes ago}. Репозиторий
// без коммитов reflog не имеет, это пустой список.
func (r Repo) Reflog() ([]ReflogEntry, error) {
	out, err := r.inRoot("reflog", "show", "--date=relative", "--format=%H%x1f%P%x1f%gd%x1f%gs", "HEAD", "--")
	if err != nil {
		if !hasCommits(r.root) {
			return nil, nil
		}
		return nil, failed("reflog", err)
	}

	var entries []ReflogEntry
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.SplitN(line, "\x1f", 4)
		if len(fields) != 4 {
			continue
		}

		when := strings.TrimSuffix(strings.TrimPrefix(fields[2], "HEAD@{"), "}")
		action, subject, _ := strings.Cut(fields[3], ": ")
		entries = append(entries, ReflogEntry{Hash: fields[0], Root: fields[1] == "", When: when, Action: action, Subject: subject})
	}
	return entries, nil
}
