package extractor

import (
	"strings"
	"testing"
)

// TestCellExtractor_NegativeHeight_P1 reproduces the ENGLISH READING SKILLS
// extraction bug: text "P1" has Y=413.8 Height=-9, so CenterY=409.3.
// The cell bounds are Y=[414.9, 427.9]. P1's center (409.3) is 5.6pt below
// the cell bottom — outside even with 2pt padding.
//
// This is a pure geometric bug: negative Height shifts center below Y,
// making text at the cell boundary unreachable.
func TestCellExtractor_NegativeHeight_P1(t *testing.T) {
	t.Skip("Known limitation: P1 center 5.6pt below cell boundary. Requires refactor-007 (intersection-based grid)")
	elements := []*TextElement{
		// ENGLISH READING sections - line 1 (inside cell)
		NewTextElement("B1,B2,B3,B4,B5,B6,B7,B8,C1,C10,C2,C3", 337.9, 434.5, 200, -9, "F1", 9),
		// ENGLISH READING sections - line 2 (inside cell)
		NewTextElement(",C4,C5,C6,C7,C8,C9,D1,E1,E2,H1,H2,J1,", 337.4, 424.2, 200, -9, "F1", 9),
		// ENGLISH READING sections - line 3: P1 (just below cell boundary)
		NewTextElement("P1", 415.7, 413.8, 15, -9, "F1", 9),
		// Next course sections (should NOT be captured)
		NewTextElement("A", 418.2, 402.2, 10, -9, "F1", 9),
		// Previous course sections (should NOT be captured)
		NewTextElement("A", 418.2, 446.2, 10, -9, "F1", 9),
	}

	ce := NewCellExtractor(elements)

	// Cell bounds matching ENGLISH READING's grid cell
	bounds := NewRectangle(334.8, 414.9, 172.8, 13.0)

	content := ce.ExtractCellContent(bounds)

	p1 := elements[2]
	t.Logf("Cell bounds: Y=[%.1f, %.1f]", bounds.Y, bounds.Y+bounds.Height)
	t.Logf("P1 element (normalized): Y=%.1f, Height=%.1f, CenterY=%.1f", p1.Y, p1.Height, p1.CenterY())
	t.Logf("Expanded bounds: Y=[%.1f, %.1f]", bounds.Y-cellBoundsPadding, bounds.Y+bounds.Height+cellBoundsPadding)
	t.Logf("Extracted: %q", content)

	if !strings.Contains(content, "P1") {
		t.Errorf("P1 not found in extracted content.\n"+
			"P1 CenterY (%.1f) is %.1fpt below cell bottom (%.1f).\n"+
			"With padding=%.1f, expanded bottom=%.1f.\n"+
			"Need: CenterY >= expanded bottom, but %.1f < %.1f",
			409.3, 414.9-409.3, 414.9,
			cellBoundsPadding, bounds.Y-cellBoundsPadding,
			409.3, bounds.Y-cellBoundsPadding)
	}

	if !strings.Contains(content, "B1") {
		t.Error("B1 (line 1) should be in content")
	}
	if !strings.Contains(content, "J1") {
		t.Error("J1 (line 2) should be in content")
	}

	// Verify adjacent course sections NOT captured
	// "A" at Y=402.2 should NOT be in content (different course)
	parts := strings.Split(content, "\n")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "A" {
			t.Error("Standalone 'A' from adjacent course should NOT be captured")
		}
	}
}
