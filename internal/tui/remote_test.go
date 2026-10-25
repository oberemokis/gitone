package tui_test

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

var ctrlL = tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl}

// remoteRepo это репозиторий с двумя коммитами, ветка main которого привязана к
// origin (bare-репозиторий во временном каталоге, сети нет), и правкой a.txt в
// рабочем дереве.
func remoteRepo(t *testing.T) (origin, repo string) {
	t.Helper()

	repo = newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "c.txt", "mine\n")
	commitAll(t, repo)

	origin = filepath.Join(t.TempDir(), "origin.git")
	runGit(t, repo, "init", "-q", "--bare", "-b", "main", origin)
	runGit(t, repo, "remote", "add", "origin", origin)
	runGit(t, repo, "push", "-q", "--set-upstream", "origin", "main")
	write(t, repo, "a.txt", "two\n")

	return origin, repo
}

// publishPeer кладёт в main на origin коммит со стороны, как коллега.
func publishPeer(t *testing.T, origin, name, content string) {
	t.Helper()

	peer := filepath.Join(t.TempDir(), "peer")
	runGit(t, filepath.Dir(peer), "clone", "-q", origin, peer)
	write(t, peer, name, content)
	commitAll(t, peer)
	runGit(t, peer, "push", "-q", "origin", "main")
}

func openRemote(t *testing.T, repo string) tea.Model {
	t.Helper()
	return resize(open(t, options(repo, layout.Focus())), 120, 20)
}

// themed открывает репозиторий с темой, где цвет ошибки #070707 отличим от
// остальных, а приглушённый это mutedFg.
func themed(t *testing.T, repo string) tea.Model {
	t.Helper()

	custom := theme.Dark()
	custom.Error = lipgloss.Color("#070707")
	custom.TextMuted = lipgloss.Color("#030303")
	opts := options(repo, layout.Focus())
	opts.Theme = custom

	return resize(open(t, opts), 120, 20)
}

// pressAndSettle нажимает клавишу и доводит команды до конца, как Bubble Tea.
func pressAndSettle(model tea.Model, msg tea.KeyPressMsg) tea.Model {
	model, cmd := model.Update(msg)
	return settle(model, cmd, 6)
}

// pressIdle нажимает клавишу, на которую git не вызывается: команды нет.
func pressIdle(t *testing.T, model tea.Model, msg tea.KeyPressMsg) tea.Model {
	t.Helper()

	model, cmd := model.Update(msg)
	if cmd != nil {
		t.Fatalf("клавиша %q запустила команду, ожидался отказ без неё", msg)
	}
	return model
}

func revision(t *testing.T, repo, name string) string {
	t.Helper()
	return runGit(t, repo, "rev-parse", name)
}

