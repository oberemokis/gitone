package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"gitone/internal/i18n"
)

// release это релиз на локальном сервере: последняя версия и файлы по
// именам. Хэши в checksums.txt считаются по файлам, если не заданы явно.
type release struct {
	tag   string
	files map[string]string
	sums  string
}

func serve(t *testing.T, r release) string {
	t.Helper()

	sums := r.sums
	if sums == "" {
		var lines []string
		for name, content := range r.files {
			hash := sha256.Sum256([]byte(content))
			lines = append(lines, hex.EncodeToString(hash[:])+"  "+name)
		}
		sums = strings.Join(lines, "\n") + "\n"
	}

	// Как GitHub: «последний релиз» ведёт на адрес с тегом, а тот на
	// подписанный адрес хранилища, где тега уже нет.
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest/download/", func(w http.ResponseWriter, req *http.Request) {
		name := strings.TrimPrefix(req.URL.Path, "/releases/latest/download/")
		http.Redirect(w, req, "/releases/download/"+r.tag+"/"+name, http.StatusFound)
	})
	mux.HandleFunc("/releases/download/"+r.tag+"/", func(w http.ResponseWriter, req *http.Request) {
		name := strings.TrimPrefix(req.URL.Path, "/releases/download/"+r.tag+"/")
		if _, found := r.files[name]; !found && name != "checksums.txt" {
			http.NotFound(w, req)
			return
		}
		http.Redirect(w, req, "/assets/"+hex.EncodeToString([]byte(name))+"?sig=1", http.StatusFound)
	})
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, req *http.Request) {
		name, _ := hex.DecodeString(strings.TrimPrefix(req.URL.Path, "/assets/"))
		if string(name) == "checksums.txt" {
			fmt.Fprint(w, sums)
			return
		}
		fmt.Fprint(w, r.files[string(name)])
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL + "/releases"
}

// fakeBinary печатает версию, как настоящий gitone --version.
func fakeBinary(version string) string {
	return "#!/bin/sh\necho gitone " + version + "\n"
}

// installed кладёт «текущий» gitone во временный каталог.
func installed(t *testing.T) string {
	t.Helper()

	exe := filepath.Join(t.TempDir(), "gitone")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func newUpdater(base, current, exe string) updater {
	return updater{base: base, client: http.DefaultClient, current: current, exe: exe, goos: "linux", goarch: "amd64"}
}

func TestUpdateReplacesBinary(t *testing.T) {
	base := serve(t, release{tag: "v0.4.0", files: map[string]string{"gitone_linux_amd64": fakeBinary("0.4.0")}})
	exe := installed(t)

	result, err := newUpdater(base, "0.3.0", exe).run(t.Context())
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if result.from != "0.3.0" || result.to != "0.4.0" {
		t.Errorf("run() = %+v, want from 0.3.0 to 0.4.0", result)
	}
	if got := readFile(t, exe); got != fakeBinary("0.4.0") {
		t.Errorf("бинарник после run() = %q, want новый", got)
	}
	if info, err := os.Stat(exe); err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("права после run() = %v, %v, want 0755", info.Mode().Perm(), err)
	}
}

// На Windows работающий exe не перезаписать, он уходит в .old. Та же
// последовательность проверяется здесь на любой системе.
func TestUpdateWindowsMovesOldAside(t *testing.T) {
	base := serve(t, release{tag: "v0.4.0", files: map[string]string{"gitone_windows_amd64.exe": fakeBinary("0.4.0")}})
	exe := installed(t)
	if err := os.WriteFile(exe+".old", []byte("older"), 0o755); err != nil {
		t.Fatal(err)
	}

	u := newUpdater(base, "0.3.0", exe)
	u.goos = "windows"
	if _, err := u.run(t.Context()); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if got := readFile(t, exe); got != fakeBinary("0.4.0") {
		t.Errorf("бинарник после run() = %q, want новый", got)
	}
	if got := siblings(t, exe); !slices.Equal(got, []string{exe}) {
		t.Errorf("файлы рядом с exe после run() = %q, want только exe", got)
	}
}

// Запасной файл с pid на Windows занят, пока старый gitone работает, и убрать
// его в тот же update нельзя. Следующий update должен найти его и убрать.
func TestUpdateWindowsRemovesLeftoverOldFiles(t *testing.T) {
	base := serve(t, release{tag: "v0.4.0", files: map[string]string{"gitone_windows_amd64.exe": fakeBinary("0.4.0")}})
	exe := installed(t)
	if err := os.WriteFile(exe+".4242.old", []byte("leftover"), 0o755); err != nil {
		t.Fatal(err)
	}

	u := newUpdater(base, "0.3.0", exe)
	u.goos = "windows"
	if _, err := u.run(t.Context()); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if got := siblings(t, exe); !slices.Equal(got, []string{exe}) {
		t.Errorf("файлы рядом с exe после run() = %q, want только exe", got)
	}
}

