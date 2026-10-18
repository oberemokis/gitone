package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// stateFile это состояние gitone между запусками (#248). Его пишет
// приложение, а не пользователь, поэтому оно лежит отдельно от config.json:
// конфиг пользователя gitone не переписывает.
const stateFile = "state.json"

// recentLimit это сколько недавних репозиториев помнит состояние.
// ponytail: число зашито, настройкой станет, если двадцати не хватит.
const recentLimit = 20

// State это содержимое state.json: недавние репозитории, последний первым.
type State struct {
	RecentRepos []string `json:"recentRepos"`
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

// RememberRepo ставит root первым в список недавних, без повтора и не дальше
// предела, и записывает state.json. Каталог настроек создаётся, если его нет.
// Запись идёт через временный файл: оборванная запись не портит состояние.
func RememberRepo(dir, root string) error {
	state := ReadState(dir)
	state.RecentRepos = slices.DeleteFunc(state.RecentRepos, func(path string) bool { return path == root })
	state.RecentRepos = append([]string{root}, state.RecentRepos...)
	state.RecentRepos = state.RecentRepos[:min(len(state.RecentRepos), recentLimit)]

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	temp, err := os.CreateTemp(dir, stateFile+".*")
	if err != nil {
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(temp.Name())
		return err
	}
	return os.Rename(temp.Name(), filepath.Join(dir, stateFile))
}