// Case 1: ветка отстаёт от origin, p перематывает её, дифф перечитывается, в
// подвале заметка об успехе. Дифф коммита HEAD~1..HEAD меняется вместе с HEAD.
func TestPullFastForwardsAndRereadsDiff(t *testing.T) {
	origin, repo := remoteRepo(t)
	opts := options(repo, layout.Focus())
	opts.Args = []string{"HEAD~1", "HEAD"}
	model := resize(open(t, opts), 120, 20)
	publishPeer(t, origin, "b.txt", "two\n")
	if got := openedFile(screenOf(model)); got != "c.txt" {
		t.Fatalf("до pull открыт %q, ожидался c.txt", got)
	}

	model = pressAndSettle(model, key('p'))

	if got := revision(t, repo, "HEAD"); got != revision(t, origin, "main") {
		t.Errorf("HEAD %s, ожидался main с origin %s", got, revision(t, origin, "main"))
	}
	if got := openedFile(screenOf(model)); got != "b.txt" {
		t.Errorf("после pull открыт %q, ожидался b.txt: дифф не перечитан", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Fast-forwarded") || strings.Contains(footer, "Pulling") {
		t.Errorf("подвал %q, ожидалась заметка Fast-forwarded без индикатора", footer)
	}
}

func TestPullReportsUpToDate(t *testing.T) {
	_, repo := remoteRepo(t)

	model := pressAndSettle(openRemote(t, repo), key('p'))

	if footer := footerOf(model); !strings.Contains(footer, "Already up to date") {
		t.Errorf("подвал %q, ожидалось Already up to date", footer)
	}
}

// Case 2: ветки разошлись, git отказывает, ничего не меняется.
func TestPullOfDivergedBranchesShowsGitRefusal(t *testing.T) {
	origin, repo := remoteRepo(t)
	publishPeer(t, origin, "b.txt", "peer\n")
	runGit(t, repo, "add", "a.txt")
	runGit(t, repo, "commit", "-q", "-m", "local")
	before := revision(t, repo, "HEAD")

	model := pressAndSettle(openRemote(t, repo), key('p'))

	if footer := footerOf(model); !strings.Contains(footer, "Not possible to fast-forward") {
		t.Errorf("подвал %q, ожидался отказ git", footer)
	}
	if got := revision(t, repo, "HEAD"); got != before {
		t.Errorf("HEAD %s, ожидался прежний %s", got, before)
	}
	if _, err := os.Stat(filepath.Join(repo, "b.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("b.txt появился в рабочем дереве: %v", err)
	}
}

// Case 3: коммиты, опережающие origin, уходят на него.
func TestPushSendsCommitsAndSaysSo(t *testing.T) {
	origin, repo := remoteRepo(t)
	runGit(t, repo, "add", "a.txt")
	runGit(t, repo, "commit", "-q", "-m", "ahead")

	model := pressAndSettle(openRemote(t, repo), key('P'))

	if got := revision(t, origin, "main"); got != revision(t, repo, "HEAD") {
		t.Errorf("main на origin %s, ожидался HEAD %s", got, revision(t, repo, "HEAD"))
	}
	if footer := footerOf(model); !strings.Contains(footer, "Pushed to origin/main") {
		t.Errorf("подвал %q, ожидалось Pushed to origin/main", footer)
	}
}

// Case 4: ветка без upstream создаётся на origin и привязывается.
func TestPushCreatesBranchOnOrigin(t *testing.T) {
	origin, repo := remoteRepo(t)
	runGit(t, repo, "switch", "-q", "-c", "feature")

	model := pressAndSettle(openRemote(t, repo), key('P'))

	if got := runGit(t, repo, "rev-parse", "--abbrev-ref", "@{u}"); got != "origin/feature" {
		t.Errorf("upstream ветки %q, ожидался origin/feature", got)
	}
	if got := revision(t, origin, "feature"); got != revision(t, repo, "HEAD") {
		t.Errorf("feature на origin %s, ожидался HEAD %s", got, revision(t, repo, "HEAD"))
	}
	if footer := footerOf(model); !strings.Contains(footer, "Pushed to origin/feature") {
		t.Errorf("подвал %q, ожидалось Pushed to origin/feature", footer)
	}
}

// Case 5: при detached HEAD оба отказывают текстом из словаря.
func TestPullAndPushRefuseDetachedHead(t *testing.T) {
	for name, pressed := range map[string]tea.KeyPressMsg{"pull": key('p'), "push": key('P')} {
		t.Run(name, func(t *testing.T) {
			_, repo := remoteRepo(t)
			runGit(t, repo, "checkout", "-q", "--detach")

			model := pressAndSettle(openRemote(t, repo), pressed)

			want := name + ": HEAD is detached, switch to a branch first"
			if footer := footerOf(model); !strings.Contains(footer, want) {
				t.Errorf("подвал %q, ожидалось %q", footer, want)
			}
		})
	}
}

// Case 6: пока идёт pull или push, по центру подвала серый спиннер с надписью,
// как у загрузки диффа. После ответа он пропадает.
func TestRemoteSpinnerIsCenteredAndMuted(t *testing.T) {
	cases := []struct {
		name  string
		press tea.KeyPressMsg
		label string
	}{
		{name: "pull", press: key('p'), label: "Pulling…"},
		{name: "push", press: key('P'), label: "Pushing…"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, repo := remoteRepo(t)

			running, cmd := themed(t, repo).Update(tc.press)

			footer := footerOf(running)
			if !strings.Contains(footer, "⠋ "+tc.label) {
				t.Fatalf("подвал %q, ожидался спиннер с %q", footer, tc.label)
			}
			// на экране 120 колонок без полей остаётся 118, надпись шириной 10 встаёт в
			// колонку (118-10)/2 = 54 и ещё на колонку правее из-за левого поля
			if got := ansi.StringWidth(footer[:strings.Index(footer, "⠋")]); got != 55 {
				t.Errorf("спиннер с колонки %d, ожидалась 55, по центру: %q", got, footer)
			}
			for _, text := range []string{"⠋", tc.label} {
				if got := styleBefore(t, running.View().Content, tc.label, text); !strings.Contains(got, mutedFg) {
					t.Errorf("перед %q стиль %q, ожидался приглушённый цвет %s", text, got, mutedFg)
				}
			}

			done := settle(running, cmd, 6)
			if footer := footerOf(done); strings.Contains(footer, "⠋") || strings.Contains(footer, "ing…") {
				t.Errorf("после ответа в подвале остался индикатор: %q", footer)
			}
		})
	}
}

// Спиннер pull крутится, пока git работает, цепочка шагов своя и не зависит от
// загрузки диффа.
func TestRemoteSpinnerTurns(t *testing.T) {
	_, repo := remoteRepo(t)

	model, cmd := openRemote(t, repo).Update(key('p'))
	turned := false
	for _, msg := range spinFirst(runAll(cmd)) {
		model, _ = model.Update(msg)
		turned = turned || strings.Contains(footerOf(model), "⠙ Pulling…")
	}

	if !turned {
		t.Error("спиннер не сделал шаг, пока шёл pull")
	}
}

// Перечитывание дифа по r во время pull не гасит индикатор pull и не крадёт
// его место: в центре остаётся Pulling, а не loading.
func TestReloadDuringPullKeepsPullIndicator(t *testing.T) {
	_, repo := remoteRepo(t)

	model, _ := openRemote(t, repo).Update(key('p'))
	model, _ = model.Update(key('r'))

	if footer := footerOf(model); !strings.Contains(footer, "Pulling…") || strings.Contains(footer, "loading") {
		t.Errorf("подвал %q, ожидался индикатор pull без loading", footer)
	}
}

// Case 7: пока идёт pull, действия, меняющие репозиторий, отказывают заметкой и
// ничего не запускают, а просмотр работает.
func TestActionsRefuseWhilePullRuns(t *testing.T) {
	_, repo := remoteRepo(t)
	model, _ := openRemote(t, repo).Update(key('p'))

	for _, pressed := range []tea.KeyPressMsg{key('x'), space, key('c'), key('z'), key('p'), key('P')} {
		model = pressIdle(t, model, pressed)

		if footer := footerOf(model); !strings.Contains(footer, "pull is running") {
			t.Errorf("после %q подвал %q, ожидалось «pull is running»", pressed.String(), footer)
		}
		if body := screenText(model); strings.Contains(body, "Commit") || strings.Contains(footerOf(model), "Discard changes") {
			t.Errorf("после %q открылось окно или вопрос:\n%s", pressed.String(), body)
		}
	}
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Errorf("a.txt = %q, правка не должна пропасть", got)
	}

	model = openAsync(model, key('l'))
	if body := screenText(model); !strings.Contains(body, "Log") {
		t.Errorf("лог не открылся во время pull:\n%s", body)
	}
}

