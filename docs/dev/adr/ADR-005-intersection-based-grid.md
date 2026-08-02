# ADR-005: Intersection-Based Grid Construction

**Date**: 2026-08-02
**Status**: Accepted
**Supersedes**: implicit sorted-coordinate grid in BuildGrid()

---

## Problem

Current `BuildGrid()` constructs grid from **sorted Y/X arrays** of ruling line coordinates. Each unique Y becomes a row boundary, each unique X a column boundary. This creates grid rows of **minimum ruling line spacing** (~13pt).

Multi-line text content (20-30pt) spans multiple grid rows. Text elements whose center falls in the wrong grid row get assigned to wrong cells. This limits extraction accuracy to 97.7% — the remaining 2.3% are text elements 1-6pt outside their visual cell boundary.

No ADR ever decided this approach. It was the simplest initial implementation.

## Decision

Replace sorted-coordinate grid with **intersection-based cell finding** (tabula-java approach).

### Current approach (sorted coordinates)
```
H-lines: Y = [100, 113, 126, 139]
V-lines: X = [0, 90, 335, 508, 556]

Grid = 3 rows × 4 cols (every H-line creates a boundary)
Row heights: 13pt each
```

### New approach (intersection points)
```
For each pair of V-lines (column boundaries):
  Find H-lines that INTERSECT both V-lines
  These H-lines define row boundaries FOR THIS COLUMN

Cell = rectangle between 4 intersection points
Different columns can have different row boundaries
```

### Why intersection-based

A horizontal ruling line at Y=113 may span X=[90, 508] (cols 1-2 only). In sorted-coordinate grid, Y=113 creates a row boundary for ALL columns. In intersection-based grid, Y=113 creates boundaries only for cols 1-2, not for cols 0 (TIME) or 3 (VENUE).

Result: TIME and VENUE columns have larger cells (matching their visual merged appearance), while COURSE TITLE and SECTIONS columns have per-row cells. Multi-line text stays within its visual cell.

### Reference

tabula-java `SpreadsheetExtractionAlgorithm.findCells()`:
```java
// For each pair of perpendicular intersections:
//   topLeft = intersection(hLine1, vLine1)
//   bottomRight = intersection(hLine2, vLine2)
//   if both corners exist → valid cell
```

## Consequences

- Multi-line text stays in one cell (no overflow between rows)
- Merged cell detection may become simpler (cells already match visual boundaries)
- Post-processing patterns (C, D, E, F in mergeTextContinuations) become unnecessary
- Grid construction is more complex but produces correct results
- 97.7% → target 100% sections accuracy

## Implementation

1. Add `FindIntersections(hLines, vLines)` → intersection point map
2. Add `FindCellsFromIntersections(intersections)` → cells with 4-corner bounds
3. Replace `BuildGrid()` internals, keep same external API
4. Remove or simplify `mergeTextContinuations` patterns
5. Update golden tests
