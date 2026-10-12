package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

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

	model, err := loadWithNotice(dir, os.Args[1:])
	if err != nil {
		exit(err)
	}

	if _, err := tea.NewProgram(model).Run(); err != nil {
		exit(err)
	}
}

// Дифф грузится до запуска интерфейса: ошибка git (вне репозитория, неверный
// аргумент) должна остаться в терминале текстом, а не мелькнуть на экране,
// который сразу закроется.
func loadWithNotice(dir string, args []string) (tea.Model, error) {
	printed := make(chan struct{})
	timer := time.AfterFunc(loadingNoticeDelay, func() {
		fmt.Fprint(os.Stderr, "Loading diff…")
		close(printed)
	})

	files, err := tui.Load(dir, args)

	if !timer.Stop() {
		<-printed
		fmt.Fprint(os.Stderr, "\r\x1b[K")
	}

	return tui.New(dir, args, files), err
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
