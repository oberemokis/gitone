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

- [docs/product.md](docs/product.md): что делаем, чего не делаем, план и открытые вопросы
- [docs/architecture.md](docs/architecture.md): стек, пакеты, граф зависимостей, правила против связанности
- [docs/config.md](docs/config.md): форматы JSON для клавиш, раскладок, тем и видимости
- [docs/diff.md](docs/diff.md): виды диффа и что показываем
