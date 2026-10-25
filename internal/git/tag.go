package git

import (
	"context"
	"strings"
)

// Tag это тег (#238): имя, коммит, на который он указывает, сообщение
// аннотированного или тема коммита лёгкого и время создания. Annotated
// отличает аннотированный тег от лёгкого. Root значит, что у коммита тега нет
// родителя, как у LogEntry.
type Tag struct {
	Name      string
	Commit    string
	Subject   string
	When      string
	Annotated bool
	Root      bool
}

// Tags возвращает теги от новых к старым. Коммит аннотированного тега и его
// родители берутся через %(*objectname) и %(*parent): сам тег это отдельный
// объект.
func (r Repo) Tags() ([]Tag, error) {
	out, err := r.inRoot("for-each-ref", "--sort=-creatordate",
		"--format=%(refname:lstrip=2)%1f%(objecttype)%1f%(objectname)%1f%(*objectname)%1f%(parent)%1f%(*parent)%1f%(creatordate:relative)%1f%(contents:subject)", "refs/tags")
	if err != nil {
		return nil, failed("tag", err)
	}

	var tags []Tag
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.SplitN(line, "\x1f", 8)
		if len(fields) != 8 {
			continue
		}

		tag := Tag{Name: fields[0], Commit: fields[2], When: fields[6], Subject: fields[7], Annotated: fields[1] == "tag", Root: fields[4] == ""}
		if tag.Annotated {
			tag.Commit, tag.Root = fields[3], fields[5] == ""
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// CreateTag ставит тег name на HEAD: аннотированный с сообщением, лёгкий без
// него. Занятое имя git отвергает, тег не перезаписывается. Имя стоит после
// «--»: «-d» git иначе прочёл бы флагом и ответил успехом без тега (#389).
func (r Repo) CreateTag(name, message string) error {
	args := []string{"tag", "--", name}
	if message != "" {
		args = []string{"tag", "--annotate", "--message", message, "--", name}
	}

	_, err := r.inRoot(args...)
	return failed("tag", headline(err))
}

// DeleteTag удаляет локальный тег. Тег на remote остаётся: его удаление это
// отдельная отправка на remote. Имя после «--», как у CreateTag.
func (r Repo) DeleteTag(name string) error {
	_, err := r.inRoot("tag", "--delete", "--", name)
	return failed("tag", headline(err))
}

// PushTag отправляет тег на origin. Отмена ctx убивает git, ошибка тогда
// context.Canceled.
func (r Repo) PushTag(ctx context.Context, name string) error {
	return failed("push", headline(r.network(ctx, r.dir, "push", "origin", "refs/tags/"+name)))
}
