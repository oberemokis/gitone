package tui

import (
	"cmp"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/tui/diffview"
)

// Однофайловые компоненты (Vue, Svelte, Astro) красятся по файлу целиком
// (#140): по хунку без открывающего тега не понять, скрипт в нём или стиль.
// Корень читает обе версии командой и отдаёт их диффу, подсветка ложится в
// кеш диффа по ключу из хэшей. Пока версий нет или их не прочитать, подсветка
// по хункам.

// versionsMsg несёт обе версии файла path по ключу. Ошибка оставляет
// подсветку по хункам.
type versionsMsg struct {
	key, path string
	old, new  []byte
	err       error
}

func versionsCmd(dir, key string, file diff.File) tea.Cmd {
	return func() tea.Msg {
		path := cmp.Or(file.NewPath, file.OldPath)
		old, err := git.Version(dir, file.OldBlob, "")
		if err != nil {
			return versionsMsg{key: key, path: path, err: err}
		}

		new, err := git.Version(dir, file.NewBlob, file.NewPath)
		return versionsMsg{key: key, path: path, old: old, new: new, err: err}
	}
}

// versionKey это ключ версий файла из их хэшей. Пусто, если версии файлу не
// нужны: язык не однофайловый компонент или хэшей в диффе нет.
func versionKey(file diff.File) string {
	if !diffview.NeedsVersions(file) {
		return ""
	}
	return file.OldBlob + ".." + file.NewBlob
}

// storeVersions кладёт прочитанные версии в кеш диффа. Не прочитались, в кеш
// идёт пустая запись: иначе чтение повторялось бы на каждое сообщение.
func (m Model) storeVersions(msg versionsMsg) Model {
	if m.reading == msg.key {
		m.reading = ""
	}
	if msg.err != nil {
		msg.old, msg.new = nil, nil
	}

	m.view = m.view.StoreVersions(msg.key, msg.path, msg.old, msg.new)
	return m
}

// wantVersions раскрашивает открытый файл версиями из кеша, а без них просит
// их прочитать, один запрос на ключ за раз. Зовётся после каждого сообщения:
// файл открывают и перечитывают многие пути, и ни один не должен забыть про
// версии.
func (m Model) wantVersions(cmd tea.Cmd) (Model, tea.Cmd) {
	if len(m.files) == 0 {
		return m, cmd
	}

	file := m.files[m.current]
	key := versionKey(file)
	if key == "" || m.view.VersionKey() == key {
		return m, cmd
	}
	if view, found := m.view.UseVersions(key); found {
		m.view = view
		return m, cmd
	}
	if m.reading == key {
		return m, cmd
	}

	m.reading = key
	return m, tea.Batch(cmd, versionsCmd(m.dir, key, file))
}
