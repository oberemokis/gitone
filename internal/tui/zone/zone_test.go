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

	if got := zone.At(zones, 3, 1); got != "window" {
		t.Fatalf("под (3,1) %q, want window", got)
	}
	if got := zone.At(zones, 8, 1); got != "below" {
		t.Fatalf("под (8,1) %q, want below", got)
	}
	if got := zone.At(zones, 8, 0); got != "" {
		t.Fatalf("под (8,0) %q, want пусто", got)
	}
}
