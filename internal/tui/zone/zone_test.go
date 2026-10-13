package zone_test

import (
	"image"
	"testing"

	"charm.land/lipgloss/v2"

	"gitone/internal/tui/zone"
)

// Метки не занимают места: место клика начинается с колонки своего текста,
// а рисунок без меток тот же, что без них был бы нарисован.
func TestScanFindsMarkedText(t *testing.T) {
	content := "ab " + zone.Mark("first", "cd") + "\n" + lipgloss.NewStyle().Bold(true).Render("x") + zone.Mark("second", "yz")

	clean, zones := zone.Scan(content)

	if want := "ab cd\n" + lipgloss.NewStyle().Bold(true).Render("x") + "yz"; clean != want {
		t.Fatalf("рисунок без меток %q, want %q", clean, want)
	}
	want := []zone.Zone{
		{Target: "first", Area: image.Rect(3, 0, 5, 1)},
		{Target: "second", Area: image.Rect(1, 1, 3, 2)},
	}
	if len(zones) != len(want) || zones[0] != want[0] || zones[1] != want[1] {
		t.Fatalf("места %+v, want %+v", zones, want)
	}
}

// Место, которое переносится на следующую строку, кончается на краю строки и
// продолжается с начала следующей.
func TestScanSplitsZoneAcrossLines(t *testing.T) {
	_, zones := zone.Scan(zone.Mark("both", "ab\ncde"))

	want := []zone.Zone{
		{Target: "both", Area: image.Rect(0, 0, 2, 1)},
		{Target: "both", Area: image.Rect(0, 1, 3, 2)},
	}
	if len(zones) != len(want) || zones[0] != want[0] || zones[1] != want[1] {
		t.Fatalf("места %+v, want %+v", zones, want)
	}
}

// At находит место под точкой, первое в списке выигрывает.
func TestAtPrefersFirstZone(t *testing.T) {
	zones := []zone.Zone{
		{Target: "window", Area: image.Rect(2, 0, 6, 3)},
		{Target: "below", Area: image.Rect(0, 1, 10, 2)},
	}

	if got := zone.At(zones, 3, 1, nil); got != "window" {
		t.Fatalf("под (3,1) %q, want window", got)
	}
	if got := zone.At(zones, 8, 1, nil); got != "below" {
		t.Fatalf("под (8,1) %q, want below", got)
	}
	if got := zone.At(zones, 8, 0, nil); got != "" {
		t.Fatalf("под (8,0) %q, want пусто", got)
	}
}

// Просторное место ловит клик на клетку вокруг себя, но точное попадание в
// другое место сильнее, а место с пустой целью закрывает то, что после него
// (#134).
func TestAtWidensRoomyZones(t *testing.T) {
	roomy := func(target string) bool { return target == "button" || target == "outside" }
	zones := []zone.Zone{
		{Target: "button", Area: image.Rect(4, 4, 6, 5)},
		{Target: "row", Area: image.Rect(0, 5, 10, 6)},
		{Area: image.Rect(0, 0, 10, 8)},
		{Target: "outside", Area: image.Rect(0, 8, 4, 9)},
	}

	for _, tc := range []struct {
		x, y int
		want string
	}{
		{x: 3, y: 3, want: "button"},
		{x: 6, y: 4, want: "button"},
		{x: 5, y: 5, want: "row"},
		{x: 8, y: 2, want: ""},
		{x: 2, y: 7, want: ""},
		{x: 4, y: 9, want: "outside"},
	} {
		if got := zone.At(zones, tc.x, tc.y, roomy); got != tc.want {
			t.Errorf("под (%d,%d) %q, want %q", tc.x, tc.y, got, tc.want)
		}
	}
}
