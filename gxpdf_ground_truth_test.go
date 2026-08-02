package gxpdf

// Ground truth tests compare GxPDF extraction against a verified reference
// produced by DeepSeek from the same PDF. The reference JSON captures the
// semantic content: day → slot (time + venue) → courses (title + sections).
//
// Unlike cell-level golden tests (gxpdf_golden_test.go) which snapshot the
// raw grid output, ground truth tests validate WHAT was extracted, not HOW
// cells are laid out. A refactoring that changes grid structure but preserves
// course titles and sections should still pass these tests.
//
// Ground truth: testdata/golden/issue79_ground_truth.json
// Source: DeepSeek conversion of testdata/pdfs/issue79/sample.pdf

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// GroundTruth is the top-level structure of the DeepSeek reference JSON.
type GroundTruth struct {
	Schedule []GroundTruthDay `json:"schedule"`
}

// GroundTruthDay represents one day (= one PDF page).
type GroundTruthDay struct {
	Day   int               `json:"day"`
	Date  *string           `json:"date"`
	Slots []GroundTruthSlot `json:"slots"`
}

// GroundTruthSlot represents one time slot within a day.
type GroundTruthSlot struct {
	Time    string              `json:"time"`
	Venue   string              `json:"venue"`
	Courses []GroundTruthCourse `json:"courses"`
}

// GroundTruthCourse represents one course with its sections.
type GroundTruthCourse struct {
	Title    string   `json:"title"`
	Sections []string `json:"sections"`
}

const groundTruthPath = "testdata/golden/issue79_ground_truth.json"
const groundTruthPDF = "testdata/pdfs/issue79/sample.pdf"

func loadGroundTruth(t *testing.T) GroundTruth {
	t.Helper()
	data, err := os.ReadFile(groundTruthPath)
	if err != nil {
		t.Fatalf("read ground truth: %v", err)
	}
	var gt GroundTruth
	if err := json.Unmarshal(data, &gt); err != nil {
		t.Fatalf("unmarshal ground truth: %v", err)
	}
	return gt
}

// TestGroundTruth_CourseTitles verifies that every course title from the
// DeepSeek reference is present in our extraction output, page by page.
func TestGroundTruth_CourseTitles(t *testing.T) {
	if _, err := os.Stat(groundTruthPDF); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", groundTruthPDF)
	}

	gt := loadGroundTruth(t)
	doc, err := Open(groundTruthPDF)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close()

	if doc.PageCount() < len(gt.Schedule) {
		t.Fatalf("PDF has %d pages, ground truth has %d days", doc.PageCount(), len(gt.Schedule))
	}

	totalExpected := 0
	totalFound := 0
	totalMissing := 0

	for dayIdx, day := range gt.Schedule {
		// Extract all text from the page's Lattice table
		page := doc.Page(dayIdx)
		tables, err := page.ExtractTablesWithOptions(&ExtractionOptions{
			Method: MethodLattice,
		})
		if err != nil {
			t.Errorf("Day %d: ExtractTablesWithOptions: %v", day.Day, err)
			continue
		}

		// Collect all non-empty cell texts from all tables on this page
		var pageTexts []string
		for _, tbl := range tables {
			for r := 0; r < tbl.RowCount(); r++ {
				for c := 0; c < tbl.ColumnCount(); c++ {
					text := strings.TrimSpace(tbl.Cell(r, c))
					if text != "" {
						pageTexts = append(pageTexts, text)
					}
				}
			}
		}
		pageContent := strings.Join(pageTexts, "\n")

		// Check each expected course title
		for _, slot := range day.Slots {
			for _, course := range slot.Courses {
				totalExpected++
				if containsCourseTitle(pageContent, course.Title) {
					totalFound++
				} else {
					totalMissing++
					t.Errorf("Day %d, %s: MISSING course %q",
						day.Day, slot.Time, course.Title)
				}
			}
		}
	}

	t.Logf("Ground truth: %d/%d courses found (%.1f%%), %d missing",
		totalFound, totalExpected, float64(totalFound)/float64(totalExpected)*100, totalMissing)
}

