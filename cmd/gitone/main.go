package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/tui"
)

// Первый git diff на большом репозитории идёт секунды, и пустой терминал
// всё это время выглядит зависшим. Короткая загрузка надписи не показывает.
const loadingNoticeDelay = 200 * time.Millisecond

func main() {
	dir, err := os.Getwd()
	if err != nil {
		exit(err)
	}

	// Ошибка настроек не останавливает запуск: работают встроенные значения,
	// а текст ошибки стоит в подвале.
	settings, settingsErr := loadSettings()
	opts := tui.Options{Dir: dir, Args: os.Args[1:], Config: settings}
	if settingsErr != nil {
		opts.Notice = settingsErr.Error()
	}

	model, err := loadWithNotice(opts)
	if err != nil {
		exit(err)
	}

	if _, err := tea.NewProgram(model).Run(); err != nil {
		exit(err)
	}
}

func loadSettings() (config.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config.Default(), err
	}

	return config.Load(config.Dir(os.Getenv("XDG_CONFIG_HOME"), home))
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

	if !timer.Stop() {
		<-printed
		fmt.Fprint(os.Stderr, "\r\x1b[K")
	}

	return tui.New(opts, files), err
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
