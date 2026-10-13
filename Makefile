.PHONY: build run test lint fmt

# Грамматики tree-sitter для подсветки задаёт scripts/grammar-tags.sh, тот же
# список берут CI и релиз (#139). $(shell) теряет код выхода скрипта, поэтому
# сбой виден только по пустому списку: без тегов сборка берёт все двести
# грамматик, копилефтные тоже, и всё остаётся зелёным.
TAGS := $(shell sh scripts/grammar-tags.sh)
ifeq ($(TAGS),)
$(error scripts/grammar-tags.sh не напечатал теги грамматик)
endif
export GOFLAGS := -tags=$(TAGS)

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
