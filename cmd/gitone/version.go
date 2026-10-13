package main

// version вшивается при сборке релиза: go build -ldflags "-X main.version=1.2.3".
// Без этого остаётся dev, и сборка из исходников узнаётся по нему.
var version = "dev"

// isVersionFlag узнаёт `gitone --version` и `gitone -v`. Флаг учитывается только
// один и только первым: остальные аргументы идут в git diff как есть. Контракт
// между версиями: `update` прошлых версий запускает новый бинарник с
// `--version`, флаг убирать нельзя.
func isVersionFlag(args []string) bool {
	return len(args) == 2 && (args[1] == "--version" || args[1] == "-v")
}

// versionLine это вывод `gitone --version`. Этот вывод читает `update` прошлых
// версий и сверяет его со своей строкой «gitone X.Y.Z»: формат менять нельзя,
// иначе у всех, кто стоит на старой версии, `gitone update` откажется
// ставить новую («reports … instead of …, nothing was installed»).
func versionLine(v string) string {
	return "gitone " + v
}
