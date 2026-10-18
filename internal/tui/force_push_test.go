package tui_test

import (
	"strings"
	"testing"
)

// P на переписанной ветке не отправляет её, а спрашивает про force with lease;
// любая клавиша кроме y ничего не отправляет, y отправляет (#234, случаи 1 и 3).
func TestForcePushAfterAmend(t *testing.T) {
	origin, repo := remoteRepo(t)
	runGit(t, repo, "commit", "-q", "--amend", "-m", "rewritten")
	before := runGit(t, origin, "rev-parse", "main")

	model := pressAndSettle(openRemote(t, repo), key('P'))
	if footer := footerOf(model); !strings.Contains(footer, "Force push with lease? y yes") {
		t.Fatalf("подвал %q, ожидался вопрос", footer)
	}
	model, _ = model.Update(key('n'))
	if got := runGit(t, origin, "rev-parse", "main"); got != before {
		t.Fatal("ветка отправлена без y")
	}

	model = answerYes(pressAndSettle(model, key('P')))
	if got, want := runGit(t, origin, "rev-parse", "main"), runGit(t, repo, "rev-parse", "HEAD"); got != want {
		t.Fatalf("main на origin %s, ожидалась переписанная ветка %s", got, want)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Pushed to origin/main") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}

// Коммит коллеги, которого последний fetch не видел, lease не даёт затереть:
// подвал показывает отказ git (случай 2).
func TestForcePushKeepsUnseenCommits(t *testing.T) {
	origin, repo := remoteRepo(t)
	runGit(t, repo, "commit", "-q", "--amend", "-m", "rewritten")
	publishPeer(t, origin, "b.txt", "peer\n")
	peer := runGit(t, origin, "rev-parse", "main")

	model := answerYes(pressAndSettle(openRemote(t, repo), key('P')))

	if got := runGit(t, origin, "rev-parse", "main"); got != peer {
		t.Fatalf("main на origin %s, коммит коллеги %s затёрт", got, peer)
	}
	if footer := footerOf(model); !strings.Contains(footer, "push: ") {
		t.Fatalf("подвал %q, ожидался отказ git", footer)
	}
}
