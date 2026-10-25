package git

import (
	"strings"

	"gitone/internal/i18n"
)

// Revert создаёт новый коммит с обратным диффом коммита hash (#221), сообщение
// git по умолчанию. Merge-коммит отвергается: для него git спросил бы номер
// родителя. Уже отменённый коммит тоже: git на нём отвечает «nothing to
// commit» и ничего не создаёт, отказом, а не остановкой.
// Конфликт останавливает revert, ошибка тогда git.stoppedOnConflict.
func (r Repo) Revert(hash string) error {
	return failed("revert", r.revert(hash))
}

func (r Repo) revert(hash string) error {
	if isMerge(r.root, hash) {
		return i18n.New("git.revertMerge", nil)
	}
	if changesNothing(r.root, hash, hash+"^") {
		return i18n.New("git.alreadyReverted", nil)
	}

	return r.stopping("revert", "revert", "--no-edit", hash)
}

// CherryPick переносит коммит hash в текущую ветку (#222) с его сообщением.
// Merge-коммит отвергается, как у Revert. Коммит, правка которого уже есть в
// ветке, тоже: git остановился бы на пустом cherry-pick и ждал бы решения.
// Конфликт останавливает cherry-pick, ошибка тогда git.stoppedOnConflict.
func (r Repo) CherryPick(hash string) error {
	return failed("cherry-pick", r.cherryPick(hash))
}

func (r Repo) cherryPick(hash string) error {
	if isMerge(r.root, hash) {
		return i18n.New("git.pickMerge", nil)
	}
	if changesNothing(r.root, hash+"^", hash) {
		return i18n.New("git.alreadyPicked", nil)
	}

	return r.stopping("cherry-pick", "cherry-pick", hash)
}

// isMerge говорит, что у коммита больше одного родителя.
func isMerge(root, hash string) bool {
	parents, err := run(call{dir: root}, "rev-list", "--parents", "-n", "1", hash)
	return err == nil && len(strings.Fields(parents)) > 2
}

// changesNothing говорит, что слияние HEAD с theirs от базы base, то самое,
// что делают revert и cherry-pick, оставит дерево HEAD как есть: правка уже
// там. merge-tree считает его без рабочего дерева и индекса. Конфликт или
// недоступная база (корневой коммит) значат, что изменения будут.
func changesNothing(root, base, theirs string) bool {
	merged, err := run(call{dir: root}, "merge-tree", "--write-tree", "--merge-base="+base, "HEAD", theirs)
	if err != nil {
		return false
	}

	head, err := run(call{dir: root}, "rev-parse", "HEAD^{tree}")
	tree, _, _ := strings.Cut(merged, "\n")
	return err == nil && tree == strings.TrimSpace(head)
}
