package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/layout"
	"gitone/internal/tui"
	"gitone/internal/tui/theme"
	"gitone/internal/watch"
)

// Первый git diff на большом репозитории идёт секунды, и пустой терминал
// всё это время выглядит зависшим. Короткая загрузка надписи не показывает.
const loadingNoticeDelay = 200 * time.Millisecond

// maxWatchedDirs ограничивает слежение: каждый каталог держит дескриптор, и
// большой репозиторий или node_modules без лимита съедают их все.
const maxWatchedDirs = 1000

func main() {
	if isVersionFlag(os.Args) {
		fmt.Println(versionLine(version))
		return
	}

	dir, err := os.Getwd()
	if err != nil {
		exit(err.Error())
	}

	opts, settingsErrs := loadSettings()
	opts.Dir, opts.Args = dir, os.Args[1:]
	opts.Notice = addNotice(opts.Text, "", settingsErrs...)

	// слежение живёт до конца программы, поэтому канал закрывается здесь
	done := make(chan struct{})
	defer close(done)
	if opts.Config.UI.AutoReload {
		changes, watchErr := watch.Start(dir, maxWatchedDirs, done)
		if watchErr != nil {
			opts.Notice = addNotice(opts.Text, opts.Notice, watchErr)
		} else {
			opts.Changes = changes
		}
	}

	model, err := loadWithNotice(opts)
	if err != nil {
		exit(opts.Text.Error(err))
	}

	if _, err := tea.NewProgram(model).Run(); err != nil {
		exit(opts.Text.Error(err))
	}
}

// loadSettings читает config.json, keybinds.json, тему и раскладку. Ошибка
// настроек не останавливает запуск: ошибочный файл заменяется встроенным
// значением, поэтому opts всегда пригоден. Ошибки возвращаются как есть, а
// текст из них собирается на языке интерфейса, который ещё не выбран.
func loadSettings() (tui.Options, []error) {
	opts := tui.Options{Config: config.Default(), Keys: keymap.Default(), Theme: theme.Dark(), Layout: layout.Default()}

	home, err := os.UserHomeDir()
	if err != nil {
		return opts, []error{err}
	}

	dir := config.Dir(os.Getenv("XDG_CONFIG_HOME"), home)
	settings, settingsErr := config.Load(dir)
	keys, keysErr := keymap.Load(dir)
	palette, themeErr := theme.Load(dir, settings.Theme)
	root, layoutErr := layout.Load(dir, settings.Layout)
	opts.Config, opts.Keys, opts.Theme, opts.Layout = settings, keys, palette, root

	return opts, []error{settingsErr, keysErr, themeErr, layoutErr}
}

// addNotice дописывает к тексту подвала тексты ошибок на языке интерфейса
// через «; »: подвал показывает одну строку. nil пропускается, иначе в тексте
// осталась бы пустая часть.
func addNotice(text i18n.Dictionary, notice string, errs ...error) string {
	var parts []string
	if notice != "" {
		parts = append(parts, notice)
	}
	for _, err := range errs {
		if err != nil {
			parts = append(parts, text.Error(err))
		}
	}

	return strings.Join(parts, "; ")
}

// Дифф грузится до запуска интерфейса: ошибка git (вне репозитория, неверный
// аргумент) должна остаться в терминале текстом, а не мелькнуть на экране,
// который сразу закроется.
func loadWithNotice(opts tui.Options) (tea.Model, error) {
	printed := make(chan struct{})
	timer := time.AfterFunc(loadingNoticeDelay, func() {
		fmt.Fprint(os.Stderr, opts.Text.Text("start.loadingDiff", nil))
		close(printed)
	})

	files, err := tui.Load(opts.Dir, opts.Args)
	staged, stagedErr := tui.Staged(opts.Dir)
	opts.Staged = staged
	opts.Notice = addNotice(opts.Text, opts.Notice, stagedErr)

	if !timer.Stop() {
		<-printed
		fmt.Fprint(os.Stderr, "\r\x1b[K")
	}

	return tui.New(opts, files), err
}

func exit(text string) {
	fmt.Fprintln(os.Stderr, text)
	os.Exit(1)
}
