package git

import (
	"strconv"
	"strings"

	"gitone/internal/i18n"
)

// ResetMode это режим reset (#220): что станет с правками между HEAD и целью.
type ResetMode string

const (
	// ResetSoft оставляет правки в индексе
	ResetSoft ResetMode = "soft"
	// ResetMixed оставляет правки в рабочем дереве, индекс пуст
	ResetMixed ResetMode = "mixed"
	// ResetHard стирает правки: дерево и индекс совпадают с целью
	ResetHard ResetMode = "hard"
)

// Reset переводит текущую ветку на target в режиме mode. published говорит,
// что среди снятых коммитов есть коммиты upstream ветки: ветка теперь
// разошлась с remote, и обычный push её не отправит. Без upstream published
// всегда false.
func (r Repo) Reset(target string, mode ResetMode) (published bool, err error) {
	published, err = r.reset(target, mode)
	return published, failed("reset", err)
}

func (r Repo) reset(target string, mode ResetMode) (bool, error) {
	published := dropsPublished(r.root, target)
	if _, err := r.inRoot("reset", "--"+string(mode), target, "--"); err != nil {
		return false, headline(err)
	}
	return published, nil
}

// ResetTarget это цель reset из окна веток (#231): у текущей ветки её upstream,
// у другой ветки она сама полной ссылкой, иначе одноимённый тег git предпочёл
// бы ей. drops это коммиты HEAD, которых нет в цели: их снимет reset, а hard
// не оставит и правок. Текущая ветка без upstream отвергается ошибкой из
// словаря.
func (r Repo) ResetTarget(branch string, current bool) (target string, drops int, err error) {
	target, drops, err = r.resetTarget(branch, current)
	return target, drops, failed("rev-list", err)
}

func (r Repo) resetTarget(branch string, current bool) (string, int, error) {
	target := "refs/heads/" + branch
	if current {
		upstream, err := r.inRoot("rev-parse", "--abbrev-ref", "@{upstream}")
		if err != nil {
			return "", 0, i18n.New("git.noUpstream", nil)
		}
		target = strings.TrimSpace(upstream)
	}

	count, err := r.inRoot("rev-list", "--count", target+"..HEAD")
	if err != nil {
		return "", 0, err
	}
	drops, err := strconv.Atoi(strings.TrimSpace(count))
	return target, drops, err
}

// dropsPublished говорит, что переход HEAD на target снимет коммит, который
// уже есть в upstream. Общие предки HEAD и upstream это ровно опубликованная
// часть ветки: если хоть один из них не предок target, reset его снимет.
func dropsPublished(root, target string) bool {
	bases, err := run(call{dir: root}, "merge-base", "--all", "HEAD", "@{upstream}")
	if err != nil {
		return false
	}

	for _, base := range strings.Fields(bases) {
		if _, err := run(call{dir: root}, "merge-base", "--is-ancestor", base, target); err != nil {
			return true
		}
	}
	return false
}
