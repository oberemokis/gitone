package tui

import "testing"

// У каждого окна есть описание с разбором клавиш: иначе нажатие в нём ушло бы
// на главный экран (#352).
func TestWindowKindsComplete(t *testing.T) {
	for shown := noWindow + 1; shown < windowEnd; shown++ {
		if kind, found := windowKinds[shown]; !found || kind.key == nil {
			t.Errorf("у окна %d нет записи с key в windowKinds", shown)
		}
	}
	if _, found := windowKinds[noWindow]; found {
		t.Error("у noWindow есть запись: главный экран разбирает клавиши сам")
	}
}
