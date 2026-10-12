// Package watch сообщает, что в рабочей копии что-то изменилось. Пакет ничего не
// знает о диффе и интерфейсе: он отдаёт сигналы, а перечитывание решает корень.
package watch

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// debounce схлопывает события в одном окне в один сигнал: при сохранении
// редактор пишет файл несколько раз.
const debounce = 200 * time.Millisecond

// ErrTooManyDirs значит, что каталогов больше лимита. На macOS каждый
// наблюдаемый каталог держит дескриптор, поэтому большой репозиторий или
// node_modules слежение не тянет.
var ErrTooManyDirs = errors.New("слежение выключено: много каталогов")

// Start наблюдает каталоги root рекурсивно и присылает сигнал после каждой пачки
// изменений. Каталог .git и скрытые каталоги не наблюдаются. Если каталогов
// больше maxDirs, слежение не стартует. Если лимит превысит каталог, созданный
// уже при работе, канал закрывается. Остановка по закрытию done.
func Start(root string, maxDirs int, done <-chan struct{}) (<-chan struct{}, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	tree := &tree{watcher: watcher, root: root, limit: maxDirs, strict: true}
	if err := tree.addAll(root); err != nil {
		watcher.Close()
		return nil, err
	}

	signals := make(chan struct{}, 1)
	go tree.loop(signals, done)

	return signals, nil
}

// tree держит наблюдателя и счётчик каталогов. Счётчик общий для стартового
// обхода и каталогов, созданных позже.
type tree struct {
	watcher *fsnotify.Watcher
	root    string
	limit   int
	count   int
	// strict при старте: любая ошибка наблюдения фатальна. Во время работы
	// каталог, который пропал или не открылся, просто не наблюдаем.
	strict bool
}

func (t *tree) addAll(dir string) error {
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if t.strict || path == t.root {
				return err
			}
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if path != t.root && skipped(entry.Name()) {
			return filepath.SkipDir
		}

		t.count++
		if t.count > t.limit {
			return ErrTooManyDirs
		}
		if err := t.watcher.Add(path); err != nil && (t.strict || path == t.root) {
			return err
		}
		return nil
	})
}

// loop ждёт события, пропускает игнорируемые и после окна тишины шлёт сигнал.
// Новый каталог наблюдается сразу, до сигнала: иначе запись в него сразу после
// создания потерялась бы.
func (t *tree) loop(signals chan<- struct{}, done <-chan struct{}) {
	defer close(signals)
	defer t.watcher.Close()

	for {
		select {
		case <-done:
			return
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}
			relevant, err := t.relevant(event)
			if err != nil {
				return
			}
			if !relevant {
				continue
			}

			if !t.debounced(done) {
				return
			}
			select {
			case signals <- struct{}{}:
			default:
			}
		case _, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// debounced ждёт окно тишины и возвращает false, если пора остановиться.
func (t *tree) debounced(done <-chan struct{}) bool {
	timer := time.NewTimer(debounce)
	defer timer.Stop()

	for {
		select {
		case <-done:
			return false
		case <-timer.C:
			return true
		case event, ok := <-t.watcher.Events:
			if !ok {
				return false
			}
			if _, err := t.relevant(event); err != nil {
				return false
			}
		}
	}
}

// relevant решает, важно ли событие, и наблюдает новый каталог. Игнорируемые
// пути не важны. Ошибка возвращается только при переполнении лимита: остальное
// во время работы не останавливает наблюдение.
func (t *tree) relevant(event fsnotify.Event) (bool, error) {
	rel, err := filepath.Rel(t.root, event.Name)
	if err != nil || ignoredPath(rel) {
		return false, nil
	}

	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := t.addAll(event.Name); errors.Is(err, ErrTooManyDirs) {
				return false, err
			}
		}
	}

	return true, nil
}

// ignoredPath ищет в пути скрытый элемент, в том числе .git.
func ignoredPath(rel string) bool {
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part != "." && skipped(part) {
			return true
		}
	}

	return false
}

func skipped(name string) bool {
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}
