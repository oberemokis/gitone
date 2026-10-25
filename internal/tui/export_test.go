package tui

// Шаги спиннеров для внешних тестов: части пакета команд они выбирают по
// типу ответа, а не по месту в пакете, и перестановка аргументов tea.Batch
// их не роняет.
type (
	SpinMsg       = spinMsg
	RemoteSpinMsg = remoteSpinMsg
)
