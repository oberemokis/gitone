#!/bin/sh
# Ставит gitone на macOS и Linux одной командой:
#
#   curl -fsSL https://raw.githubusercontent.com/oberemokis/gitone/main/install.sh | sh
#
# Переменные окружения:
#   GITONE_VERSION      версия, например v0.4.0; по умолчанию последняя
#   GITONE_INSTALL_DIR  каталог; по умолчанию ~/.local/bin
#   GITONE_RELEASES_URL адрес релизов, его подменяет CI локальным сервером
#
# Скрипт ставит версии с v0.4.0: в более старых релизах нет бинарника без
# версии в имени. PATH он не меняет и sudo не зовёт.
#
# Всё тело в функциях, а вызов стоит последней строкой: оборванная загрузка
# через curl | sh иначе выполнила бы половину скрипта.

set -eu

main() {
	base="${GITONE_RELEASES_URL:-https://github.com/oberemokis/gitone/releases}"
	dir="${GITONE_INSTALL_DIR:-$HOME/.local/bin}"
	version="${GITONE_VERSION:-}"
	asset="gitone_$(detect_os)_$(detect_arch)"

	if [ -n "$version" ]; then
		case "$version" in
		v*) ;;
		*) version="v$version" ;;
		esac
		url="$base/download/$version"
	else
		url="$base/latest/download"
	fi

	tmp="$(mktemp -d)"
	staged=""
	trap cleanup EXIT
	trap 'exit 1' INT TERM

	fetch "$url/$asset" "$tmp/$asset"
	fetch "$url/checksums.txt" "$tmp/checksums.txt"
	verify "$tmp" "$asset"

	# Временное имя в целевом каталоге: mv внутри каталога атомарен, и в PATH
	# никогда не лежит недокачанный или неисполняемый gitone.
	mkdir -p "$dir"
	staged="$dir/.gitone.$$"
	cp "$tmp/$asset" "$staged"
	chmod 755 "$staged"
	mv "$staged" "$dir/gitone"
	staged=""

	say "$("$dir/gitone" --version) installed to $dir/gitone"
	check_path "$dir"
}

cleanup() {
	rm -rf "$tmp"
	if [ -n "$staged" ]; then
		rm -f "$staged"
	fi
}

detect_os() {
	case "$(uname -s)" in
	Darwin) echo darwin ;;
	Linux) echo linux ;;
	*) fail "unsupported system: $(uname -s); on Windows use install.ps1" ;;
	esac
}

# Под Rosetta uname -m отдаёт x86_64, хотя машина arm64: ставим родную сборку.
detect_arch() {
	machine="$(uname -m)"
	if [ "$machine" = x86_64 ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || true)" = 1 ]; then
		machine=arm64
	fi

	case "$machine" in
	x86_64 | amd64) echo amd64 ;;
	arm64 | aarch64) echo arm64 ;;
	*) fail "unsupported architecture: $machine" ;;
	esac
}

fetch() {
	curl -fsSL --retry 3 -o "$2" "$1" ||
		fail "cannot download $1 (only gitone v0.4.0 and later install this way)"
}

# verify сверяет бинарник со строкой своего файла в checksums.txt: сам
# checksums.txt перечисляет все файлы релиза, и sha256sum -c споткнулся бы о
# те, что не скачаны.
verify() {
	want="$(awk -v name="$2" '$2 == name { print $1 }' "$1/checksums.txt")"
	if [ -z "$want" ]; then
		fail "checksums.txt has no line for $2, nothing was installed"
	fi

	if command -v sha256sum >/dev/null 2>&1; then
		got="$(sha256sum "$1/$2" | awk '{ print $1 }')"
	elif command -v shasum >/dev/null 2>&1; then
		got="$(shasum -a 256 "$1/$2" | awk '{ print $1 }')"
	else
		fail "neither sha256sum nor shasum found, cannot verify the download"
	fi

	if [ "$got" != "$want" ]; then
		fail "checksum of $2 does not match checksums.txt, nothing was installed"
	fi
}

check_path() {
	case ":$PATH:" in
	*":$1:"*) ;;
	*) say "$1 is not in PATH, add it to your shell profile: export PATH=\"$1:\$PATH\"" ;;
	esac

	found="$(command -v gitone 2>/dev/null || true)"
	if [ -n "$found" ] && [ "$found" != "$1/gitone" ]; then
		say "another gitone comes first in PATH: $found"
	fi
}

say() {
	echo "$*"
}

fail() {
	echo "gitone install: $*" >&2
	exit 1
}

main "$@"