// Во время push те же действия отказывают с его названием.
func TestActionsRefuseWhilePushRuns(t *testing.T) {
	_, repo := remoteRepo(t)
	model, _ := openRemote(t, repo).Update(key('P'))

	model = pressIdle(t, model, key('p'))

	if footer := footerOf(model); !strings.Contains(footer, "push is running") {
		t.Errorf("подвал %q, ожидалось «push is running»", footer)
	}
}

// Пока идёт pull, в окнах отказывают переключение ветки и worktree, apply и drop
// у stash. Окно остаётся открытым, причина стоит в подвале.
func TestWindowActionsRefuseWhilePullRuns(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, repo string)
		open  tea.KeyPressMsg
		keys  []tea.KeyPressMsg
		check func(t *testing.T, repo string)
	}{
		{
			name:  "переключение ветки",
			setup: func(t *testing.T, repo string) { runGit(t, repo, "branch", "aaa") },
			open:  key('B'),
			// ветки идут по алфавиту: aaa над main, курсор стоит на текущей
			keys: []tea.KeyPressMsg{key('k'), {Code: tea.KeyEnter}},
			check: func(t *testing.T, repo string) {
				if got := runGit(t, repo, "branch", "--show-current"); got != "main" {
					t.Errorf("текущая ветка %q, ожидалась main", got)
				}
			},
		},
		{
			name:  "удаление ветки",
			setup: func(t *testing.T, repo string) { runGit(t, repo, "branch", "aaa") },
			open:  key('B'),
			keys:  []tea.KeyPressMsg{key('k'), key('d')},
			check: func(t *testing.T, repo string) {
				if got := runGit(t, repo, "branch", "--list", "aaa"); got == "" {
					t.Error("ветка aaa удалена, ожидалась на месте")
				}
			},
		},
		{
			name:  "новый тег",
			setup: func(t *testing.T, repo string) {},
			open:  key('t'),
			// имя и пустое сообщение: открытое поле поставило бы лёгкий тег v9
			keys: []tea.KeyPressMsg{key('n'), key('v'), key('9'), {Code: tea.KeyEnter}, {Code: tea.KeyEnter}},
			check: func(t *testing.T, repo string) {
				if got := runGit(t, repo, "tag", "--list"); got != "" {
					t.Errorf("теги %q, ожидалось ни одного", got)
				}
			},
		},
		{
			name: "переключение worktree",
			setup: func(t *testing.T, repo string) {
				runGit(t, repo, "worktree", "add", "-q", "-b", "side", filepath.Join(t.TempDir(), "side"))
			},
			open: key('W'),
			keys: []tea.KeyPressMsg{key('j'), {Code: tea.KeyEnter}},
			check: func(t *testing.T, repo string) {
				if got := runGit(t, repo, "branch", "--show-current"); got != "main" {
					t.Errorf("текущая ветка %q, ожидалась main", got)
				}
			},
		},
		{
			name: "применение stash",
			setup: func(t *testing.T, repo string) {
				runGit(t, repo, "stash", "push", "-q")
			},
			open: key('Z'),
			keys: []tea.KeyPressMsg{key('a')},
			check: func(t *testing.T, repo string) {
				if got := readText(t, filepath.Join(repo, "a.txt")); got != "one\n" {
					t.Errorf("a.txt = %q, stash не должен применяться", got)
				}
			},
		},
		{
			name: "удаление stash",
			setup: func(t *testing.T, repo string) {
				runGit(t, repo, "stash", "push", "-q")
			},
			open: key('Z'),
			keys: []tea.KeyPressMsg{key('d'), key('y')},
			check: func(t *testing.T, repo string) {
				if got := runGit(t, repo, "stash", "list"); got == "" {
					t.Error("stash удалён, ожидался на месте")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, repo := remoteRepo(t)
			tc.setup(t, repo)
			model, _ := openRemote(t, repo).Update(key('p'))
			model = openAsync(model, tc.open)

			// команды нажатий доводятся до конца: действие, которое отказ
			// пропустил, иначе не дошло бы до git, и проверка не упала бы
			for _, pressed := range tc.keys {
				model = pressAndSettle(model, pressed)
			}

			if footer := footerOf(model); !strings.Contains(footer, "pull is running") {
				t.Errorf("подвал %q, ожидалось «pull is running»", footer)
			}
			tc.check(t, repo)
		})
	}
}

