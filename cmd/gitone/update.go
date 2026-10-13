package main

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gitone/internal/i18n"
)

// releaseBase это страница релизов. Переадресации проходятся до конца,
// поэтому переименование репозитория update не ломает. Старое имя после
// переименования нельзя отдавать другому репозиторию: уже установленные
// gitone и скрипты установки пошли бы за релизами к нему.
const releaseBase = "https://github.com/oberemokis/gitone/releases"

// updater ставит последний релиз поверх работающего gitone. Адрес релизов,
// версия, путь и система лежат в полях, чтобы тест подставил свой сервер.
type updater struct {
	base    string
	client  *http.Client
	current string
	exe     string
	goos    string
	goarch  string
	// idle это сколько тело ответа может молчать, ноль значит idleTimeout
	idle time.Duration
	// downloading сообщает версию перед загрузкой: иначе медленная сеть
	// выглядит как зависание
	downloading func(version string)
}

type updateResult struct {
	from, to string
}

func (r updateResult) upToDate() bool {
	return r.from == r.to
}

// isUpdateCommand узнаёт `gitone update`. Как и --version, только одиночный
// аргумент: `gitone update --` остаётся диффом против ветки update.
func isUpdateCommand(args []string) bool {
	return len(args) == 2 && args[1] == "update"
}

// runUpdate обновляет работающий gitone и печатает итог на языке интерфейса.
// Ctrl+C прерывает загрузку, и бинарник остаётся прежним.
func runUpdate(text i18n.Dictionary) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		exit(text.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = headerTimeout

	u := updater{
		base:    releaseBase,
		client:  &http.Client{Transport: transport},
		current: version,
		exe:     exe,
		goos:    runtime.GOOS,
		goarch:  runtime.GOARCH,
		downloading: func(latest string) {
			fmt.Println(text.Text("update.downloading", i18n.Args{"version": latest}))
		},
	}
	result, err := u.run(ctx)
	if err != nil {
		exit(text.Error(err))
	}

	if result.upToDate() {
		fmt.Println(text.Text("update.upToDate", i18n.Args{"version": result.to}))
		return
	}
	fmt.Println(text.Text("update.done", i18n.Args{"from": result.from, "to": result.to}))
}

// Оборванное соединение иначе повесило бы update без единого слова, поэтому
// ждать можно только ограниченное время: ответа сервера не дольше headerTimeout,
// следующего байта тела не дольше idleTimeout. Общего потолка на загрузку нет:
// бинарник около 27 МБ, и на медленном канале любой потолок оборвал бы
// загрузку, которая идёт.
const (
	headerTimeout = 30 * time.Second
	idleTimeout   = 30 * time.Second
)

// run ставит новую версию, а при любом отказе оставляет бинарник прежним.
// Сборка из исходников (версия dev) не обновляется: она бывает новее
// последнего релиза, и update молча откатил бы её.
func (u updater) run(ctx context.Context) (updateResult, error) {
	if u.current == "dev" {
		return updateResult{}, i18n.New("update.dev", nil)
	}

	sums, tag, err := u.latestChecksums(ctx)
	if err != nil {
		return updateResult{}, err
	}
	latest := strings.TrimPrefix(tag, "v")
	if !newer(latest, u.current) {
		return updateResult{from: u.current, to: u.current}, nil
	}

	if u.downloading != nil {
		u.downloading(latest)
	}
	binary, err := u.download(ctx, tag, sums)
	if err != nil {
		return updateResult{}, err
	}
	if err := u.install(binary, latest); err != nil {
		return updateResult{}, err
	}

	return updateResult{from: u.current, to: latest}, nil
}

// releaseTag это тег в адресе файла релиза: …/download/v0.4.0/checksums.txt.
var releaseTag = regexp.MustCompile(`/download/(v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.]+)?)/checksums\.txt$`)

// latestChecksums берёт checksums.txt последнего релиза. Версию даёт адрес,
// через который прошла переадресация «последний релиз», отдельный запрос к
// API не нужен. Последний адрес цепочки тега не содержит: GitHub отдаёт файл
// с подписанного адреса хранилища.
func (u updater) latestChecksums(ctx context.Context) ([]byte, string, error) {
	url := u.base + "/latest/download/checksums.txt"
	body, chain, err := u.fetch(ctx, url)
	if err != nil {
		return nil, "", err
	}

	for _, visited := range chain {
		if match := releaseTag.FindStringSubmatch(visited.Path); match != nil {
			return body, match[1], nil
		}
	}

	return nil, "", i18n.New("update.noVersion", i18n.Args{"url": url})
}

// download берёт бинарник под свою систему и сверяет его с checksums.txt.
func (u updater) download(ctx context.Context, tag string, sums []byte) ([]byte, error) {
	asset := assetName(u.goos, u.goarch)
	want, found := checksumFor(sums, asset)
	if !found {
		return nil, i18n.New("update.noChecksum", i18n.Args{"asset": asset})
	}

	binary, _, err := u.fetch(ctx, u.base+"/download/"+tag+"/"+asset)
	if err != nil {
		return nil, err
	}

	got := sha256.Sum256(binary)
	if hex.EncodeToString(got[:]) != want {
		return nil, i18n.New("update.checksumMismatch", i18n.Args{"asset": asset})
	}

	return binary, nil
}

