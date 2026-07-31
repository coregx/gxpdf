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

		// Build map: course title → sections text from extraction
		courseSections := make(map[string]string)
		for _, tbl := range tables {
			for r := 0; r < tbl.RowCount(); r++ {
				// Column 1 = COURSE TITLE, Column 2 = SECTIONS (4-col table)
				if tbl.ColumnCount() >= 3 {
					title := strings.TrimSpace(tbl.Cell(r, 1))
					sections := strings.TrimSpace(tbl.Cell(r, 2))
					if title != "" {
						courseSections[title] = sections
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
				if extractedSections == expectedSections {
					totalMatched++
				} else {
					t.Errorf("Day %d, %q: sections mismatch\n  want: %s\n  got:  %s",
						day.Day, course.Title, expectedSections, extractedSections)
				}
			}
		}
	}

	if totalChecked > 0 {
		t.Logf("Sections: %d/%d matched (%.1f%%)",
			totalMatched, totalChecked, float64(totalMatched)/float64(totalChecked)*100)
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

		// Count non-empty course titles extracted
		extractedCourses := 0
		for _, tbl := range tables {
			for r := 0; r < tbl.RowCount(); r++ {
				if tbl.ColumnCount() >= 2 {
					title := strings.TrimSpace(tbl.Cell(r, 1))
					if title != "" && title != "COURSE TITLE" && !strings.HasPrefix(title, "Day ") {
						extractedCourses++
					}
				}
			}
		}

		expectedCourses := 0
		for _, slot := range day.Slots {
			expectedCourses += len(slot.Courses)
		}

		if extractedCourses != expectedCourses {
			t.Errorf("Day %d: course count want %d, got %d (delta=%+d)",
				day.Day, expectedCourses, extractedCourses, extractedCourses-expectedCourses)
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