// Case 8: esc во время pull отменяет операцию: подвал говорит Cancelled, git не
// оставляет index.lock, ветка на месте.
func TestEscCancelsPull(t *testing.T) {
	origin, repo := remoteRepo(t)
	publishPeer(t, origin, "b.txt", "two\n")
	before := revision(t, repo, "HEAD")
	model, cmd := openRemote(t, repo).Update(key('p'))

	model = pressIdle(t, model, escape)
	model = settle(model, cmd, 6)

	if footer := footerOf(model); !strings.Contains(footer, "Cancelled") || strings.Contains(footer, "Pulling") {
		t.Errorf("подвал %q, ожидалось Cancelled без индикатора", footer)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "index.lock")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("остался index.lock: %v", err)
	}
	if got := revision(t, repo, "HEAD"); got != before {
		t.Errorf("HEAD %s, ожидался прежний %s: отменённый pull перемотал ветку", got, before)
	}
}

func TestEscCancelsPush(t *testing.T) {
	origin, repo := remoteRepo(t)
	runGit(t, repo, "add", "a.txt")
	runGit(t, repo, "commit", "-q", "-m", "ahead")
	originBefore := revision(t, origin, "main")
	model, cmd := openRemote(t, repo).Update(key('P'))

	model = pressIdle(t, model, escape)
	model = settle(model, cmd, 6)

	if footer := footerOf(model); !strings.Contains(footer, "Cancelled") {
		t.Errorf("подвал %q, ожидалось Cancelled", footer)
	}
	if got := revision(t, origin, "main"); got != originBefore {
		t.Errorf("main на origin %s, ожидался прежний %s: отменённый push дошёл", got, originBefore)
	}
}

