package tui_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/askpass"
	"gitone/internal/layout"
)

// TestMain отвечает git, когда тест запускает этот бинарь как GIT_ASKPASS
// (#375): вопрос идёт в окно тем же путём, что у gitone.
func TestMain(m *testing.M) {
	if asked, err := askpass.Reply(os.Args[1:], os.Stdout); asked {
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	os.Exit(m.Run())
}

// credentialRepo это репозиторий, origin которого требует логин и пароль, как
// https-сервер без записи в хранилище учётных данных, и модель с сервером
// askpass. Сервер на любой запрос отвечает 401 и записывает присланные пары.
func credentialRepo(t *testing.T) (model tea.Model, server *askpass.Server, sent func() []string) {
	t.Helper()

	var mu sync.Mutex
	var pairs []string
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, password, ok := r.BasicAuth(); ok {
			mu.Lock()
			pairs = append(pairs, user+":"+password)
			mu.Unlock()
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="gitone"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(remote.Close)

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "remote", "add", "origin", remote.URL+"/repo.git")
	// своя программа пользователя спросила бы вместо окна (случай 3)
	t.Setenv("GIT_ASKPASS", "")
	os.Unsetenv("GIT_ASKPASS")

	server, err := askpass.Listen()
	if err != nil {
		t.Fatalf("askpass.Listen: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	opts := options(repo, layout.Focus())
	opts.Askpass = server
	model = resize(open(t, opts), 120, 20)

	return model, server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(pairs)
	}
}

// inBackground выполняет команду в своей горутине: git ждёт ответа окна, и
// окно получает сообщения, пока команда не вернулась.
func inBackground(cmd tea.Cmd) <-chan []tea.Msg {
	done := make(chan []tea.Msg, 1)
	go func() { done <- runAll(cmd) }()
	return done
}

// Fetch спрашивает имя и пароль в окне: имя видно при наборе, пароль идёт
// точками, а сервер получает именно эту пару (#375, случай 1).
func TestFetchAsksCredentialsInWindow(t *testing.T) {
	model, _, sent := credentialRepo(t)
	questions := model.Init()

	model, fetch := model.Update(key('F'))
	fetched := inBackground(fetch)
	model = settle(model, questions, 1)
	if screen := screenText(model); !strings.Contains(screen, "Username for 'http://127.0.0.1:") {
		t.Fatalf("окно не спросило имя:\n%s", screen)
	}
	model = typeText(model, "ilya")
	if screen := screenText(model); !strings.Contains(screen, "> ilya_") {
		t.Errorf("имя не видно при наборе:\n%s", screen)
	}

	model, next := model.Update(enter)
	model = typeText(settle(model, next, 1), "s3cret")
	screen := screenText(model)
	if !strings.Contains(screen, "Password for 'http://ilya@127.0.0.1:") || !strings.Contains(screen, "> ••••••_") {
		t.Errorf("пароль спрошен не точками:\n%s", screen)
	}
	if strings.Contains(screen, "s3cret") {
		t.Errorf("пароль виден на экране:\n%s", screen)
	}

	model, _ = model.Update(enter)
	for _, msg := range <-fetched {
		model, _ = model.Update(msg)
	}

	if got := sent(); !slices.Equal(got, []string{"ilya:s3cret"}) {
		t.Errorf("сервер получил %q, ожидалось ilya:s3cret", got)
	}
	if screen := screenText(model); strings.Contains(screen, "Password for") {
		t.Errorf("окно вопроса не закрылось:\n%s", screen)
	}
}

// esc отменяет вопрос вместе с fetch: тост «Cancelled», как у esc во время
// fetch, сервер учётных данных не получает (случай 2).
func TestEscapeCancelsCredentials(t *testing.T) {
	model, _, sent := credentialRepo(t)
	questions := model.Init()

	model, fetch := model.Update(key('F'))
	fetched := inBackground(fetch)
	model, _ = settle(model, questions, 1).Update(escape)
	for _, msg := range <-fetched {
		model, _ = model.Update(msg)
	}

	if screen := screenText(model); strings.Contains(screen, "Username for") {
		t.Errorf("окно вопроса не закрылось:\n%s", screen)
	}
	if notice := noticeOf(model); notice != "Cancelled" {
		t.Errorf("тост %q, ожидался «Cancelled»", notice)
	}
	if got := sent(); len(got) != 0 {
		t.Errorf("сервер получил %q после отмены", got)
	}
}

// Заданный пользователем GIT_ASKPASS отвечает сам, окно gitone не спрашивает
// (случай 3).
func TestOwnAskpassAnswersInstead(t *testing.T) {
	model, server, sent := credentialRepo(t)
	program := filepath.Join(t.TempDir(), "askpass")
	if err := os.WriteFile(program, []byte("#!/bin/sh\necho theirs\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_ASKPASS", program)

	_, fetch := model.Update(key('F'))
	select {
	case <-inBackground(fetch):
	case request := <-server.Requests():
		request.Cancel()
		t.Fatalf("окно спросило %q при своём GIT_ASKPASS", request.Prompt)
	}

	if got := sent(); !slices.Equal(got, []string{"theirs:theirs"}) {
		t.Errorf("сервер получил %q, ожидалось theirs:theirs", got)
	}
}

// Вопрос, пришедший при открытом поле фильтра лога, ждёт его закрытия и не
// затирает набранное (случай 4). Правки репозитория во время fetch отказывают,
// а фильтр лога только читает.
func TestCredentialsWaitForOpenPrompt(t *testing.T) {
	model, _, _ := credentialRepo(t)
	questions := model.Init()

	model, fetch := model.Update(key('F'))
	fetched := inBackground(fetch)
	model = pressAndSettle(model, key('l'))
	model = pressAndSettle(model, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model = typeText(pressAndSettle(model, enter), "ilya")
	model = settle(model, questions, 1)
	if screen := screenText(model); strings.Contains(screen, "Username for") || !strings.Contains(screen, "> ilya_") {
		t.Errorf("вопрос открылся поверх поля фильтра:\n%s", screen)
	}

	model, _ = model.Update(escape)
	if screen := screenText(model); !strings.Contains(screen, "Username for") {
		t.Errorf("после поля фильтра вопрос не открылся:\n%s", screen)
	}

	model.Update(escape)
	<-fetched
}
