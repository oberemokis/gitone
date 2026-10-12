# gitone

Терминальное приложение для работы с git на Go и Bubble Tea. Горячие клавиши, раскладки, видимость элементов и тема настраиваются через JSON. Дифф показывается в видах split и unified.

Проект в подготовке. Сейчас приложение показывает `hello world` по центру экрана и выходит по `q`.

## Команды

```
make build   сборка в bin/gitone
make run     запуск
make test    тесты
make lint    go vet и проверка gofmt
make fmt     gofmt
```

## Документы

- [docs/product.md](docs/product.md): что делаем и чего не делаем
- [docs/architecture.md](docs/architecture.md): стек, пакеты, граф зависимостей, правила против связанности
- [docs/config.md](docs/config.md): форматы JSON для клавиш, раскладок, тем и видимости
- [docs/diff.md](docs/diff.md): виды диффа и что показываем

## Задачи

План, решения и открытые вопросы живут в задачах на GitHub, порядок в [дорожной карте #1](https://github.com/oberemokis/gitone/issues/1). Изменение приходит в `main` только пул-реквестом с зелёной проверкой CI. Порядок веток и коммитов описан в [CLAUDE.md](CLAUDE.md).