// Без операции esc на главном экране ничего не делает.
func TestEscDoesNothingWithoutRemoteOperation(t *testing.T) {
	_, repo := remoteRepo(t)
	model := openRemote(t, repo)
	before := screenText(model)

	model = pressIdle(t, model, escape)

	if got := screenText(model); got != before {
		t.Errorf("esc изменил экран:\n%s\n---\n%s", before, got)
	}
}

// Пока открыта справка, esc закрывает её, а pull продолжает идти: отмена
// только вторым нажатием.
func TestEscClosesHelpBeforeCancelling(t *testing.T) {
	_, repo := remoteRepo(t)
	model, _ := openRemote(t, repo).Update(key('p'))
	model, _ = model.Update(key('?'))

	model = pressIdle(t, model, escape)

	if body := screenText(model); strings.Contains(body, "Hotkeys") || !strings.Contains(footerOf(model), "Pulling…") {
		t.Errorf("esc с открытой справкой: ожидалась закрытая справка и идущий pull:\n%s", body)
	}
}

// Case 9: remote просит пароль. Git не читает терминал, причина стоит в
// подвале, интерфейс на месте.
func TestPullAskingForPasswordShowsErrorAndKeepsScreen(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="gitone"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	_, repo := remoteRepo(t)
	runGit(t, repo, "remote", "set-url", "origin", server.URL+"/repo.git")

	model := pressAndSettle(openRemote(t, repo), key('p'))

	if footer := footerOf(model); !strings.Contains(footer, "terminal prompts disabled") {
		t.Errorf("подвал %q, ожидался отказ git без пароля", footer)
	}
	if got := openedFile(screenOf(model)); got != "a.txt" {
		t.Errorf("открыт %q, ожидался a.txt: экран испорчен", got)
	}
}

