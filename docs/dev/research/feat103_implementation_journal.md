# feat-103: Implementation Journal

**Date**: 2026-07-31 — 2026-08-02
**Goal**: sections accuracy 85.9% → 100% (311/311)
**Result**: 97.7% (304/311), 7 remaining
**Branch**: feat/multiline-cell-text (14 commits)

---

## Progression

```
85.9% (263/306) → baseline (v0.9.1 released)
90.5% (277/306) → mergeTextContinuations patterns A-D
91.0% (283/311) → two-pass extraction + ground truth verified
96.1% (→ reverted) → Pattern D with isCourseTitleText (content-aware, rejected)
97.4% (303/311) → findCourseRow longest-section preference
97.7% (304/311) → Pattern F split + venue cleanup ← STABLE CEILING
```

---

## What Worked

### 1. Two-Pass Extraction (91.0%)
**Commit**: 9acbbcc
**What**: Non-merged cells extracted FIRST, merged cells SECOND.
**Why it works**: Courses (non-merged col 1) get priority for text assignment. Merged TIME cells (col 0) only get remaining text. Prevents TIME cells from "stealing" course titles at X=52.
**Impact**: +8% (283→291)

### 2. Pattern F: Split Mixed Title+Sections (97.4%)
**Commit**: 12c1b0c
**What**: `splitTitleAndSections()` separates course titles from section codes in mixed newline-separated cells.
**Why it works**: Grid edge extension creates oversized last/first rows that capture text from multiple visual rows. Pattern F detects ALL-CAPS multi-word titles mixed with comma-separated codes and splits them.
**Impact**: +6% (283→303)

### 3. Venue Contamination Cleanup (97.7%)
**Commit**: d870a98
**What**: `cleanVenueContamination()` removes "All", "Annexes", "&", "Building" from sections cells.
**Why it works**: VENUE column text bleeds into SECTIONS column due to adjacent merged cells. Cleanup pass runs after all continuation merging.
**Impact**: +0.3% (303→304)

### 4. Intersection-Based Grid (ADR-005)
**Commit**: 0855332 + dfffcc9
**What**: `BuildGridFromIntersections` uses 4-corner cell verification from tabula-java.
**Why it works**: Produces per-column cells — col 0 (TIME): 3 tall cells, col 3 (VENUE): 1 cell. Correctly detects partial H-lines (X=[89.5, 508.1]) that don't span all columns.
**Impact on accuracy**: 0% (same 97.7%) — because col 2 H-lines DO intersect col 2 V-lines.
**Architectural value**: correct enterprise approach, benefits PDFs with partial H-lines.

### 5. Ground Truth Verification (7 DeepSeek errors found)
**Commits**: 250829d, 7aa33b4
**What**: User provided 10 PDF screenshots. Found 7 errors in DeepSeek ground truth:
- PRINCIPLES OF ECONOMICS: shifted sections across 3 courses
- BUSINESS COMMUNICATION: "DD" → "D,D" OCR split
- OOP2: "BB" → "B,B" OCR split
- DESIGN AND SIMULATION: title "IEA" → "IE", sections "D" → "A"

---

## What Failed and Why

### 1. Visual Pre-Grouping (52-54%)
**What**: Group TextElements by X/Y proximity BEFORE cell extraction (tabula-java approach).
**Regression**: 97.7% → 52.7% (massive)
**Root cause**: Groups consecutive courses' sections together (same X, close Y). No reliable way to stop grouping at course boundaries without ruling line awareness. Grid-aware grouping also conflicted with post-processing patterns C/D/E/F which expect per-row extraction.
**Lesson**: Pre-grouping requires REMOVING post-processing patterns, not adding on top.

### 2. CenterY = Y for Negative Height (89.7%)
**What**: Return Y instead of Y+Height/2 for negative-Height elements.
**Regression**: 97.7% → 89.7% (25 cases)
**Root cause**: Changes containment for ALL elements, not just boundary cases. Elements above cells (Y > cell top) now have CenterY=Y > cell top → fall in cell ABOVE instead of their own cell. Creates cascading shift: sections from cell N assigned to cell N-1 → cell N loses content → cell N+1 inherits wrong sections.
**Lesson**: Global CenterY change affects ALL cells, not just the 7 target cases.

