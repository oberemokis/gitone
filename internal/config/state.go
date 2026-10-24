package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// stateFile это состояние gitone между запусками (#248). Его пишет
// приложение, а не пользователь, поэтому оно лежит отдельно от config.json:
// конфиг пользователя gitone не переписывает.
const stateFile = "state.json"

// recentLimit это сколько недавних репозиториев помнит состояние.
// ponytail: число зашито, настройкой станет, если двадцати не хватит.
const recentLimit = 20

// State это содержимое state.json: недавние репозитории, последний первым, и
// вид, переключённый клавишами.
type State struct {
	RecentRepos []string `json:"recentRepos"`
	View        View     `json:"view,omitzero"`
}

// View это вид, который меняют клавиши и помнит state.json (#369): diff.mode и
// diff.wrap поверх config.json, панель файлов и флаги диффа (#217). Saved
// время записи, нулевое значит, что вида нет.
type View struct {
	Mode             string `json:"mode"`
	Wrap             bool   `json:"wrap"`
	Files            bool   `json:"files"`
	IgnoreWhitespace bool   `json:"ignoreWhitespace"`
	// Context строки контекста вокруг хунков, -1 значит число git
	Context int       `json:"context"`
	Saved   time.Time `json:"saved"`
}

// ReadState читает state.json из каталога настроек. Файла нет, значит
// состояние пустое. Испорченный файл тоже пустое состояние: это не настройка
// пользователя, и следующая запись его перепишет.
func ReadState(dir string) State {
	data, err := os.ReadFile(filepath.Join(dir, stateFile))
	if err != nil {
		return State{}
	}

	var state State
	if json.Unmarshal(data, &state) != nil {
		return State{}
	}
	return state
}

// ReadView читает вид из state.json. Вида нет, он испорчен или config.json
// поправлен после его записи, значит нулевой View. Правка конфига главнее
// клавиш: иначе поправленный руками diff.wrap молча не сработал бы, а к auto
// после m не было бы пути.
func ReadView(dir string) View {
	view := ReadState(dir).View
	if view.Saved.IsZero() || !knownMode(view.Mode) || view.Context < -1 {
		return View{}
	}

	info, err := os.Stat(filepath.Join(dir, fileName))
	if err == nil && info.ModTime().After(view.Saved) {
		return View{}
	}
	return view
}

// SaveView записывает вид в state.json со временем записи, список недавних
// остаётся как был.
// ponytail: запись на каждое нажатие без очереди; два нажатия подряд, пока
// state.json держит другой gitone, могут лечь в обратном порядке. Очередь
// понадобится, если это заметят.
func SaveView(dir string, view View) error {
	view.Saved = time.Now()
	return updateState(dir, func(state *State) { state.View = view })
}

// RememberRepo ставит root первым в список недавних, без повтора и не дальше
// предела, и записывает state.json.
func RememberRepo(dir, root string) error {
	return updateState(dir, func(state *State) {
		state.RecentRepos = slices.DeleteFunc(state.RecentRepos, func(path string) bool { return path == root })
		state.RecentRepos = append([]string{root}, state.RecentRepos...)
		state.RecentRepos = state.RecentRepos[:min(len(state.RecentRepos), recentLimit)]
	})
}

// updateState меняет состояние функцией change и записывает state.json.
// Каталог настроек создаётся, если его нет. Запись идёт через временный файл:
// оборванная запись не портит состояние, а при отказе временный файл
// удаляется. Чтение и запись идут под блокировкой: два gitone, запущенных
// разом, иначе прочли бы одно состояние, и запись одного пропала бы. Не взяли
// блокировку, запись пропускается: состояние это удобство.
func updateState(dir string, change func(*State)) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	unlock, locked := lockState(dir)
	if !locked {
		return nil
	}
	defer unlock()

	state := ReadState(dir)
	change(&state)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	temp, err := os.CreateTemp(dir, stateFile+".*")
	if err != nil {
		return err
	}
	_, err = temp.Write(append(data, '\n'))
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temp.Name(), filepath.Join(dir, stateFile))
	}
	if err != nil {
		os.Remove(temp.Name())
	}
	return err
}

// staleLock это возраст, после которого блокировка считается оставленной
// упавшим процессом: запись state.json занимает миллисекунды.
const staleLock = 5 * time.Second

// lockState берёт state.json.lock, созданный с O_EXCL, и ждёт около секунды,
// если его держит другой gitone.
// ponytail: опрос раз в 25 мс, flock понадобится, если процессов станет много.
func lockState(dir string) (unlock func(), locked bool) {
	name := filepath.Join(dir, stateFile+".lock")
	for range 40 {
		file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			file.Close()
			return func() { os.Remove(name) }, true
		}
		if info, statErr := os.Stat(name); statErr == nil && time.Since(info.ModTime()) > staleLock {
			os.Remove(name)
			continue
		}
		time.Sleep(25 * time.Millisecond)
	}
	return nil, false
}
