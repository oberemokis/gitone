package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
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
	dir, err := os.Getwd()
	if err != nil {
		exit(err)
	}

	// Ошибка настроек не останавливает запуск: работают встроенные значения,
	// а текст ошибки стоит в подвале.
	opts, settingsErr := loadSettings()
	opts.Dir, opts.Args = dir, os.Args[1:]
	if settingsErr != nil {
		opts.Notice = settingsErr.Error()
	}

	// слежение живёт до конца программы, поэтому канал закрывается здесь
	done := make(chan struct{})
	defer close(done)
	if opts.Config.UI.AutoReload {
		changes, watchErr := watch.Start(dir, maxWatchedDirs, done)
		if watchErr != nil {
			opts.Notice = joinNotices(opts.Notice, watchErr.Error())
		} else {
			opts.Changes = changes
		}
	}

	model, err := loadWithNotice(opts)
	if err != nil {
		exit(err)
	}

	if _, err := tea.NewProgram(model).Run(); err != nil {
		exit(err)
	}
}

// loadSettings читает config.json, keybinds.json, тему и раскладку. Ошибки всех
// склеиваются в одну строку: подвал показывает одну первую строку. Ошибочный
// файл заменяется встроенным значением, поэтому opts всегда пригоден.
func loadSettings() (tui.Options, error) {
	opts := tui.Options{Config: config.Default(), Keys: keymap.Default(), Theme: theme.Dark(), Layout: layout.Default()}

	home, err := os.UserHomeDir()
	if err != nil {
		return opts, err
	}

	dir := config.Dir(os.Getenv("XDG_CONFIG_HOME"), home)
	settings, settingsErr := config.Load(dir)
	keys, keysErr := keymap.Load(dir)
	palette, themeErr := theme.Load(dir, settings.Theme)
	root, layoutErr := layout.Load(dir, settings.Layout)
	opts.Config, opts.Keys, opts.Theme, opts.Layout = settings, keys, palette, root

	var problems []string
	for _, err := range []error{settingsErr, keysErr, themeErr, layoutErr} {
		if err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) == 0 {
		return opts, nil
	}

	return opts, errors.New(strings.Join(problems, "; "))
}

// joinNotices склеивает тексты ошибок в одну строку подвала.
func joinNotices(notices ...string) string {
	var present []string
	for _, notice := range notices {
		if notice != "" {
			present = append(present, notice)
		}
	}

	return strings.Join(present, "; ")
}

// Дифф грузится до запуска интерфейса: ошибка git (вне репозитория, неверный
// аргумент) должна остаться в терминале текстом, а не мелькнуть на экране,
// который сразу закроется.
func loadWithNotice(opts tui.Options) (tea.Model, error) {
	printed := make(chan struct{})
	timer := time.AfterFunc(loadingNoticeDelay, func() {
		fmt.Fprint(os.Stderr, "Loading diff…")
		close(printed)
	})

	files, err := tui.Load(opts.Dir, opts.Args)
	staged, stagedErr := tui.Staged(opts.Dir)
	opts.Staged = staged
	opts.Notice = joinNotices(opts.Notice, errorText(stagedErr))

	if !timer.Stop() {
		<-printed
		fmt.Fprint(os.Stderr, "\r\x1b[K")
	}

	return tui.New(opts, files), err
}

// errorText нужен там, где ошибки может не быть: nil в тексте стал бы пустой
// строкой подвала.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