func TestUpdateUpToDate(t *testing.T) {
	base := serve(t, release{tag: "v0.4.0", files: map[string]string{"gitone_linux_amd64": fakeBinary("0.4.0")}})
	exe := installed(t)

	result, err := newUpdater(base, "0.4.0", exe).run(t.Context())
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !result.upToDate() {
		t.Errorf("run() = %+v, want уже последняя", result)
	}
	if got := readFile(t, exe); got != "old" {
		t.Errorf("бинарник после run() = %q, want нетронутый", got)
	}
}

// Отказы: бинарник остаётся прежним, ошибка называет причину.
func TestUpdateKeepsBinaryOnFailure(t *testing.T) {
	good := map[string]string{"gitone_linux_amd64": fakeBinary("0.4.0")}

	tests := []struct {
		name    string
		release release
		current string
		wantErr string
	}{
		{name: "сумма не сошлась", release: release{tag: "v0.4.0", files: good, sums: strings.Repeat("0", 64) + "  gitone_linux_amd64\n"}, current: "0.3.0", wantErr: "checksum"},
		{name: "нет строки в checksums.txt", release: release{tag: "v0.4.0", files: good, sums: strings.Repeat("0", 64) + "  other\n"}, current: "0.3.0", wantErr: "gitone_linux_amd64"},
		{name: "нет сборки под систему", release: release{tag: "v0.4.0", files: map[string]string{"gitone_darwin_arm64": fakeBinary("0.4.0")}}, current: "0.3.0", wantErr: "gitone_linux_amd64"},
		{name: "бинарник называет не ту версию", release: release{tag: "v0.4.0", files: map[string]string{"gitone_linux_amd64": fakeBinary("0.3.9")}}, current: "0.3.0", wantErr: "0.3.9"},
		{name: "бинарник не запускается", release: release{tag: "v0.4.0", files: map[string]string{"gitone_linux_amd64": "not a program"}}, current: "0.3.0", wantErr: "does not run"},
		{name: "сборка из исходников", release: release{tag: "v0.4.0", files: good}, current: "dev", wantErr: "make build"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exe := installed(t)

			_, err := newUpdater(serve(t, tc.release), tc.current, exe).run(t.Context())

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("run() error = %v, want с %q", err, tc.wantErr)
			}
			if got := readFile(t, exe); got != "old" {
				t.Errorf("бинарник после run() = %q, want нетронутый", got)
			}
			if entries, _ := os.ReadDir(filepath.Dir(exe)); len(entries) != 1 {
				t.Errorf("в каталоге после run() %d файлов, want только gitone", len(entries))
			}
		})
	}
}

func TestUpdateWithoutNetwork(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	base := server.URL + "/releases"
	server.Close()
	exe := installed(t)

	_, err := newUpdater(base, "0.3.0", exe).run(t.Context())

	if err == nil {
		t.Fatal("run() без сети error = nil, want ошибка")
	}
	if got := readFile(t, exe); got != "old" {
		t.Errorf("бинарник после run() = %q, want нетронутый", got)
	}
}

// Сбой после переадресации приходит от http.Client с последним адресом цепочки,
// а это подписанный адрес хранилища: подпись в сообщение попасть не должна.
func TestUpdateTransportErrorNamesRequestedURL(t *testing.T) {
	storage := httptest.NewServer(http.NotFoundHandler())
	signed := storage.URL + "/assets/checksums?sig=secret"
	storage.Close()
	server := httptest.NewServer(http.RedirectHandler(signed, http.StatusFound))
	t.Cleanup(server.Close)
	base := server.URL + "/releases"
	exe := installed(t)

	_, err := newUpdater(base, "0.3.0", exe).run(t.Context())

	requested := base + "/latest/download/checksums.txt"
	if err == nil || !strings.Contains(err.Error(), requested) {
		t.Errorf("run() error = %v, want с адресом %s", err, requested)
	}
	if err != nil && strings.Contains(err.Error(), "sig=secret") {
		t.Errorf("run() error = %v, want без подписи адреса хранилища", err)
	}
}

// Ошибка про переадресации идёт через словарь: на втором языке она переводится.
func TestUpdateRedirectLoopIsTranslated(t *testing.T) {
	server := httptest.NewServer(http.RedirectHandler("/loop", http.StatusFound))
	t.Cleanup(server.Close)
	base := server.URL + "/releases"
	text := i18n.Custom("xx", map[string]string{"update.tooManyRedirects": "XX {count}"})

	_, err := newUpdater(base, "0.3.0", installed(t)).run(t.Context())

	want := `Get "` + base + `/latest/download/checksums.txt": XX 10`
	if err == nil || text.Error(err) != want {
		t.Errorf("text.Error(%v) = %q, want %q", err, text.Error(err), want)
	}
}

