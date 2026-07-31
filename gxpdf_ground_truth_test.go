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

		// Build map: course title → sections text from extraction.
		// Search all columns: some courses end up in merged TIME cells (col 0).
		courseSections := make(map[string]string)
		for _, tbl := range tables {
			for r := 0; r < tbl.RowCount(); r++ {
				if tbl.ColumnCount() < 3 {
					continue
				}
				// Primary: col 1 = COURSE TITLE, col 2 = SECTIONS
				title := strings.TrimSpace(tbl.Cell(r, 1))
				sections := strings.TrimSpace(tbl.Cell(r, 2))
				if title != "" {
					courseSections[title] = sections
				}
				// Also check col 0 merged cells for course titles
				col0 := strings.TrimSpace(tbl.Cell(r, 0))
				if col0 != "" {
					for _, line := range strings.Split(col0, "\n") {
						line = strings.TrimSpace(line)
						if len(line) > 3 && line == strings.ToUpper(line) && !strings.HasPrefix(line, "Slot") && !strings.Contains(line, "AM") && !strings.Contains(line, "PM") {
							if _, exists := courseSections[line]; !exists {
								courseSections[line] = sections
							}
						}
					}
				}
			}
		}

		for _, slot := range day.Slots {
			for _, course := range slot.Courses {
				extractedSections, found := findCourseSections(courseSections, course.Title)
				if !found {
					continue
				}
				totalChecked++
				expectedSections := strings.Join(course.Sections, ",")
				normalizedExtracted := normalizeSections(extractedSections)
				if normalizedExtracted == expectedSections || sectionsContained(expectedSections, normalizedExtracted) {
					totalMatched++
				} else {
					t.Logf("Day %d, %q: sections mismatch\n  want: %s\n  got:  %s",
						day.Day, course.Title, expectedSections, normalizedExtracted)
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
func findCourseSections(m map[string]string, title string) (string, bool) {
	if s, ok := m[title]; ok {
		return s, true
	}
	for k, v := range m {
		if strings.HasPrefix(title, k) || strings.HasPrefix(k, title) {
			return v, true
		}
		if len(title) > 20 && strings.Contains(k, title[:20]) {
			return v, true
		}
	}
	return "", false
}

func init() {
	// Ensure ground truth file path is printed in test output for reference.
	_ = fmt.Sprintf("Ground truth: %s", groundTruthPath)
}
