package main

// version вшивается при сборке релиза: go build -ldflags "-X main.version=1.2.3".
// Без этого остаётся dev, и сборка из исходников узнаётся по нему.
var version = "dev"

// isVersionFlag узнаёт `gitone --version` и `gitone -v`. Флаг учитывается только
// один и только первым: остальные аргументы идут в git diff как есть.
func isVersionFlag(args []string) bool {
	return len(args) == 2 && (args[1] == "--version" || args[1] == "-v")
}

func versionLine(v string) string {
	return "gitone " + v
}
