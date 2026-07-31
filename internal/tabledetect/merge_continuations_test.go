package tabledetect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaintable "github.com/coregx/gxpdf/internal/models/table"
)

// --- helpers ---

// buildTable builds a domaintable.Table from a [][]string of cell texts.
// Row 0 = first row (header), last row = footer.
func buildTable(rows [][]string) *domaintable.Table {
	if len(rows) == 0 || len(rows[0]) == 0 {
		return nil
	}
	tbl, err := domaintable.NewTable(len(rows), len(rows[0]))
	if err != nil {
		panic(err)
	}
	for r, row := range rows {
		for c, text := range row {
			cell := domaintable.NewCellWithBounds(
				text, r, c,
				domaintable.NewRectangle(float64(c*100), float64(r*20), 100, 20),
			)
			if err := tbl.SetCell(r, c, cell); err != nil {
				panic(err)
			}
		}
	}
	return tbl
}

// cellText returns the text of cell (r, c), or "" if nil.
func cellText(tbl *domaintable.Table, r, c int) string {
	cell := tbl.GetCell(r, c)
	if cell == nil {
		return ""
	}
	return cell.Text
}

// --- stripTrailingVenueContamination ---

func TestStripTrailingVenueContamination_KnownVenueWords(t *testing.T) {
	tt := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip Annexes after section list",
			input: "U\nAnnexes",
			want:  "U",
		},
		{
			name:  "strip ANNEX uppercase regardless of comma",
			input: "A,B,C\nAnnex",
			want:  "A,B,C", // known venue word "ANNEX" is stripped even without trailing comma
		},
		{
			name:  "strip D after trailing-comma list",
			input: "A,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,Q,R,\nD",
			want:  "A,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,Q,R,",
		},
		{
			name:  "strip Building after trailing-comma list",
			input: "A,B,C,\nBuilding",
			want:  "A,B,C,",
		},
		{
			name:  "no strip: suffix has comma (multi-token)",
			input: "A,B,C\nD,E,F",
			want:  "A,B,C\nD,E,F",
		},
		{
			name:  "no strip: no newline",
			input: "A,B,C",
			want:  "A,B,C",
		},
		{
			name:  "no strip: suffix too long",
			input: "A,B,C,\nSomeLongVenueName",
			want:  "A,B,C,\nSomeLongVenueName",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := stripTrailingVenueContamination(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestStripTrailingVenueContamination_AllAsVenue(t *testing.T) {
	// "All" after trailing comma is venue text, not a section code.
	input := "A,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,Q,R,S,T,\nAll"
	got := stripTrailingVenueContamination(input)
	assert.Equal(t, "A,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,Q,R,S,T,", got)
}

func TestStripTrailingVenueContamination_NoStrip(t *testing.T) {
	// "U\nV" — both look like section codes, prefix doesn't end with comma.
	input := "U\nV"
	got := stripTrailingVenueContamination(input)
	assert.Equal(t, "U\nV", got)
}

// --- sectionsPrefixBeforeNewline ---

func TestSectionsPrefixBeforeNewline(t *testing.T) {
	tt := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single-line input: no newline",
			input: "A,B,C,D,E",
			want:  "",
		},
		{
			name:  "valid sections prefix before newline",
			input: "U,V\nA,B",
			want:  "U,V",
		},
		{
			name:  "non-section prefix: course title",
			input: "TECHNOLOGY IN LANGUAGE LEARNING\nA,B",
			want:  "",
		},
		{
			name:  "empty prefix",
			input: "\nA,B",
			want:  "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := sectionsPrefixBeforeNewline(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

// --- mergeTextContinuations (Pattern A) ---

func TestMergeTextContinuations_PatternA_HeaderSplit(t *testing.T) {
	// "SECTIONS\nA,B,C," in header row — sections chunk should go to next row.
	tbl := buildTable([][]string{
		{columnHeaderTime, "COURSE TITLE", "SECTIONS\nA,B,C,", ""},
		{"", "ENGLISH READING", "D,E,F", ""},
		{"", "ALGEBRA", "X", ""},
	})

	mergeTextContinuations(tbl)

	// Header cell: only "SECTIONS" remains.
	assert.Equal(t, "SECTIONS", cellText(tbl, 0, 2))
	// Next row gets the prepended sections chunk.
	assert.Equal(t, "A,B,C,D,E,F", cellText(tbl, 1, 2))
}

func TestMergeTextContinuations_PatternA_OnlyKnownHeaders(t *testing.T) {
	// "Annexes\nA,B,C" — "Annexes" is NOT a known header, should NOT be split.
	tbl := buildTable([][]string{
		{columnHeaderTime, "COURSE TITLE", "Annexes\nA,B,C", ""},
		{"", "ALGEBRA", "X,Y,Z", ""},
	})

	mergeTextContinuations(tbl)

	// Should be unchanged — "Annexes" is not in knownSectionsColumnHeaders.
	assert.Equal(t, "Annexes\nA,B,C", cellText(tbl, 0, 2))
}

// --- mergeTextContinuations (Pattern B) ---

func TestMergeTextContinuations_PatternB_OrphanPrefix(t *testing.T) {
	// Row 0: no title, sections = "B1,B2,B3"
	// Row 1: title = "ENGLISH", sections = ",C1,C2" (starts with comma → continuation)
	tbl := buildTable([][]string{
		{"", "", "B1,B2,B3", ""},
		{"", "ENGLISH READING SKILLS", ",C1,C2", ""},
		{"", "ALGEBRA", "X", ""},
	})

	mergeTextContinuations(tbl)

	// Row 0 sections moved to prefix of Row 1.
	assert.Equal(t, "", cellText(tbl, 0, 2))
	assert.Equal(t, "B1,B2,B3,C1,C2", cellText(tbl, 1, 2))
}

// --- mergeTextContinuations (Pattern C) ---

func TestMergeTextContinuations_PatternC_TrailingComma(t *testing.T) {
	// Row 0 ends with comma → Row 1 is continuation.
	tbl := buildTable([][]string{
		{"", "COURSE TITLE", "SECTIONS", ""},
		{"", "ENGLISH READING SKILLS", "A,B,C,D,E,F,", ""},
		{"", "", "G,H,I,J", ""},
		{"", "ALGEBRA", "X", ""},
	})

	mergeTextContinuations(tbl)

	assert.Equal(t, "A,B,C,D,E,F,G,H,I,J", cellText(tbl, 1, 2))
	assert.Equal(t, "", cellText(tbl, 2, 2))
	assert.Equal(t, "X", cellText(tbl, 3, 2))
}

func TestMergeTextContinuations_PatternC_MultiRowChain(t *testing.T) {
	// Chain: Row 0 → Row 1 → Row 2 (both end with comma until Row 2).
	tbl := buildTable([][]string{
		{"", "COURSE TITLE", "SECTIONS", ""},
		{"", "LONG COURSE NAME", "A,B,C,", ""},
		{"", "", "D,E,F,", ""},
		{"", "", "G,H,I", ""},
		{"", "OTHER COURSE", "X", ""},
	})

	mergeTextContinuations(tbl)

	assert.Equal(t, "A,B,C,D,E,F,G,H,I", cellText(tbl, 1, 2))
	assert.Equal(t, "", cellText(tbl, 2, 2))
	assert.Equal(t, "", cellText(tbl, 3, 2))
}

func TestMergeTextContinuations_PatternC_StopsAtTitle(t *testing.T) {
	// Row 0 ends with comma, Row 1 has its own full-length course title → stop.
	// Note: isCourseTitleText requires > 10 chars AND all-caps AND space.
	tbl := buildTable([][]string{
		{"", "COURSE TITLE", "SECTIONS", ""},
		{"", "INTRODUCTION TO ALGORITHMS", "A,B,C,", ""},
		{"", "PRINCIPLES OF MATHEMATICS", "D,E,F", ""},
	})

	mergeTextContinuations(tbl)

	// Should NOT merge into Row 2 because Row 2 has its own course title.
	// Row 1 keeps trailing comma (nothing was merged).
	assert.Equal(t, "A,B,C,", cellText(tbl, 1, 2))
	assert.Equal(t, "D,E,F", cellText(tbl, 2, 2))
}

func TestMergeTextContinuations_PatternC_Plus_VenueStrip(t *testing.T) {
	// Pattern C+: Row 1 c2 ends with "\nD" (venue contamination).
	// Stripping reveals trailing comma → Row 2 is continuation.
	tbl := buildTable([][]string{
		{"", "COURSE TITLE", "SECTIONS", ""},
		{"", "ELECTRONIC DEVICES", "A,AA,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,Q,R,\nD", ""},
		{"", "", "S,T,U,V,W,X,Y,Z", ""},
		{"", "NEXT COURSE", "W", ""},
	})

	mergeTextContinuations(tbl)

	// Row 1 should have A,...,R merged with S,...,Z (D was venue).
	result := cellText(tbl, 1, 2)
	assert.Contains(t, result, "A,AA,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,Q,R,")
	assert.Contains(t, result, "S,T,U,V,W,X,Y,Z")
	assert.Equal(t, "", cellText(tbl, 2, 2))
}

func TestMergeTextContinuations_PatternC_PartialHarvest(t *testing.T) {
	// Pattern C partial harvest: Row 1 ends with comma (after venue strip).
	// Row 2 has its own title, but c2 starts with continuation before a newline.
	tbl := buildTable([][]string{
		{"", "COURSE TITLE", "SECTIONS", ""},
		{"", "NUMERICAL METHODS FOR SCIENCE AND ENGINEERING", "A,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,Q,R,S,T,", ""},
		{"", "TECHNOLOGY IN LANGUAGE LEARNING", "U,V\nA,B", ""},
	})

	mergeTextContinuations(tbl)

	// Row 1 should get "U,V" prepended.
	result := cellText(tbl, 1, 2)
	assert.Contains(t, result, "U,V")
	// Row 2 c2 should have "U,V" removed, leaving only "A,B".
	row2 := cellText(tbl, 2, 2)
	assert.Equal(t, "A,B", row2)
}

// --- mergeTextContinuations (Pattern D) ---

func TestMergeTextContinuations_PatternD_OrphanBeforeTitle(t *testing.T) {
	// Row N has no title, Row N+1 has a title.
	// Row N's sections belong to Row N+1.
	tbl := buildTable([][]string{
		{"", "COURSE TITLE", "SECTIONS", ""},
		{"", "BASICS IN SOCIAL SCIENCE", "A,A[ARCH],B,B", ""},
		{"", "", "[ARCH],C,D,E,F,G,H,H1,H2,I,J", ""},
		{"", "DESIGN THEORY II", "A", ""},
		{"", "NEXT COURSE", "X", ""},
	})

	mergeTextContinuations(tbl)

	// Row 2 (orphan) sections should be prepended to Row 3 (DESIGN THEORY II).
	assert.Equal(t, "", cellText(tbl, 2, 2))
	result := cellText(tbl, 3, 2)
	assert.Contains(t, result, "[ARCH],C,D,E,F,G,H,H1,H2,I,J")
}

func TestMergeTextContinuations_PatternD_SkipsWhenC0HasCourse(t *testing.T) {
	// Pattern D should NOT fire when c0 (TIME column) has a course title.
	// The c2 at that row belongs to the c0 course, not the next c1 course.
	tbl := buildTable([][]string{
		{"", "COURSE TITLE", "SECTIONS", ""},
		{"URBAN DESIGN I\nPHYSICS 1", "", "A", ""},
		{"", "URBAN DESIGN II", "A,B", ""},
	})

	mergeTextContinuations(tbl)

	// Row 1 c2 "A" should NOT be moved to Row 2.
	assert.Equal(t, "A", cellText(tbl, 1, 2))
	assert.Equal(t, "A,B", cellText(tbl, 2, 2))
}

func TestMergeTextContinuations_PatternD_SkipsWhenPrevEndsWithComma(t *testing.T) {
	// Pattern D should NOT fire if the previous row ends with comma
	// (Row N is already a Pattern C continuation).
	tbl := buildTable([][]string{
		{"", "COURSE TITLE", "SECTIONS", ""},
		{"", "PREVIOUS COURSE", "A,B,C,", ""},
		{"", "", "D,E,F", ""},
		{"", "NEXT COURSE NAME", "X", ""},
	})

	mergeTextContinuations(tbl)

	// Row 2 is a Pattern C continuation of Row 1 → should merge there, not via Pattern D.
	result := cellText(tbl, 1, 2)
	assert.Contains(t, result, "A,B,C,D,E,F")
	assert.Equal(t, "", cellText(tbl, 2, 2))
}

// --- No-op guards ---

func TestMergeTextContinuations_NilTable(t *testing.T) {
	require.NotPanics(t, func() { mergeTextContinuations(nil) })
}

func TestMergeTextContinuations_SingleRow(t *testing.T) {
	tbl := buildTable([][]string{{"", "COURSE", "A,B,C", ""}})
	require.NotPanics(t, func() { mergeTextContinuations(tbl) })
	assert.Equal(t, "A,B,C", cellText(tbl, 0, 2))
}

func TestMergeTextContinuations_SingleCol(t *testing.T) {
	tbl := buildTable([][]string{{"A"}, {"B"}, {"C"}})
	require.NotPanics(t, func() { mergeTextContinuations(tbl) })
}

func TestMergeTextContinuations_EmptyTable(t *testing.T) {
	tbl := buildTable([][]string{{"", "", "", ""}, {"", "", "", ""}})
	require.NotPanics(t, func() { mergeTextContinuations(tbl) })
}

// --- isTitleSectionsPair ---

func TestIsTitleSectionsPair_True(t *testing.T) {
	// Typical schedule: c0=TIME, c1=COURSE TITLE, c2=SECTIONS.
	rows := make([][]string, 20)
	for i := range rows {
		if i == 0 {
			rows[i] = []string{"", "COURSE TITLE", "SECTIONS", ""}
		} else {
			rows[i] = []string{"", "INTRODUCTION TO MATHEMATICS", "A,B,C,D,E", ""}
		}
	}
	tbl := buildTable(rows)
	assert.True(t, isTitleSectionsPair(tbl, 1, 2))
}

func TestIsTitleSectionsPair_False_WrongColumns(t *testing.T) {
	// c0=TIME, c1=COURSE TITLE: checking (0, 1) pair won't qualify as title|sections.
	rows := make([][]string, 15)
	for i := range rows {
		rows[i] = []string{"9:00AM", "INTRODUCTION TO MATHEMATICS", "A,B,C,D", ""}
	}
	tbl := buildTable(rows)
	// (0,1): c0=time (not title), c1=titles — fails sections check on c1.
	assert.False(t, isTitleSectionsPair(tbl, 0, 1))
}

func TestIsTitleSectionsPair_False_TooFewRows(t *testing.T) {
	tbl := buildTable([][]string{
		{"", "MATHEMATICS", "A,B", ""},
		{"", "PHYSICS", "C,D", ""},
	})
	// Only 2 rows — sampleSize < 3.
	assert.False(t, isTitleSectionsPair(tbl, 1, 2))
}

// --- looksLikeSectionsContinuation ---

func TestLooksLikeSectionsContinuation(t *testing.T) {
	tt := []struct {
		input string
		want  bool
	}{
		{"A,B,C,D,E", true},
		{"A,AA,B,BB,C,CC", true},
		{"[ARCH],C,D,E", true},
		{"All", true},
		{"SECTIONS", false},
		{"VENUE", false},
		{"COURSE TITLE", false},
		{"", false},
		{"A\nBuilding", false},   // 50% valid → below 75%
		{"A,B\nBuilding", false}, // 67% → below 75%
		// "A,B,C\nBuilding" → parts A(✓),B(✓),C(✓),Building(✗) = 75% → passes threshold
		// but stripTrailingVenueContamination removes "Building" before check in practice
		{"A,B,C\nBuilding", true}, // 75% = threshold, passes looksLike
		{"U,V,W,X,Y,Z", true},
		{"B1,B2,B3,B4,B5", true},
	}

	for _, tc := range tt {
		t.Run(tc.input, func(t *testing.T) {
			got := looksLikeSectionsContinuation(tc.input)
			assert.Equal(t, tc.want, got, "looksLikeSectionsContinuation(%q)", tc.input)
		})
	}
}

// --- isSectionCode ---

func TestIsSectionCode(t *testing.T) {
	tt := []struct {
		input string
		want  bool
	}{
		{"A", true},
		{"Z", true},
		{"A1", true},
		{"C10", true},
		{"AA", true},
		{"BB", true},
		{"[ARCH]", true},
		{"[FST/FE]", true},
		{"All", true},
		{"", false},
		{"SECTIONS", false},
		{"Building", false},
		{"1A", false},    // starts with digit
		{"ABCDE", false}, // > 4 chars, no bracket
		{"a", false},     // lowercase
		{"ab", false},    // lowercase
	}

	for _, tc := range tt {
		t.Run(tc.input, func(t *testing.T) {
			got := isSectionCode(tc.input)
			assert.Equal(t, tc.want, got, "isSectionCode(%q)", tc.input)
		})
	}
}
