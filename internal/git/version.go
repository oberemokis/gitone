package git

import "os"

// Version читает версию файла по хэшу из строки index диффа: подсветке
// однофайловых компонентов нужен файл целиком (#140). Объекта с хэшем новой
// версии правки вне индекса в git нет, git его только вычисляет: тогда, если
// задан путь от корня репозитория, читается файл рабочего дерева. Пустой хэш
// значит, что версии нет.
func (r Repo) Version(hash, path string) ([]byte, error) {
	if hash == "" {
		return nil, nil
	}

	content, err := r.inRoot("cat-file", "blob", hash)
	if err == nil {
		return []byte(content), nil
	}
	if path == "" {
		return nil, failed("version", err)
	}

	name, err := r.file(path)
	if err != nil {
		return nil, failed("version", err)
	}
	data, err := os.ReadFile(name)
	return data, failed("version", err)
}