// Подсказка зависит от системы: на Windows sudo нет, а пути в тексте стоят в
// кавычках, потому что в них бывают пробелы («C:\Program Files\gitone»).
func TestUpdateWithoutWritePermission(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root пишет в любой каталог")
	}

	tests := []struct {
		name string
		goos string
		// want получает каталог и путь к exe
		want string
	}{
		{name: "unix", goos: "linux", want: `cannot write to "%[1]s", run: sudo "%[2]s" update`},
		{name: "windows", goos: "windows", want: `cannot write to "%[1]s", run gitone update as administrator`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := serve(t, release{tag: "v0.4.0", files: map[string]string{
				"gitone_linux_amd64":       fakeBinary("0.4.0"),
				"gitone_windows_amd64.exe": fakeBinary("0.4.0"),
			}})
			exe := installed(t)
			dir := filepath.Dir(exe)
			if err := os.Chmod(dir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(dir, 0o755) })
			u := newUpdater(base, "0.3.0", exe)
			u.goos = tc.goos

			_, err := u.run(t.Context())

			want := fmt.Sprintf(tc.want, dir, exe)
			if err == nil || err.Error() != want {
				t.Errorf("run() error = %v, want %q", err, want)
			}
			if got := readFile(t, exe); got != "old" {
				t.Errorf("бинарник после run() = %q, want нетронутый", got)
			}
		})
	}
}

// roundTripFunc это транспорт из функции: тест отдаёт ответ сам, без сети.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// drip отдаёт тело кусками по килобайту с паузой между ними. С hang после
// последнего куска тело молчит и не заканчивается, как повисшее соединение.
// Отмена запроса обрывает тело, как у настоящего транспорта. Паузы идут по
// часам пузыря synctest, настоящего ожидания нет.
func drip(chunks int, gap time.Duration, hang bool) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, writer := io.Pipe()
		go func() {
			<-req.Context().Done()
			writer.CloseWithError(context.Cause(req.Context()))
		}()
		go func() {
			for range chunks {
				time.Sleep(gap)
				writer.Write(bytes.Repeat([]byte("x"), 1024))
			}
			if !hang {
				writer.Close()
			}
		}()

		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: body}, nil
	})
}

// Загрузка дольше таймаута простоя проходит, пока байты идут: общего потолка
// на неё нет. Бинарник около 27 МБ на медленном канале иначе не скачать.
func TestFetchSlowBodyWithinIdleTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		u := updater{client: &http.Client{Transport: drip(6, 100*time.Millisecond, false)}, idle: 300 * time.Millisecond}

		body, _, err := u.fetch(t.Context(), "https://example.test/gitone")

		if err != nil {
			t.Fatalf("fetch() error = %v", err)
		}
		if len(body) != 6*1024 {
			t.Errorf("fetch() отдал %d байт, want %d", len(body), 6*1024)
		}
	})
}

func TestFetchStalledBody(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		u := updater{client: &http.Client{Transport: drip(2, 100*time.Millisecond, true)}, idle: 300 * time.Millisecond}

		_, _, err := u.fetch(t.Context(), "https://example.test/gitone")

		want := "https://example.test/gitone: no data for 300ms, nothing was installed"
		if err == nil || err.Error() != want {
			t.Errorf("fetch() error = %v, want %q", err, want)
		}
	})
}

func TestNewer(t *testing.T) {
	tests := []struct {
		candidate, current string
		want               bool
	}{
		{candidate: "0.4.0", current: "0.3.0", want: true},
		{candidate: "0.3.0", current: "0.3.0", want: false},
		{candidate: "0.10.0", current: "0.9.0", want: true},
		{candidate: "0.9.0", current: "0.10.0", want: false},
		{candidate: "1.0.0", current: "0.99.99", want: true},
		{candidate: "0.4.0", current: "0.4.0-rc.1", want: true},
		{candidate: "0.4.0-rc.1", current: "0.4.0", want: false},
		{candidate: "0.4.0-rc.2", current: "0.4.0-rc.1", want: true},
	}

	for _, tc := range tests {
		if got := newer(tc.candidate, tc.current); got != tc.want {
			t.Errorf("newer(%q, %q) = %v, want %v", tc.candidate, tc.current, got, tc.want)
		}
	}
}

func TestIsUpdateCommand(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"gitone", "update"}, want: true},
		// дифф против ветки update остаётся доступен
		{args: []string{"gitone", "update", "--"}, want: false},
		{args: []string{"gitone", "main"}, want: false},
		{args: []string{"gitone"}, want: false},
	}

	for _, tc := range tests {
		if got := isUpdateCommand(tc.args); got != tc.want {
			t.Errorf("isUpdateCommand(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

// siblings перечисляет всё, что лежит в одном каталоге с exe, включая его самого.
func siblings(t *testing.T, exe string) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(filepath.Dir(exe), "*"))
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
