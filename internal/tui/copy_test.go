package tui_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// chooseAndCollect выбирает строку меню enter и доводит команды до конца, как
// settle, собирая все ответы: запись в буфер обмена Bubble Tea выполняет сама,
// и проверить её можно только по сообщению команды. Вложенный пакет команд
// раскрывается, как это делает Bubble Tea.
func chooseAndCollect(model tea.Model) (tea.Model, []string) {
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	var seen []string
	pending := []tea.Cmd{cmd}
	for depth := 0; depth < 6 && len(pending) > 0; depth++ {
		var next []tea.Cmd
		for _, cmd := range pending {
			for _, msg := range runAll(cmd) {
				if batch, isBatch := msg.(tea.BatchMsg); isBatch {
					next = append(next, batch...)
					continue
				}
				seen = append(seen, fmt.Sprint(msg))
				var follow tea.Cmd
				model, follow = model.Update(msg)
				next = append(next, follow)
			}
		}
		pending = next
	}
	return model, seen
}

// y в логе открывает меню, первая строка кладёт полный хэш в буфер обмена (#224,
// случай 1).
func TestCopyCommitHash(t *testing.T) {
	repo, model := openLog(t, 2, 120, 24)
	hash := runGit(t, repo, "rev-parse", "HEAD")

	model, _ = model.Update(key('y'))
	if body := screenText(model); !strings.Contains(body, "Copy the full hash") {
		t.Fatalf("y не открыл меню копирования:\n%s", body)
	}
	model, seen := chooseAndCollect(model)

	if !slices.Contains(seen, hash) {
		t.Fatalf("в буфер не ушёл хэш %s, ответы команд: %q", hash, seen)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Copied the hash of "+hash[:7]) {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}

// Вторая строка сохраняет патч в корень и кладёт его в буфер (случаи 2 и 3).
func TestCopyCommitPatch(t *testing.T) {
	repo, model := openLog(t, 2, 120, 24)

	model, _ = model.Update(key('y'))
	model, _ = model.Update(key('j'))
	model, seen := chooseAndCollect(model)

	patch := readText(t, filepath.Join(repo, "0001-commit.patch"))
	if !strings.Contains(patch, "Subject: [PATCH] commit") || !slices.Contains(seen, patch) {
		t.Fatalf("патч не сохранён или не ушёл в буфер, ответы команд: %q", seen)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Patch saved to 0001-commit.patch") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}