// TestGroundTruth_CourseSections verifies that sections for each found course
// match the ground truth.
//
// Strategy: for each course title in ground truth, find the ROW where it
// appears in the extracted table (any column), then read sections from
// col 2 of THAT row. This handles courses in merged TIME cells (col 0).
func TestGroundTruth_CourseSections(t *testing.T) {
	if _, err := os.Stat(groundTruthPDF); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", groundTruthPDF)
	}

	gt := loadGroundTruth(t)
	doc, err := Open(groundTruthPDF)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close()

	totalChecked := 0
	totalMatched := 0

	for dayIdx, day := range gt.Schedule {
		page := doc.Page(dayIdx)
		tables, err := page.ExtractTablesWithOptions(&ExtractionOptions{
			Method: MethodLattice,
		})
		if err != nil {
			continue
		}
		if len(tables) == 0 {
			continue
		}
		tbl := tables[0]

		for _, slot := range day.Slots {
			for _, course := range slot.Courses {
				// Find the row containing this course title (any column)
				row := findCourseRow(tbl, course.Title)
				if row < 0 {
					continue
				}

				totalChecked++
				expectedSections := strings.Join(course.Sections, ",")

				// Read sections from col 2 of the found row
				extractedSections := ""
				if tbl.ColumnCount() > 2 {
					extractedSections = strings.TrimSpace(tbl.Cell(row, 2))
				}
				normalizedExtracted := normalizeSections(extractedSections)

				if normalizedExtracted == expectedSections || sectionsContained(expectedSections, normalizedExtracted) {
					totalMatched++
				} else {
					t.Logf("Day %d, %q (row %d): sections mismatch\n  want: %s\n  got:  %s",
						day.Day, course.Title, row, expectedSections, normalizedExtracted)
				}
			}
		}
	}

	if totalChecked > 0 {
		pct := float64(totalMatched) / float64(totalChecked) * 100
		t.Logf("Sections: %d/%d matched (%.1f%%)", totalMatched, totalChecked, pct)
		if pct < 80 {
			t.Errorf("Sections accuracy %.1f%% is below 80%% threshold", pct)
		}
	}
}

// findCourseRow finds the best row index where a course title appears.
//
// Strategy: find ALL rows containing the title, prefer the one where
// col 2 (SECTIONS) is non-empty. This handles cases where the same
// course appears on multiple rows (e.g. in merged TIME cell on one row
// and in regular COURSE TITLE column on another).
func findCourseRow(tbl *Table, title string) int {
	var candidates []int
	for r := 0; r < tbl.RowCount(); r++ {
		for c := 0; c < tbl.ColumnCount(); c++ {
			text := tbl.Cell(r, c)
			matched := strings.Contains(text, title)
			if !matched && len(title) > 20 {
				matched = strings.Contains(text, title[:20])
			}
			if matched {
				candidates = append(candidates, r)
				break
			}
		}
	}
	if len(candidates) == 0 {
		return -1
	}
	// Prefer row with longest section-like content in col 2.
	// Longer = more likely complete (vs truncated first occurrence).
	bestRow := candidates[0]
	bestLen := -1
	for _, r := range candidates {
		if tbl.ColumnCount() > 2 {
			sect := strings.TrimSpace(tbl.Cell(r, 2))
			if isVenueText(sect) {
				continue
			}
			if len(sect) > bestLen {
				bestLen = len(sect)
				bestRow = r
			}
		}
	}
	return bestRow
}

// TestGroundTruth_PageCount verifies we extract the right number of pages.
func TestGroundTruth_PageCount(t *testing.T) {
	if _, err := os.Stat(groundTruthPDF); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", groundTruthPDF)
	}

	gt := loadGroundTruth(t)
	doc, err := Open(groundTruthPDF)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close()

	if doc.PageCount() != len(gt.Schedule) {
		t.Errorf("PageCount: want %d (days), got %d", len(gt.Schedule), doc.PageCount())
	}
}

// TestGroundTruth_CourseCountPerSlot checks course counts per slot.
func TestGroundTruth_CourseCountPerSlot(t *testing.T) {
	if _, err := os.Stat(groundTruthPDF); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", groundTruthPDF)
	}

	gt := loadGroundTruth(t)
	doc, err := Open(groundTruthPDF)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close()

	for dayIdx, day := range gt.Schedule {
		page := doc.Page(dayIdx)
		tables, err := page.ExtractTablesWithOptions(&ExtractionOptions{
			Method: MethodLattice,
		})
		if err != nil {
			continue
		}

		// Count courses found via title matching (same as CourseTitles test).
		var pageTexts []string
		for _, tbl := range tables {
			for r := 0; r < tbl.RowCount(); r++ {
				for c := 0; c < tbl.ColumnCount(); c++ {
					text := strings.TrimSpace(tbl.Cell(r, c))
					if text != "" {
						pageTexts = append(pageTexts, text)
					}
				}
			}
		}
		pageContent := strings.Join(pageTexts, "\n")

		expectedCourses := 0
		foundCourses := 0
		for _, slot := range day.Slots {
			for _, course := range slot.Courses {
				expectedCourses++
				if containsCourseTitle(pageContent, course.Title) {
					foundCourses++
				}
			}
		}

		if foundCourses != expectedCourses {
			t.Errorf("Day %d: course count want %d, got %d (delta=%+d)",
				day.Day, expectedCourses, foundCourses, foundCourses-expectedCourses)
		}
	}
}