### 3. Y-Baseline Fallback (89.7%)
**What**: For negative-Height elements whose center is below cell, also check if Y is inside.
**Regression**: 97.7% → 89.7%
**Root cause**: Elements at cell boundary have Y inside BOTH this cell AND the cell above. With 2pt padding on each side, cells overlap by 4pt. Y=413.8 is inside expanded bounds of BOTH cell 23 [399.9, 416.9] and cell 24 [412.9, 429.9]. Stateful extraction assigns to first-processed cell.
**Lesson**: Any fallback containment point creates double-assignment at boundaries.

### 4. Overlap Check with Threshold (97.4%)
**What**: Check if element bounding box overlaps cell by ≥10%/30%.
**Regression**: 97.7% → 97.4% (10% threshold), P1 fails at 30%
**Root cause**: P1 overlap = 0.9pt/9pt = 10%. At 10% threshold, other elements' tiny overlaps with adjacent cells also match → pull wrong text. At 30% threshold, P1 excluded.
**Lesson**: Grid rows 13pt, text height 9pt → overlap fractions too small to distinguish.

### 5. Increased Padding (97.4%)
**What**: cellBoundsPadding 2pt → 6pt.
**Regression**: 97.7% → 97.4%
**Root cause**: 6pt padding on 13pt rows = 46% overlap. Adjacent cells overlap by 12pt out of 13pt → almost complete overlap. Elements at boundaries assigned to wrong cell.
**Lesson**: Padding can't exceed ~1/3 of row height without causing adjacent-cell conflicts.

### 6. Pattern G: Orphan Merge UP (96.1%)
**What**: Orphan sections rows (no title) merged UP to previous titled row.
**Regression**: 97.7% → 96.1% (5 cases)
**Root cause**: Too aggressive — sections from course N+1's row merged into course N when course N+1 has no visible title (title in merged TIME cell). Can't distinguish "orphan belonging UP" from "sections belonging to merged-cell course."
**Lesson**: Merge direction (UP vs DOWN) requires knowing which course the sections belong to — fundamentally content-aware.

### 7. Pattern D with isCourseTitleText (96.1%)
**What**: Pattern D orphan check uses `isCourseTitleText()` instead of non-empty.
**Regression**: 97.7% → 96.1%
**Root cause**: TIME labels like "3:00 PM" fail `isCourseTitleText` → treated as orphans → their sections moved to wrong course. Pattern D was designed for empty-title rows, not TIME-label rows.
**User rejection**: content-aware functions (`isCourseTitleText`, `isTimeLabelText`) are document-specific. Different PDF = different content types. NOT enterprise.

---

## The 7 Remaining Cases (Root Cause)

All 7 share ONE geometric root cause: **text positioned between ruling lines that create 13pt grid rows**.

The PDF draws thin filled rectangles as table borders. `RulingLineDetector` decomposes these into individual ruling lines. Adjacent rectangles' edges create phantom boundaries at 13pt intervals WITHIN visual cells that span multiple text lines.

Example: ENGLISH READING SKILLS row is visually ~39pt tall (3 text lines). But the filled rectangles from adjacent rows create H-lines at Y=414.9, 427.9, 440.8 — three 13pt boundaries inside what is visually ONE cell.

**Fix**: `RulingLineDetector` should detect when two adjacent thin rectangles share an edge and NOT create a ruling line there. The shared edge is a border between cells' visual borders, not a cell separator.

**Why intersection grid didn't help**: Interior H-lines at X=[89.5, 508.1] DO intersect col 2 V-lines at X=334.8 and X=507.6 → intersection-based grid still creates 13pt rows for col 2.

---

## Architecture Decisions

- **Content-aware logic REJECTED**: `isCourseTitleText`, `isTimeLabelText`, `isSections` — all document-specific. User explicitly prohibited.
- **Intersection grid ACCEPTED** (ADR-005): correct enterprise architecture, benefits PDFs with partial H-lines.
- **Post-processing patterns**: necessary with current grid, but should be simplified once RulingLineDetector is fixed.
- **LLM-assisted detection**: documented as future vision in CLAUDE.md.

---

## Next Steps

1. **Fix RulingLineDetector**: don't create phantom H-lines from adjacent rectangle edges
2. **Simplify mergeTextContinuations**: with correct grid rows, many patterns become unnecessary
3. **Release v0.9.2**: with 97.7% + intersection grid + golden tests
4. **Target 100%**: after RulingLineDetector fix
