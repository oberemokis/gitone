.PHONY: build run test lint fmt

# Грамматики tree-sitter для подсветки: сто распространённых языков без
# грамматик под копилефтом. Без тегов в бинарник уходят все двести, с GPL
# среди них (решение в #101). Те же теги стоят в .goreleaser.yaml.
export GOFLAGS := -tags=grammar_set_core,gotreesitter_no_copyleft

build:
	go build -o bin/gitone ./cmd/gitone

run:
	go run ./cmd/gitone

# -race нужен коду с горутинами (watch), -shuffle=on ловит зависимость тестов
# от порядка, сид go test печатает сам. Локально флаги переопределяются:
# make test TESTFLAGS=.
TESTFLAGS ?= -race -shuffle=on

# Без кеша: architecture_test.go спрашивает граф у go list, и кеш go test не
# знает, что результат зависит от импортов других пакетов. Из кеша он
# показывал «ok» при нарушенном графе.
test:
	go test -count=1 $(TESTFLAGS) ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

fmt:
	gofmt -w .
