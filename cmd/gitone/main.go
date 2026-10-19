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
)

// Первый git diff на большом репозитории идёт секунды, и пустой терминал
// всё это время выглядит зависшим. Короткая загрузка надписи не показывает.
const loadingNoticeDelay = 200 * time.Millisecond

// pollInterval как externalChangeCheckInterval у lazygit: правка видна почти
// сразу, а git diff рабочей копии дешёвый, git сверяет файлы по индексу.
const pollInterval = 2 * time.Second

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
	opts.Args = os.Args[1:]

	// Язык выбирается после чтения config.json, поэтому ошибки настроек ещё не
	// стали текстом: они выйдут на выбранном языке.
	text, languageErr := i18n.Choose(opts.Config.Language, os.Getenv("LC_ALL"), os.Getenv("LC_MESSAGES"), os.Getenv("LANG"))
	opts.Text = text

	// ошибки настроек обновлению не мешают: битая тема не повод не обновиться
	if isUpdateCommand(os.Args) {
		runUpdate(text)
		return
	}
	opts.Notice = addNotice(text, "", append(settingsErrs, languageErr)...)

	// git отвечает на языке интерфейса, если у него есть такой перевод.
	// LANGUAGE меняет только язык сообщений и не трогает кодировку, поэтому
	// хукам пользователя при коммите она безопаснее LC_ALL.
	if err := os.Setenv("LANGUAGE", text.Lang()); err != nil {
		opts.Notice = addNotice(text, opts.Notice, err)
	}

	if opts.Config.UI.AutoReload {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		opts.Ticks = ticker.C
	}

	model, err := loadWithNotice(opts, dir)
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
	// state.json с недавними репозиториями лежит рядом с конфигом (#248)
	opts.StateDir = dir

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

// Репозиторий каталога dir открывается и дифф грузится до запуска интерфейса:
// ошибка git (вне репозитория, неверный аргумент) должна остаться в терминале
// текстом, а не мелькнуть на экране, который сразу закроется.
func loadWithNotice(opts tui.Options, dir string) (tea.Model, error) {
	repo, err := tui.Open(dir)
	if err != nil {
		return nil, err
	}
	opts.Repo = repo

	printed := make(chan struct{})
	timer := time.AfterFunc(loadingNoticeDelay, func() {
		fmt.Fprint(os.Stderr, opts.Text.Text("start.loadingDiff", nil))
		close(printed)
	})

	files, err := tui.Load(repo, opts.Args)
	index, stagedErr := tui.ReadIndex(repo)
	opts.Index = index
	// ветку и worktree перечитывание найдёт снова, а ошибку диффа Load уже вернул
	opts.Location, _ = tui.Locate(repo)
	opts.Notice = addNotice(opts.Text, opts.Notice, stagedErr)

	if !timer.Stop() {
		<-printed
		fmt.Fprint(os.Stderr, "\r\x1b[K")
	}

	return tui.New(opts, files), err
}

// exit печатает ошибку до запуска интерфейса. Текст может нести то, чем
// управляется терминал (отказ git повторяет имя ветки или вывод хука), поэтому
// он проходит ту же очистку, что и поля в окнах.
func exit(text string) {
	fmt.Fprintln(os.Stderr, tui.Printable(text))
	os.Exit(1)
}