// fetch отдаёт тело ответа и адреса, через которые прошли переадресации.
// Ответ не 200 это ошибка: страница 404 иначе ушла бы в бинарник. В тексте
// ошибки запрошенный адрес, а не подписанный адрес хранилища.
func (u updater) fetch(ctx context.Context, url string) ([]byte, []*neturl.URL, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}

	chain := []*neturl.URL{request.URL}
	client := *u.client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errors.New("stopped after 10 redirects")
		}
		chain = append(chain, next.URL)
		return nil
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, nil, i18n.New("update.badStatus", i18n.Args{"url": url, "status": response.Status})
	}

	// Таймер простоя стартует после заголовков и сдвигается на каждом Read:
	// тело, которое идёт медленно, но идёт, не прерывается.
	idle := cmp.Or(u.idle, idleTimeout)
	timer := time.AfterFunc(idle, func() { cancel(errStalled) })
	defer timer.Stop()

	body, err := io.ReadAll(&idleReader{reader: response.Body, timer: timer, idle: idle})
	if err != nil && errors.Is(context.Cause(ctx), errStalled) {
		return nil, nil, i18n.New("update.stalled", i18n.Args{"url": url, "timeout": idle})
	}
	return body, chain, err
}

// errStalled это причина отмены запроса, когда тело ответа замолчало.
var errStalled = errors.New("response body stalled")

// idleReader сдвигает таймер простоя после каждого Read.
type idleReader struct {
	reader io.Reader
	timer  *time.Timer
	idle   time.Duration
}

func (r *idleReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.timer.Reset(r.idle)
	return n, err
}

// maxRedirects как у http.Client по умолчанию.
const maxRedirects = 10

func assetName(goos, goarch string) string {
	name := "gitone_" + goos + "_" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// checksumFor ищет строку файла в checksums.txt: «хэш  имя».
func checksumFor(sums []byte, asset string) (string, bool) {
	lines := bufio.NewScanner(bytes.NewReader(sums))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], true
		}
	}

	return "", false
}

// install кладёт бинарник рядом со старым, проверяет, что он запускается и
// называет нужную версию, и только потом ставит на место. Прерванная
// установка оставляет старый бинарник и убирает временный.
func (u updater) install(binary []byte, version string) error {
	info, err := os.Stat(u.exe)
	if err != nil {
		return err
	}

	dir := filepath.Dir(u.exe)
	temp, err := os.CreateTemp(dir, ".gitone-update-*")
	if errors.Is(err, fs.ErrPermission) {
		return i18n.New("update.noPermission", i18n.Args{"dir": dir, "exe": u.exe})
	}
	if err != nil {
		return err
	}
	placed := false
	defer func() {
		if !placed {
			os.Remove(temp.Name())
		}
	}()

	if err := writeExecutable(temp, binary, info.Mode().Perm()); err != nil {
		return err
	}
	if err := verify(temp.Name(), version); err != nil {
		return err
	}
	if err := replace(u.goos, temp.Name(), u.exe); err != nil {
		return err
	}

	placed = true
	return nil
}

func writeExecutable(file *os.File, binary []byte, mode fs.FileMode) error {
	_, err := file.Write(binary)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	return os.Chmod(file.Name(), mode)
}

// verify запускает новый бинарник с --version: битая сборка или чужая
// архитектура иначе встала бы на место, и обновиться было бы уже нечем.
func verify(path, version string) error {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return i18n.New("update.doesNotRun", i18n.Args{"err": err})
	}

	if got, want := strings.TrimSpace(string(out)), versionLine(version); got != want {
		return i18n.New("update.wrongVersion", i18n.Args{"got": got, "want": want})
	}
	return nil
}

// replace ставит новый файл на место старого. Rename поверх файла атомарен,
// но на Windows работающий exe не перезаписать: он уходит в .old, а при
// сбое возвращается. .old занят, пока старый gitone работает, поэтому его
// убирает следующий update.
func replace(goos, temp, exe string) error {
	if goos != "windows" {
		return os.Rename(temp, exe)
	}

	old := exe + ".old"
	if err := os.Remove(old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		// прошлый .old ещё занят другим запущенным gitone
		old = fmt.Sprintf("%s.%d.old", exe, os.Getpid())
	}
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(temp, exe); err != nil {
		return errors.Join(err, os.Rename(old, exe))
	}

	os.Remove(old)
	return nil
}

// newer сравнивает версии числами: 0.10.0 новее 0.9.0. Предрелиз младше
// релиза с теми же числами.
func newer(candidate, current string) bool {
	candidateCore, candidatePre, _ := strings.Cut(candidate, "-")
	currentCore, currentPre, _ := strings.Cut(current, "-")

	candidateParts, currentParts := strings.Split(candidateCore, "."), strings.Split(currentCore, ".")
	for index := range max(len(candidateParts), len(currentParts)) {
		if order := cmp.Compare(part(candidateParts, index), part(currentParts, index)); order != 0 {
			return order > 0
		}
	}

	switch {
	case candidatePre == currentPre:
		return false
	case candidatePre == "":
		return true
	case currentPre == "":
		return false
	}
	return candidatePre > currentPre
}

func part(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}

	number, _ := strconv.Atoi(parts[index])
	return number
}