// TestGroundTruth_Summary prints an overall extraction quality report.
func TestGroundTruth_Summary(t *testing.T) {
	if _, err := os.Stat(groundTruthPDF); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", groundTruthPDF)
	}

	gt := loadGroundTruth(t)
	doc, err := Open(groundTruthPDF)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close()

	totalCourses := 0
	foundCourses := 0

	for dayIdx, day := range gt.Schedule {
		page := doc.Page(dayIdx)
		tables, err := page.ExtractTablesWithOptions(&ExtractionOptions{
			Method: MethodLattice,
		})
		if err != nil {
			continue
		}

		var pageTexts []string
		for _, tbl := range tables {
			for r := 0; r < tbl.RowCount(); r++ {
				for c := 0; c < tbl.ColumnCount(); c++ {
					text := strings.TrimSpace(tbl.Cell(r, c))
					if text != "" {
						pageTexts = append(pageTexts, text)
					}
				}
			}
		}
		pageContent := strings.Join(pageTexts, "\n")

		dayCourses := 0
		dayFound := 0
		for _, slot := range day.Slots {
			for _, course := range slot.Courses {
				dayCourses++
				totalCourses++
				if containsCourseTitle(pageContent, course.Title) {
					dayFound++
					foundCourses++
				}
			}
		}
		t.Logf("Day %d: %d/%d courses (%.0f%%)", day.Day, dayFound, dayCourses,
			float64(dayFound)/float64(dayCourses)*100)
	}

	pct := float64(foundCourses) / float64(totalCourses) * 100
	t.Logf("TOTAL: %d/%d courses extracted (%.1f%%)", foundCourses, totalCourses, pct)

	if pct < 80 {
		t.Errorf("Extraction quality %.1f%% is below 80%% threshold", pct)
	}
}

// isVenueText returns true if text looks like VENUE content, not sections.
func isVenueText(s string) bool {
	lower := strings.ToLower(s)
	venueWords := []string{"all", "annexes", "building", "venue", "sections"}
	for _, w := range venueWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// normalizeSections cleans extracted sections text for comparison.
// Removes newlines (from multi-line cells), collapses spaces,
// strips trailing commas and whitespace.
func normalizeSections(s string) string {
	s = strings.ReplaceAll(s, "\n", ",")
	s = strings.ReplaceAll(s, " ", "")
	// Collapse multiple commas
	for strings.Contains(s, ",,") {
		s = strings.ReplaceAll(s, ",,", ",")
	}
	s = strings.TrimRight(s, ",")
	s = strings.TrimLeft(s, ",")
	return s
}

// sectionsContained checks if all expected sections appear in the extracted text.
// Handles cases where extracted text has extra sections from bleeding.
func sectionsContained(expected, extracted string) bool {
	if extracted == "" {
		return expected == ""
	}
	expectedParts := strings.Split(expected, ",")
	for _, part := range expectedParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(extracted, part) {
			return false
		}
	}
	return true
}

// containsCourseTitle checks if pageContent contains the course title.
// Handles truncation (our extraction truncates long titles with "...").
func containsCourseTitle(pageContent, title string) bool {
	if strings.Contains(pageContent, title) {
		return true
	}
	// Try prefix match for truncated titles
	if len(title) > 30 {
		prefix := title[:30]
		if strings.Contains(pageContent, prefix) {
			return true
		}
	}
	// Try first 20 chars
	if len(title) > 20 {
		prefix := title[:20]
		if strings.Contains(pageContent, prefix) {
			return true
		}
	}
	return false
}

// findCourseSections finds sections for a course title in the extracted map.
// Handles truncated titles.

// TestGroundTruth_ColumnSeparation verifies that columns are not merged.
// Regression test for #86: expandBoundsIntoMergedNeighbors was pulling
// header text from adjacent columns.
func TestGroundTruth_ColumnSeparation(t *testing.T) {
	if _, err := os.Stat(groundTruthPDF); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", groundTruthPDF)
	}

	doc, err := Open(groundTruthPDF)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close()

	page := doc.Page(0)
	tables, err := page.ExtractTablesWithOptions(&ExtractionOptions{Method: MethodLattice})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("no tables")
	}
	tbl := tables[0]

	// Column 1 should never contain "TIME" as standalone text
	// Column 2 should never contain "VENUE" as standalone text
	for r := 0; r < tbl.RowCount(); r++ {
		col1 := tbl.Cell(r, 1)
		col2 := tbl.Cell(r, 2)
		if strings.Contains(col1, "TIME") && !strings.Contains(col1, "OVERTIME") {
			t.Errorf("Row %d col 1 contains TIME text (should be in col 0): %q", r, col1)
		}
		if strings.HasPrefix(strings.TrimSpace(col2), "VENUE") {
			t.Errorf("Row %d col 2 starts with VENUE text (should be in col 3): %q", r, col2)
		}
	}
}

func init() {
	// Ensure ground truth file path is printed in test output for reference.
	_ = fmt.Sprintf("Ground truth: %s", groundTruthPath)
}