// Case 10: клавиша pull переназначается и отключается.
func TestPullKeyFollowsKeybinds(t *testing.T) {
	t.Run("переназначение", func(t *testing.T) {
		origin, repo := remoteRepo(t)
		publishPeer(t, origin, "b.txt", "two\n")
		opts := options(repo, layout.Focus())
		opts.Keys = loadKeys(t, `{"keybinds":{"git.pull":"ctrl+l"}}`)
		model := resize(open(t, opts), 120, 20)

		model = pressIdle(t, model, key('p'))
		model = pressAndSettle(model, ctrlL)

		if got := revision(t, repo, "HEAD"); got != revision(t, origin, "main") {
			t.Errorf("HEAD %s, ожидался main с origin: ctrl+l не подтянул ветку", got)
		}
		if footer := footerOf(model); !strings.Contains(footer, "Fast-forwarded") {
			t.Errorf("подвал %q, ожидалось Fast-forwarded", footer)
		}
	})

	t.Run("отключение", func(t *testing.T) {
		_, repo := remoteRepo(t)
		opts := options(repo, layout.Focus())
		opts.Keys = loadKeys(t, `{"keybinds":{"git.pull":"none"}}`)
		model := resize(open(t, opts), 120, 20)

		model = pressIdle(t, model, key('p'))

		if footer := footerOf(model); strings.Contains(footer, "Pulling") || strings.Contains(footer, "up to date") {
			t.Errorf("подвал %q: отключённая клавиша запустила pull", footer)
		}
	})
}

// Тост итога и отмены в рамке не цвета ошибки, а ошибки git и отказа в
// рамке цвета ошибки (#371).
func TestRemoteNoticeColors(t *testing.T) {
	errorColor := lipgloss.Color("#070707")

	t.Run("успех", func(t *testing.T) {
		_, repo := remoteRepo(t)

		model := pressAndSettle(themed(t, repo), key('p'))

		if !strings.Contains(noticeOf(model), "Already up to date") || sameRGB(borderColor(t, model, toastRow), errorColor) {
			t.Errorf("тост об успехе %q в рамке цвета ошибки", noticeOf(model))
		}
	})

	t.Run("отмена", func(t *testing.T) {
		_, repo := remoteRepo(t)
		model, cmd := themed(t, repo).Update(key('p'))
		model = pressIdle(t, model, escape)

		model = settle(model, cmd, 6)

		if !strings.Contains(noticeOf(model), "Cancelled") || sameRGB(borderColor(t, model, toastRow), errorColor) {
			t.Errorf("тост об отмене %q в рамке цвета ошибки", noticeOf(model))
		}
	})

	t.Run("ошибка git", func(t *testing.T) {
		_, repo := remoteRepo(t)
		runGit(t, repo, "checkout", "-q", "--detach")

		model := pressAndSettle(themed(t, repo), key('p'))

		if got := borderColor(t, model, toastRow); !strings.Contains(noticeOf(model), "detached") || !sameRGB(got, errorColor) {
			t.Errorf("тост ошибки git %q в рамке %v, ожидался цвет ошибки", noticeOf(model), got)
		}
	})

	t.Run("отказ во время pull", func(t *testing.T) {
		_, repo := remoteRepo(t)
		model, _ := themed(t, repo).Update(key('p'))
		model = pressIdle(t, model, key('x'))

		if got := borderColor(t, model, toastRow); !strings.Contains(noticeOf(model), "pull is running") || !sameRGB(got, errorColor) {
			t.Errorf("тост отказа %q в рамке %v, ожидался цвет ошибки", noticeOf(model), got)
		}
	})
}

// Справка называет оба действия с клавишами.
func TestHelpListsPullAndPush(t *testing.T) {
	_, repo := remoteRepo(t)
	model := resize(open(t, options(repo, layout.Focus())), 100, 40)

	model, _ = model.Update(key('?'))

	body := screenText(model)
	for _, want := range []string{"Pull, fast-forward only", "Push the branch"} {
		if !strings.Contains(body, want) {
			t.Errorf("в справке нет %q:\n%s", want, body)
		}
	}
}
