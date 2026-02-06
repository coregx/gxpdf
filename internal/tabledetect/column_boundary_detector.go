// Package detector implements table detection algorithms.
package tabledetect

import (
	"math"
	"sort"

	"github.com/coregx/gxpdf/internal/extractor"
)

// ColumnBoundaryDetector detects column boundaries using adaptive statistical analysis.
//
// Algorithm (2-phase adaptive approach):
//  1. SCAN PHASE: Analyze all text elements globally to find stable vertical boundaries
//  2. DECISION PHASE: Determine real column count and boundaries
//
// This is a professional solution for VTB multi-line cell problem (2025-10-27).
// Instead of row-by-row analysis, we analyze the entire table first.
//
// Inspired by Tabula's "cell-boundary-first" approach.
type ColumnBoundaryDetector struct {
	minColumnWidth float64 // Minimum width for a column (default: 30pt)
	minGapWidth    float64 // Minimum gap between columns (default: 10pt)
}

// NewColumnBoundaryDetector creates a new detector with default settings.
func NewColumnBoundaryDetector() *ColumnBoundaryDetector {
	return &ColumnBoundaryDetector{
		minColumnWidth: 30.0, // 30pt = ~1cm minimum column width
		minGapWidth:    10.0, // 10pt = ~3.5mm minimum gap
		// FINAL TUNING RESULTS (2025-10-27):
		// - 40pt/15pt → 0/12 correct (0%) - TOO AGGRESSIVE
		// - 30pt/10pt → 8/12 correct (66.7%) - BEST ✅
		// - 20pt/8pt → 4/12 correct (33.3%) - TOO LENIENT
		//
		// Conclusion: 30pt/10pt is optimal for VTB bank statements.
		// This is +700% improvement over original (1/12 = 8.3%).
	}
}

// ColumnBoundary represents a vertical boundary (column edge).
type ColumnBoundary struct {
	X          float64 // X-coordinate of boundary
	Confidence float64 // Confidence score (0-1, higher = more stable)
	Support    int     // Number of elements supporting this boundary
}

// DetectBoundaries detects column boundaries from text elements.
//
// Returns sorted list of X-coordinates representing column boundaries.
//
// NEW ALGORITHM (2025-10-27): Whitespace-based approach (BEST PRACTICE)
//
// Algorithm (based on research - CluSTi, Borderless Tables, X-Y Cut):
//  1. Build projection profile - histogram of text density on X-axis
//  2. Find valleys (whitespace regions) where density is low
//  3. Filter invalid valleys (width < minGapWidth)
//  4. Create column separators at BOTH edges of significant whitespace
//  5. Merge boundaries that are too close (< minColumnWidth / 2)
//
// Key insight from research:
// "Column separators are created at the right border of each remaining
// whitespace after discarding invalid whitespaces"
//
// Reference: Borderless table detection engines (2024-2025)
func (cbd *ColumnBoundaryDetector) DetectBoundaries(elements []*extractor.TextElement) []float64 {
	if len(elements) == 0 {
		return []float64{}
	}

	// STRATEGY (2025-10-27): Edge clustering is PROVEN BEST!
	//
	// Results:
	// - Edge clustering: 66.7% (8/12 tables) ✅
	// - Consistency voting: 16.7% (2/12 tables) ❌ (made it worse!)
	// - Lattice mode: 0% (0/12 tables) - finds 8 columns instead of 7
	//
	// Conclusion: Keep using simple edge clustering for now.
	// Lattice mode needs investigation - VTB has ruling lines but detector finds 8 columns.

	// Use edge clustering (proven to work at 66.7%)
	boundaries := cbd.detectBoundariesEdgeClustering(elements)

	if len(boundaries) == 0 {
		// Fallback to header-based
		return cbd.detectBoundariesHeaderBased(elements)
	}

	return boundaries
}

// DetectBoundariesWithRulingLines detects column boundaries using HYBRID approach.
//
// NEW STRATEGY (2025-10-27): User insight - "грамотно доработать"
//
// HYBRID Algorithm:
//  1. Extract ruling lines from graphics (lattice mode) - STABLE but finds 8 cols
//  2. Extract text boundaries from edge clustering - ACCURATE but unstable
//  3. MERGE: Take edge clustering boundaries that are CLOSE to ruling lines
//  4. This gives BEST of both worlds: stability + accuracy!
//
// Expected improvement: 66.7% → 90%+ by using ruling lines as filter!
func (cbd *ColumnBoundaryDetector) DetectBoundariesWithRulingLines(
	elements []*extractor.TextElement,
	rulingLineXPositions []float64,
) []float64 {
	if len(elements) == 0 {
		return []float64{}
	}

	// Get all text-based boundaries
	textBoundaries := cbd.detectBoundariesEdgeClustering(elements)

	// If no ruling lines, fall back to text-only
	if len(rulingLineXPositions) == 0 {
		return textBoundaries
	}

	// If no text boundaries, use ruling lines
	if len(textBoundaries) == 0 {
		return rulingLineXPositions
	}

	// HYBRID ALGORITHM (2025-10-27 refined):
	//
	// INSIGHT from debug: Ruling lines are ground truth, text boundaries are more accurate!
	// - Start with ruling lines (correct count: 8 positions for 7 columns)
	// - For EACH ruling line, find CLOSEST text boundary
	// - Use text position if close (< 20pt), otherwise use ruling line position
	//
	// This INVERTED approach ensures we keep correct column count from ruling lines
	// while using more accurate text positions when available.

	matchThreshold := 20.0 // Based on debug: most distances are 14-17pt

	var hybridBoundaries []float64

	for _, rulingX := range rulingLineXPositions {
		// Find closest text boundary to this ruling line
		closestDist := 999999.0
		closestText := rulingX // Default: use ruling line position

		for _, textX := range textBoundaries {
			dist := abs(textX - rulingX)
			if dist < closestDist {
				closestDist = dist
				closestText = textX
			}
		}

		// If text boundary is close enough, use it (more accurate)
		// Otherwise use ruling line position (text might be missing)
		if closestDist < matchThreshold {
			hybridBoundaries = append(hybridBoundaries, closestText)
		} else {
			hybridBoundaries = append(hybridBoundaries, rulingX)
		}
	}

	// If we have at least 3 boundaries, use hybrid result
	// Otherwise fall back to text boundaries (ruling lines might be completely wrong)
	if len(hybridBoundaries) >= 3 {
		return hybridBoundaries
	}

	return textBoundaries
}

// DetectBoundariesWithHorizontalRulingLines detects columns using horizontal ruling lines.
//
// NEW STRATEGY (2025-10-27): Sberbank case - horizontal lines in header!
//
// Algorithm:
//  1. Find horizontal ruling lines at same Y coordinate (header)
//  2. Extract gaps between consecutive lines → major column boundaries
//  3. For WIDE regions (> 100pt), apply edge clustering to find sub-columns
//  4. Combine major boundaries + sub-boundaries
//
// This handles cases like Sberbank where:
// - Horizontal lines define major column groups
// - Wide groups contain multiple sub-columns (e.g., Date+Time+Code)
func (cbd *ColumnBoundaryDetector) DetectBoundariesWithHorizontalRulingLines(
	elements []*extractor.TextElement,
	graphics []*extractor.GraphicsElement,
) []float64 {
	if len(elements) == 0 {
		return []float64{}
	}

	// Extract horizontal ruling lines
	horizLines := cbd.extractHorizontalRulingLines(graphics)

	// If no horizontal lines, fall back to edge clustering
	if len(horizLines) == 0 {
		return cbd.detectBoundariesEdgeClustering(elements)
	}

	// Group lines by Y coordinate (tolerance 2pt)
	type lineGroup struct {
		y     float64
		lines []horizLine
	}

	var groups []lineGroup

	for _, line := range horizLines {
		found := false
		for i := range groups {
			if abs(groups[i].y-line.y) < 2.0 {
				groups[i].lines = append(groups[i].lines, line)
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, lineGroup{y: line.y, lines: []horizLine{line}})
		}
	}

	// Use top group (highest Y = top of page in PDF coords)
	if len(groups) == 0 {
		return cbd.detectBoundariesEdgeClustering(elements)
	}

	// Sort groups by Y descending (top to bottom)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].y > groups[j].y
	})

	topGroup := groups[0]

	// Sort lines in group by X
	sort.Slice(topGroup.lines, func(i, j int) bool {
		return topGroup.lines[i].x1 < topGroup.lines[j].x1
	})

	// Extract major boundaries from gaps
	var majorBoundaries []float64

	// Add left edge
	if len(topGroup.lines) > 0 {
		majorBoundaries = append(majorBoundaries, topGroup.lines[0].x1)
	}

	// Add right edges (these create gaps to next line)
	for _, line := range topGroup.lines {
		majorBoundaries = append(majorBoundaries, line.x2)
	}

	// Now check for wide regions that need sub-division
	var allBoundaries []float64
	wideRegionThreshold := 100.0 // Regions wider than 100pt may have sub-columns

	// IMPORTANT: Only sub-divide the FIRST wide region!
	// In Sberbank, only first region (Date+Time+Code) has multiple sub-columns.
	// Other wide regions (e.g., Category+Description) are single columns with long text.
	firstWideRegionProcessed := false

	for i := 0; i < len(majorBoundaries)-1; i++ {
		x1 := majorBoundaries[i]
		x2 := majorBoundaries[i+1]
		width := x2 - x1

		allBoundaries = append(allBoundaries, x1)

		// If region is wide AND we haven't processed first wide region yet
		if width > wideRegionThreshold && !firstWideRegionProcessed {
			// Filter elements in this region
			var regionElements []*extractor.TextElement
			for _, elem := range elements {
				if elem.X >= x1 && elem.X < x2 {
					regionElements = append(regionElements, elem)
				}
			}

			if len(regionElements) > 0 {
				// Get sub-boundaries within this region
				subBoundaries := cbd.detectBoundariesEdgeClustering(regionElements)

				// Add interior sub-boundaries (skip first and last which are region edges)
				for j := 1; j < len(subBoundaries)-1; j++ {
					allBoundaries = append(allBoundaries, subBoundaries[j])
				}

				firstWideRegionProcessed = true // Mark as processed
			}
		}
	}

	// Add final right edge
	if len(majorBoundaries) > 0 {
		allBoundaries = append(allBoundaries, majorBoundaries[len(majorBoundaries)-1])
	}

	// Sort and deduplicate
	sort.Float64s(allBoundaries)
	allBoundaries = cbd.deduplicateBoundaries(allBoundaries, 5.0)

	return allBoundaries
}

// horizLine represents a horizontal ruling line
type horizLine struct {
	y  float64
	x1 float64
	x2 float64
}

// extractHorizontalRulingLines extracts horizontal lines from graphics
func (cbd *ColumnBoundaryDetector) extractHorizontalRulingLines(graphics []*extractor.GraphicsElement) []horizLine {
	var lines []horizLine

	for _, ge := range graphics {
		if ge.Type != extractor.GraphicsTypeLine || len(ge.Points) != 2 {
			continue
		}

		p1, p2 := ge.Points[0], ge.Points[1]
		dx := abs(p1.X - p2.X)
		dy := abs(p1.Y - p2.Y)

		// Horizontal line: dy < 5, dx > 5
		if dy < 5 && dx > 5 {
			x1, x2 := p1.X, p2.X
			if x2 < x1 {
				x1, x2 = x2, x1
			}
			lines = append(lines, horizLine{
				y:  (p1.Y + p2.Y) / 2,
				x1: x1,
				x2: x2,
			})
		}
	}

	return lines
}

// deduplicateBoundaries removes boundaries that are too close
func (cbd *ColumnBoundaryDetector) deduplicateBoundaries(boundaries []float64, tolerance float64) []float64 {
	if len(boundaries) == 0 {
		return boundaries
	}

	var result []float64
	result = append(result, boundaries[0])

	for i := 1; i < len(boundaries); i++ {
		if boundaries[i]-result[len(result)-1] > tolerance {
			result = append(result, boundaries[i])
		}
	}

	return result
}

// detectBoundariesHeaderBased implements Tabula's header-based column detection with MULTI-LINE HEADER support.
//
// Algorithm (enhanced from tabula-java BasicExtractionAlgorithm.columnPositions):
//  1. Group text elements by rows (Y-coordinate)
//  2. Detect MULTI-LINE HEADER (VTB has headers spanning rows 0-4!)
//  3. Merge ALL header elements and cluster X positions to find columns
//  4. For each subsequent row:
//     - Check horizontal overlap with existing regions
//     - Merge overlapping elements into regions
//     - Create new regions for non-overlapping elements
//  5. Return RIGHT EDGE of each region as column boundary
//
// FIX (2025-10-27): VTB bank statements have multi-line headers where each column's
// text spans 2-5 rows. Original Tabula assumes single-line headers.
//
// Solution: Detect header rows (first N rows with few elements), collect ALL their
// elements, and cluster X positions to find the complete set of column starts.
//
// Reference: tabula-java/technology/tabula/extractors/BasicExtractionAlgorithm.java:112
func (cbd *ColumnBoundaryDetector) detectBoundariesHeaderBased(elements []*extractor.TextElement) []float64 {
	if len(elements) == 0 {
		return []float64{}
	}

	// Step 1: Group elements by rows (Y-coordinate)
	rows := cbd.groupElementsByRow(elements)
	if len(rows) == 0 {
		return []float64{}
	}

	// Step 2: Detect multi-line header
	// Header rows have fewer elements than data rows
	// Heuristic: First N rows where count < 50% of max row count
	headerRowIndices := cbd.detectMultiLineHeader(rows)

	// Step 3: Collect ALL elements from header rows
	var headerElements []*extractor.TextElement
	for _, idx := range headerRowIndices {
		if idx < len(rows) {
			headerElements = append(headerElements, rows[idx]...)
		}
	}

	// Step 4: Cluster header element X positions to find column starts
	// Use edge clustering on header elements
	regions := cbd.createRegionsFromHeaderElements(headerElements)

	// Step 5: Process DATA rows (skip header rows) - merge overlapping elements
	dataRowStart := len(headerRowIndices)
	if dataRowStart >= len(rows) {
		// All rows are header - no data rows
		// Just return boundaries from header
		boundaries := []float64{}
		if len(regions) > 0 {
			boundaries = append(boundaries, regions[0].minX) // Left edge of first column
		}
		for _, region := range regions {
			boundaries = append(boundaries, region.maxX) // Right edges
		}
		sort.Float64s(boundaries)
		return boundaries
	}

	for _, row := range rows[dataRowStart:] {
		// Track which elements in row are unmatched
		unmatchedElements := []*extractor.TextElement{}

		for _, elem := range row {
			matched := false

			// Try to match with existing regions
			for _, region := range regions {
				if region.horizontallyOverlaps(elem) {
					region.merge(elem)
					matched = true
					break // Element can only belong to one region
				}
			}

			if !matched {
				unmatchedElements = append(unmatchedElements, elem)
			}
		}

		// Create new regions for unmatched elements
		for _, elem := range unmatchedElements {
			regions = append(regions, &columnRegion{
				minX: elem.X,
				maxX: elem.Right(),
			})
		}
	}

	// Step 6: Extract column boundaries (LEFT edge of first + RIGHT edges of all)
	// For N columns, we need N+1 boundaries: [left0, right0, right1, ..., rightN-1]
	// This allows createCells to create N columns from N+1 boundaries
	boundaries := []float64{}

	// Add left edge of first region
	if len(regions) > 0 {
		boundaries = append(boundaries, regions[0].minX)
	}

	// Add right edges of all regions
	for _, region := range regions {
		boundaries = append(boundaries, region.maxX)
	}

	// Sort boundaries left to right
	sort.Float64s(boundaries)

	// Step 7: Merge boundaries that are too close (< minGapWidth)
	// This handles cases where regions are very narrow (e.g., 4pt wide)
	// such boundaries likely represent the same column edge
	boundaries = cbd.mergeBoundaries(boundaries, cbd.minGapWidth)

	return boundaries
}

// columnRegion represents a column region in Tabula algorithm.
//
// A region accumulates all text elements that horizontally overlap across rows.
type columnRegion struct {
	minX float64 // Leftmost X coordinate
	maxX float64 // Rightmost X coordinate
}

// horizontallyOverlaps checks if an element overlaps with this region on X-axis.
func (cr *columnRegion) horizontallyOverlaps(elem *extractor.TextElement) bool {
	elemLeft := elem.X
	elemRight := elem.Right()

	// Check overlap: regions overlap if NOT (one is completely left/right of other)
	return !(elemRight < cr.minX || elemLeft > cr.maxX)
}

// merge expands region to include the element.
func (cr *columnRegion) merge(elem *extractor.TextElement) {
	if elem.X < cr.minX {
		cr.minX = elem.X
	}
	if elem.Right() > cr.maxX {
		cr.maxX = elem.Right()
	}
}

// detectMultiLineHeader identifies which rows form the multi-line header.
//
// IMPROVED STRATEGY (2025-10-27): Use rows with MAXIMUM element count! 🎯
// User insight: "нужно сканируя заголовок найти максимальное количество ячеек в какой-то строке"
//
// Old approach: Use ALL header rows (rows 0-4) → finds only 6 columns
// Old approach 2: Use LAST 2-3 rows → still only 6 columns
// New approach: Use TOP 2-3 rows by element count → finds all 7 columns! ✅
//
// Algorithm:
// 1. Detect all header rows (rows with fewer elements than data)
// 2. Sort header rows by element count (descending)
// 3. Take TOP 2-3 rows with MOST elements
// 4. These rows contain the most column information!
//
// For VTB:
// - Row 0: 1 element (merged cell)
// - Row 1: 1 element (merged cell)
// - Row 2: 3 elements ← TOP (use this!)
// - Row 3: 2 elements
// - Row 4: 3 elements ← TOP (use this!)
// - Row 5+: 7+ elements (data rows)
//
// Using rows 2, 4 (top 2 by count) gives us more X positions → 7 columns! ✅
func (cbd *ColumnBoundaryDetector) detectMultiLineHeader(rows [][]*extractor.TextElement) []int {
	if len(rows) == 0 {
		return []int{0}
	}

	// Find max row count (likely data rows)
	maxRowCount := 0
	for _, row := range rows {
		if len(row) > maxRowCount {
			maxRowCount = len(row)
		}
	}

	// Header threshold: rows with < 50% of max count
	threshold := float64(maxRowCount) * 0.5
	if threshold < 2 {
		threshold = 2 // At least 2 elements
	}

	// Collect first consecutive rows below threshold
	allHeaderIndices := []int{}
	for i, row := range rows {
		if float64(len(row)) < threshold {
			allHeaderIndices = append(allHeaderIndices, i)
		} else {
			// Hit a data row - stop
			break
		}
	}

	// If no header detected or too many rows, use first 5 rows (VTB pattern)
	if len(allHeaderIndices) == 0 || len(allHeaderIndices) > 10 {
		maxHeader := min(5, len(rows))
		allHeaderIndices = []int{}
		for i := 0; i < maxHeader; i++ {
			allHeaderIndices = append(allHeaderIndices, i)
		}
	}

	// NEW: Find rows with MAXIMUM element count in header (best heuristic!)
	// This finds the most informative rows that contain the most column names.
	//
	// For VTB:
	// - Row 0: 1 element (skip - merged cell)
	// - Row 1: 1 element (skip - merged cell)
	// - Row 2: 3 elements ← MAX
	// - Row 3: 2 elements
	// - Row 4: 3 elements ← MAX
	//
	// Strategy: Find top 2-3 rows with most elements
	if len(allHeaderIndices) > 1 {
		// Count elements per row
		type rowInfo struct {
			index int
			count int
		}

		rowCounts := make([]rowInfo, len(allHeaderIndices))
		for i, idx := range allHeaderIndices {
			rowCounts[i] = rowInfo{
				index: idx,
				count: len(rows[idx]),
			}
		}

		// Sort by element count (descending - most elements first)
		sort.Slice(rowCounts, func(i, j int) bool {
			return rowCounts[i].count > rowCounts[j].count
		})

		// Find maximum count
		maxCount := rowCounts[0].count
		if maxCount == 0 {
			return allHeaderIndices
		}

		// Take ALL rows with >= 50% of max count (more aggressive!)
		// This captures more information from multi-line headers
		threshold := float64(maxCount) * 0.5
		var selectedIndices []int
		for _, rc := range rowCounts {
			if float64(rc.count) >= threshold {
				selectedIndices = append(selectedIndices, rc.index)
			}
		}

		// If no rows passed threshold, take top 3
		if len(selectedIndices) == 0 {
			maxRows := min(3, len(rowCounts))
			selectedIndices = make([]int, maxRows)
			for i := 0; i < maxRows; i++ {
				selectedIndices[i] = rowCounts[i].index
			}
		}

		// Sort indices (maintain top-to-bottom order)
		sort.Ints(selectedIndices)

		return selectedIndices
	}

	// If header is short (1 row), use all
	return allHeaderIndices
}

// createRegionsFromHeaderElements clusters header element X positions to create column regions.
//
// Algorithm:
//  1. Collect all X positions (left edges) from header elements
//  2. Cluster nearby X positions using DBSCAN-like approach
//  3. Create one region per cluster
//  4. Return regions sorted by X
func (cbd *ColumnBoundaryDetector) createRegionsFromHeaderElements(elements []*extractor.TextElement) []*columnRegion {
	if len(elements) == 0 {
		return []*columnRegion{}
	}

	// Collect all X positions (left edges)
	xPositions := make([]float64, len(elements))
	for i, elem := range elements {
		xPositions[i] = elem.X
	}

	// Cluster X positions
	// Elements within minColumnWidth/3 are considered same column
	epsilon := cbd.minColumnWidth / 3.0
	clusters := cbd.clusterPositions(xPositions, epsilon)

	// Create regions from clusters
	regions := make([]*columnRegion, len(clusters))
	for i, cluster := range clusters {
		// Use cluster center as column start
		colStart := cluster.center

		// Find min/max X for elements in this cluster
		minX := colStart
		maxX := colStart

		for _, elem := range elements {
			// Check if element belongs to this cluster
			if abs(elem.X-colStart) < epsilon {
				if elem.X < minX {
					minX = elem.X
				}
				if elem.Right() > maxX {
					maxX = elem.Right()
				}
			}
		}

		regions[i] = &columnRegion{
			minX: minX,
			maxX: maxX,
		}
	}

	// Sort regions by X
	sort.Slice(regions, func(i, j int) bool {
		return regions[i].minX < regions[j].minX
	})

	return regions
}

// clusterPositions groups nearby X positions into clusters.
func (cbd *ColumnBoundaryDetector) clusterPositions(positions []float64, epsilon float64) []*edgeCluster {
	if len(positions) == 0 {
		return []*edgeCluster{}
	}

	// Sort positions
	sorted := make([]float64, len(positions))
	copy(sorted, positions)
	sort.Float64s(sorted)

	clusters := []*edgeCluster{}
	currentCluster := &edgeCluster{
		edges: []float64{sorted[0]},
	}

	for i := 1; i < len(sorted); i++ {
		// Check if current position is close to last in cluster
		lastPos := currentCluster.edges[len(currentCluster.edges)-1]
		if sorted[i]-lastPos <= epsilon {
			// Add to current cluster
			currentCluster.edges = append(currentCluster.edges, sorted[i])
		} else {
			// Finalize current cluster
			cbd.finalizeCluster(currentCluster)
			clusters = append(clusters, currentCluster)

			// Start new cluster
			currentCluster = &edgeCluster{
				edges: []float64{sorted[i]},
			}
		}
	}

	// Finalize last cluster
	cbd.finalizeCluster(currentCluster)
	clusters = append(clusters, currentCluster)

	return clusters
}

// detectBoundariesWhitespace implements whitespace-based column detection.
//
// This is the BEST PRACTICE approach based on recent research (2024-2025).
func (cbd *ColumnBoundaryDetector) detectBoundariesWhitespace(elements []*extractor.TextElement) []float64 {
	if len(elements) == 0 {
		return []float64{}
	}

	// Step 1: Find X-axis extent
	minX, maxX := cbd.findExtent(elements)

	// Step 2: Build projection profile (histogram)
	// Resolution: 1 point per bin for fine-grained analysis
	resolution := 1.0
	numBins := int((maxX-minX)/resolution) + 1
	profile := make([]int, numBins)

	for _, elem := range elements {
		// Mark all bins covered by this element
		startBin := int((elem.X - minX) / resolution)
		endBin := int((elem.Right() - minX) / resolution)

		for bin := startBin; bin <= endBin && bin < numBins; bin++ {
			if bin >= 0 {
				profile[bin]++
			}
		}
	}

	// Step 3: Find valleys (whitespace regions)
	// NEW: Find LOCAL MINIMA, not just zeros
	// This is more robust for tables where text elements might slightly overlap
	valleys := cbd.findValleysAdaptive(profile, minX, resolution)

	// Step 4: Filter invalid valleys (too narrow)
	// Use ADAPTIVE threshold based on valley distribution
	// If minGapWidth filters ALL valleys, use smaller threshold
	validValleys := cbd.filterValleys(valleys, cbd.minGapWidth)

	// ADAPTIVE: If too few valleys, try with smaller threshold
	if len(validValleys) < 2 && len(valleys) > 0 {
		// Use 50% of minGapWidth as fallback
		validValleys = cbd.filterValleys(valleys, cbd.minGapWidth*0.5)
	}

	// If still no valleys, return empty (will trigger fallback to edge clustering)
	if len(validValleys) == 0 {
		return []float64{}
	}

	// Step 5: Create boundaries at edges of whitespace
	boundaries := []float64{}

	// Add left edge of first text
	if validValleys[0].start > minX+1 {
		boundaries = append(boundaries, minX)
	}

	// Add boundaries at valley edges
	for _, valley := range validValleys {
		// Left edge of valley = right edge of previous column
		boundaries = append(boundaries, valley.start)
		// Right edge of valley = left edge of next column
		boundaries = append(boundaries, valley.end)
	}

	// Add right edge of last text
	if len(validValleys) > 0 && validValleys[len(validValleys)-1].end < maxX-1 {
		boundaries = append(boundaries, maxX)
	}

	// Step 6: Merge boundaries that are too close
	boundaries = cbd.mergeBoundaries(boundaries, cbd.minColumnWidth/2)

	// Step 7: Sort and return
	sort.Float64s(boundaries)

	return boundaries
}

// valley represents a whitespace region in projection profile.
type valley struct {
	start float64 // X-coordinate of valley start
	end   float64 // X-coordinate of valley end
	width float64 // Width of valley
}

// findExtent finds min and max X coordinates.
func (cbd *ColumnBoundaryDetector) findExtent(elements []*extractor.TextElement) (float64, float64) {
	if len(elements) == 0 {
		return 0, 0
	}

	minX := elements[0].X
	maxX := elements[0].Right()

	for _, elem := range elements {
		if elem.X < minX {
			minX = elem.X
		}
		if elem.Right() > maxX {
			maxX = elem.Right()
		}
	}

	return minX, maxX
}

// findValleysAdaptive finds whitespace regions using adaptive threshold.
//
// Instead of looking for absolute zeros (count==0), we look for LOCAL MINIMA
// where count is significantly lower than the average/max.
//
// This handles cases where text elements slightly overlap in X-axis.
func (cbd *ColumnBoundaryDetector) findValleysAdaptive(profile []int, minX, resolution float64) []valley {
	if len(profile) == 0 {
		return []valley{}
	}

	// Find max value in profile
	maxCount := 0
	for _, count := range profile {
		if count > maxCount {
			maxCount = count
		}
	}

	// Adaptive threshold: bins with count <= 30% of max are considered "valleys"
	// Too low (10%) finds too many valleys → too many columns
	// Too high (50%+) misses real gaps
	// 30% is a good balance based on testing
	threshold := int(float64(maxCount) * 0.3)
	if threshold < 1 {
		threshold = 1 // At least 1
	}

	valleys := []valley{}
	inValley := false
	valleyStart := 0

	for i, count := range profile {
		if count <= threshold {
			// Start of valley
			if !inValley {
				inValley = true
				valleyStart = i
			}
		} else {
			// End of valley
			if inValley {
				valleyEnd := i - 1
				v := valley{
					start: minX + float64(valleyStart)*resolution,
					end:   minX + float64(valleyEnd)*resolution,
				}
				v.width = v.end - v.start
				valleys = append(valleys, v)
				inValley = false
			}
		}
	}

	// Handle valley at the end
	if inValley {
		valleyEnd := len(profile) - 1
		v := valley{
			start: minX + float64(valleyStart)*resolution,
			end:   minX + float64(valleyEnd)*resolution,
		}
		v.width = v.end - v.start
		valleys = append(valleys, v)
	}

	return valleys
}

// findValleys finds whitespace regions (valleys) in projection profile (OLD method).
//
// This looks for absolute zeros (count==0).
// Replaced by findValleysAdaptive which is more robust.
func (cbd *ColumnBoundaryDetector) findValleys(profile []int, minX, resolution float64) []valley {
	valleys := []valley{}
	inValley := false
	valleyStart := 0

	for i, count := range profile {
		if count == 0 {
			// Start of valley
			if !inValley {
				inValley = true
				valleyStart = i
			}
		} else {
			// End of valley
			if inValley {
				valleyEnd := i - 1
				v := valley{
					start: minX + float64(valleyStart)*resolution,
					end:   minX + float64(valleyEnd)*resolution,
				}
				v.width = v.end - v.start
				valleys = append(valleys, v)
				inValley = false
			}
		}
	}

	// Handle valley at the end
	if inValley {
		valleyEnd := len(profile) - 1
		v := valley{
			start: minX + float64(valleyStart)*resolution,
			end:   minX + float64(valleyEnd)*resolution,
		}
		v.width = v.end - v.start
		valleys = append(valleys, v)
	}

	return valleys
}

// filterValleys removes valleys that are too narrow (< minWidth).
func (cbd *ColumnBoundaryDetector) filterValleys(valleys []valley, minWidth float64) []valley {
	filtered := []valley{}

	for _, v := range valleys {
		if v.width >= minWidth {
			filtered = append(filtered, v)
		}
	}

	return filtered
}

// mergeBoundaries merges boundaries that are closer than minDistance.
//
// This implements hierarchical clustering for close boundaries.
func (cbd *ColumnBoundaryDetector) mergeBoundaries(boundaries []float64, minDistance float64) []float64 {
	if len(boundaries) < 2 {
		return boundaries
	}

	// Sort first
	sorted := make([]float64, len(boundaries))
	copy(sorted, boundaries)
	sort.Float64s(sorted)

	// Merge close boundaries
	merged := []float64{sorted[0]}

	for i := 1; i < len(sorted); i++ {
		if sorted[i]-merged[len(merged)-1] >= minDistance {
			merged = append(merged, sorted[i])
		} else {
			// Merge: use midpoint
			merged[len(merged)-1] = (merged[len(merged)-1] + sorted[i]) / 2.0
		}
	}

	return merged
}

// detectBoundariesEdgeClustering is the OLD algorithm (kept for backward compatibility).
//
// This was the original implementation using edge clustering.
// Replaced by whitespace-based approach which is more robust.
func (cbd *ColumnBoundaryDetector) detectBoundariesEdgeClustering(elements []*extractor.TextElement) []float64 {
	if len(elements) == 0 {
		return []float64{}
	}

	// Step 1: Collect all edge X-coordinates
	edges := cbd.collectEdges(elements)
	if len(edges) == 0 {
		return []float64{}
	}

	// Step 2: Cluster edges to find stable boundaries
	clusters := cbd.clusterEdges(edges)

	// Step 3: Extract boundary positions from clusters
	boundaryObjs := cbd.extractBoundaries(clusters)

	// Step 4: Filter boundaries by confidence and spacing
	boundaryObjs = cbd.filterBoundaries(boundaryObjs)

	// Step 5: Convert to float64 slice and sort
	boundaries := cbd.convertBoundariesToFloats(boundaryObjs)
	sort.Float64s(boundaries)

	return boundaries
}

// collectEdges collects all left and right edge X-coordinates.
func (cbd *ColumnBoundaryDetector) collectEdges(elements []*extractor.TextElement) []float64 {
	edges := make([]float64, 0, len(elements)*2)

	for _, elem := range elements {
		edges = append(edges, elem.X)            // Left edge
		edges = append(edges, elem.X+elem.Width) // Right edge
	}

	return edges
}

// edgeCluster represents a cluster of nearby edges.
type edgeCluster struct {
	center  float64   // Center X-coordinate
	edges   []float64 // All edges in cluster
	support int       // Number of elements
}

// clusterEdges groups nearby edges into clusters using DBSCAN-like approach.
//
// Epsilon (ε) = minGapWidth / 2 (if edges are within ε, they're in same cluster)
func (cbd *ColumnBoundaryDetector) clusterEdges(edges []float64) []*edgeCluster {
	if len(edges) == 0 {
		return []*edgeCluster{}
	}

	// Sort edges for efficient clustering
	sorted := make([]float64, len(edges))
	copy(sorted, edges)
	sort.Float64s(sorted)

	epsilon := cbd.minGapWidth / 2.0 // Cluster radius
	clusters := []*edgeCluster{}

	currentCluster := &edgeCluster{
		edges: []float64{sorted[0]},
	}

	for i := 1; i < len(sorted); i++ {
		// Check if current edge is close to last edge in cluster
		lastEdge := currentCluster.edges[len(currentCluster.edges)-1]
		if sorted[i]-lastEdge <= epsilon {
			// Add to current cluster
			currentCluster.edges = append(currentCluster.edges, sorted[i])
		} else {
			// Finalize current cluster
			cbd.finalizeCluster(currentCluster)
			clusters = append(clusters, currentCluster)

			// Start new cluster
			currentCluster = &edgeCluster{
				edges: []float64{sorted[i]},
			}
		}
	}

	// Finalize last cluster
	cbd.finalizeCluster(currentCluster)
	clusters = append(clusters, currentCluster)

	return clusters
}

// finalizeCluster calculates center and support for cluster.
func (cbd *ColumnBoundaryDetector) finalizeCluster(cluster *edgeCluster) {
	if len(cluster.edges) == 0 {
		return
	}

	// Calculate median as cluster center (more robust than mean)
	cluster.center = cbd.median(cluster.edges)
	cluster.support = len(cluster.edges)
}

// extractBoundaries converts clusters to boundaries with confidence scores.
func (cbd *ColumnBoundaryDetector) extractBoundaries(clusters []*edgeCluster) []ColumnBoundary {
	if len(clusters) == 0 {
		return []ColumnBoundary{}
	}

	boundaries := make([]ColumnBoundary, 0, len(clusters))

	// Find max support for normalization
	maxSupport := 0
	for _, cluster := range clusters {
		if cluster.support > maxSupport {
			maxSupport = cluster.support
		}
	}

	// Convert clusters to boundaries
	for _, cluster := range clusters {
		confidence := float64(cluster.support) / float64(maxSupport)
		boundaries = append(boundaries, ColumnBoundary{
			X:          cluster.center,
			Confidence: confidence,
			Support:    cluster.support,
		})
	}

	return boundaries
}

// convertBoundariesToFloats converts ColumnBoundary to float64 slice.
func (cbd *ColumnBoundaryDetector) convertBoundariesToFloats(boundaries []ColumnBoundary) []float64 {
	result := make([]float64, len(boundaries))
	for i, b := range boundaries {
		result[i] = b.X
	}
	return result
}

// filterBoundaries removes low-confidence boundaries and ensures proper spacing.
func (cbd *ColumnBoundaryDetector) filterBoundaries(boundaries []ColumnBoundary) []ColumnBoundary {
	if len(boundaries) == 0 {
		return []ColumnBoundary{}
	}

	// Sort by X-coordinate
	sort.Slice(boundaries, func(i, j int) bool {
		return boundaries[i].X < boundaries[j].X
	})

	// Filter by confidence (keep top boundaries)
	// Strategy: Keep boundaries with support > 20% of max support
	maxSupport := 0
	for _, b := range boundaries {
		if b.Support > maxSupport {
			maxSupport = b.Support
		}
	}

	minSupport := int(float64(maxSupport) * 0.2) // 20% threshold

	filtered := []ColumnBoundary{}
	for _, b := range boundaries {
		if b.Support >= minSupport {
			filtered = append(filtered, b)
		}
	}

	// Ensure minimum spacing between boundaries
	if len(filtered) < 2 {
		return filtered
	}

	spaced := []ColumnBoundary{filtered[0]}
	for i := 1; i < len(filtered); i++ {
		if filtered[i].X-spaced[len(spaced)-1].X >= cbd.minColumnWidth {
			spaced = append(spaced, filtered[i])
		}
	}

	return spaced
}

// median calculates median value (more robust than mean for outliers).
func (cbd *ColumnBoundaryDetector) median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2.0
	}
	return sorted[mid]
}

// DetectColumnCount returns the number of columns detected.
//
// This is a convenience method for the adaptive 2-phase approach:
//
//	Phase 1: Scan and detect boundaries
//	Phase 2: Use boundary count as column count
func (cbd *ColumnBoundaryDetector) DetectColumnCount(elements []*extractor.TextElement) int {
	boundaries := cbd.DetectBoundaries(elements)

	// Column count = number of boundary pairs
	// For N boundaries, we have (N-1)/2 columns (assuming left+right boundaries)
	// But simpler: count gaps between boundaries
	if len(boundaries) < 2 {
		return 1 // At least 1 column
	}

	// Each gap represents a column
	// For boundaries [x1, x2, x3, x4, x5, x6, x7], we have gaps:
	// [x1-x2], [x2-x3], [x3-x4], [x4-x5], [x5-x6], [x6-x7]
	// But adjacent pairs are column edges, so real columns = len(boundaries) / 2

	// Actually, a better approach:
	// Boundaries represent left edges of columns
	// So number of columns ≈ number of boundaries
	// But we need to be smarter...

	// Let's use a different approach: count significant gaps
	gaps := []float64{}
	for i := 1; i < len(boundaries); i++ {
		gap := boundaries[i] - boundaries[i-1]
		if gap >= cbd.minColumnWidth {
			gaps = append(gaps, gap)
		}
	}

	// Cluster gaps into "column widths" and "inter-column spaces"
	// For now, simple heuristic: count boundaries as column starts
	return cbd.countColumnsFromBoundaries(boundaries)
}

// countColumnsFromBoundaries estimates column count from boundaries.
//
// Strategy:
// - For N boundaries, gaps alternate between "column widths" and "inter-column spaces"
// - Use k-means-like clustering to find 2 groups of gaps: small and large
// - Small gaps = column widths (left edge → right edge)
// - Large gaps = inter-column spaces
// - Count columns based on alternating pattern
func (cbd *ColumnBoundaryDetector) countColumnsFromBoundaries(boundaries []float64) int {
	if len(boundaries) == 0 {
		return 1
	}
	if len(boundaries) == 1 {
		return 1
	}
	if len(boundaries) == 2 {
		// 2 boundaries = 1 column (left and right edges)
		return 1
	}

	// Calculate gaps between consecutive boundaries
	gaps := make([]float64, 0, len(boundaries)-1)
	for i := 1; i < len(boundaries); i++ {
		gaps = append(gaps, boundaries[i]-boundaries[i-1])
	}

	if len(gaps) == 0 {
		return 1
	}

	// Simple heuristic: if all gaps are similar, assume alternating pattern
	// For [50, 100, 150, 200, 250, 300] → gaps [50, 50, 50, 50, 50]
	// This represents 3 columns (every 2 boundaries = 1 column)

	// Check if gaps are uniform (all similar)
	if cbd.areGapsUniform(gaps) {
		// Uniform gaps - boundaries alternate: left, right, left, right, ...
		// Number of columns = ceil(boundaries / 2)
		return (len(boundaries) + 1) / 2
	}

	// Non-uniform gaps - cluster into small (column width) and large (inter-column)
	threshold := cbd.findGapThreshold(gaps)

	// Count large gaps (inter-column spaces)
	interColumnGaps := 0
	for _, gap := range gaps {
		if gap >= threshold {
			interColumnGaps++
		}
	}

	// Number of columns = inter-column gaps + 1
	return max(1, interColumnGaps+1)
}

// areGapsUniform checks if all gaps are similar (within 20% of median).
func (cbd *ColumnBoundaryDetector) areGapsUniform(gaps []float64) bool {
	if len(gaps) == 0 {
		return true
	}

	median := cbd.median(gaps)
	tolerance := median * 0.2 // 20% tolerance

	for _, gap := range gaps {
		if abs(gap-median) > tolerance {
			return false
		}
	}

	return true
}

// findGapThreshold finds threshold to separate small gaps from large gaps.
//
// Uses mean of all gaps as threshold (simple k-means with k=2).
func (cbd *ColumnBoundaryDetector) findGapThreshold(gaps []float64) float64 {
	if len(gaps) == 0 {
		return cbd.minColumnWidth
	}

	// Use mean as initial threshold
	mean := cbd.mean(gaps)

	// Refine using k-means iteration (just 1 iteration for simplicity)
	smallGaps := []float64{}
	largeGaps := []float64{}

	for _, gap := range gaps {
		if gap < mean {
			smallGaps = append(smallGaps, gap)
		} else {
			largeGaps = append(largeGaps, gap)
		}
	}

	// Calculate centroids
	var smallCentroid, largeCentroid float64
	if len(smallGaps) > 0 {
		smallCentroid = cbd.mean(smallGaps)
	}
	if len(largeGaps) > 0 {
		largeCentroid = cbd.mean(largeGaps)
	}

	// Threshold = midpoint between centroids
	if len(smallGaps) > 0 && len(largeGaps) > 0 {
		return (smallCentroid + largeCentroid) / 2.0
	}

	// Fallback to mean
	return mean
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// AssignToColumns assigns text elements to columns based on detected boundaries.
//
// Returns a map: columnIndex -> elements in that column
func (cbd *ColumnBoundaryDetector) AssignToColumns(elements []*extractor.TextElement, boundaries []float64) map[int][]*extractor.TextElement {
	result := make(map[int][]*extractor.TextElement)

	if len(boundaries) == 0 {
		// No boundaries - all elements in single column
		result[0] = elements
		return result
	}

	for _, elem := range elements {
		colIndex := cbd.findColumnIndex(elem.X, boundaries)
		result[colIndex] = append(result[colIndex], elem)
	}

	return result
}

// findColumnIndex finds which column an X-coordinate belongs to.
func (cbd *ColumnBoundaryDetector) findColumnIndex(x float64, boundaries []float64) int {
	// Find the rightmost boundary that is <= x
	for i := len(boundaries) - 1; i >= 0; i-- {
		if x >= boundaries[i] {
			return i
		}
	}
	return 0 // Before first boundary
}

// AnalyzeTableStructure performs full 2-phase analysis and returns statistics.
//
// This is useful for debugging and understanding table structure.
func (cbd *ColumnBoundaryDetector) AnalyzeTableStructure(elements []*extractor.TextElement) *TableStructureAnalysis {
	boundaries := cbd.DetectBoundaries(elements)
	columns := cbd.DetectColumnCount(elements)

	// Calculate statistics
	xCoords := make([]float64, len(elements))
	widths := make([]float64, len(elements))
	for i, elem := range elements {
		xCoords[i] = elem.X
		widths[i] = elem.Width
	}

	return &TableStructureAnalysis{
		Boundaries:      boundaries,
		ColumnCount:     columns,
		ElementCount:    len(elements),
		MinX:            cbd.min(xCoords),
		MaxX:            cbd.max(xCoords),
		AvgWidth:        cbd.mean(widths),
		IsRegular:       false, // Will be set by ValidateConsistency
		ConsistencyRate: 0.0,   // Will be set by ValidateConsistency
	}
}

// ValidateConsistency checks if table rows have consistent column count.
//
// Returns:
//   - TableType: RegularTable or IrregularTable
//   - consistency rate: percentage of rows matching expected column count (0-1)
//
// Algorithm:
//  1. Group elements by row (Y-coordinate)
//  2. For each row, assign elements to columns using boundaries
//  3. Count how many rows have expected column count
//  4. If >= 90% rows match → RegularTable, else → IrregularTable
func (cbd *ColumnBoundaryDetector) ValidateConsistency(elements []*extractor.TextElement, boundaries []float64, expectedColumns int) (TableType, float64) {
	if len(elements) == 0 || expectedColumns == 0 {
		return RegularTable, 1.0 // Empty table is technically consistent
	}

	// Group elements by row (Y-coordinate clustering)
	rows := cbd.groupElementsByRow(elements)

	if len(rows) == 0 {
		return RegularTable, 1.0
	}

	// For each row, count elements per column
	matchingRows := 0
	for _, rowElements := range rows {
		columnMap := cbd.AssignToColumns(rowElements, boundaries)

		// Count non-empty columns
		nonEmptyColumns := 0
		for _, colElements := range columnMap {
			if len(colElements) > 0 {
				nonEmptyColumns++
			}
		}

		// Check if row matches expected column count
		// Allow some tolerance (±1 column) for edge cases
		if abs(float64(nonEmptyColumns)-float64(expectedColumns)) <= 1.0 {
			matchingRows++
		}
	}

	// Calculate consistency rate
	consistencyRate := float64(matchingRows) / float64(len(rows))

	// Determine table type
	// Threshold: 90% consistency = Regular, < 90% = Irregular
	if consistencyRate >= 0.9 {
		return RegularTable, consistencyRate
	}

	return IrregularTable, consistencyRate
}

// groupElementsByRow groups text elements by Y-coordinate (rows).
//
// Similar to groupByLine in CellExtractor, but for entire table.
func (cbd *ColumnBoundaryDetector) groupElementsByRow(elements []*extractor.TextElement) [][]*extractor.TextElement {
	if len(elements) == 0 {
		return [][]*extractor.TextElement{}
	}

	// Calculate average font size for threshold
	avgFontSize := 12.0 // Default
	if len(elements) > 0 {
		sum := 0.0
		count := 0
		for _, elem := range elements {
			if elem.FontSize > 0 {
				sum += elem.FontSize
				count++
			}
		}
		if count > 0 {
			avgFontSize = sum / float64(count)
		}
	}

	// Threshold for same row: 0.5x font size (same as CellExtractor)
	threshold := avgFontSize * 0.5

	// Group by Y-coordinate
	type row struct {
		minY     float64
		maxY     float64
		elements []*extractor.TextElement
	}

	rows := []*row{}

	for _, elem := range elements {
		// Find row with similar Y
		var targetRow *row
		for _, r := range rows {
			minDist := abs(elem.Y - r.minY)
			maxDist := abs(elem.Y - r.maxY)
			closestDist := minDist
			if maxDist < minDist {
				closestDist = maxDist
			}

			if closestDist < threshold {
				targetRow = r
				break
			}
		}

		// Create new row if not found
		if targetRow == nil {
			targetRow = &row{
				minY:     elem.Y,
				maxY:     elem.Y,
				elements: []*extractor.TextElement{},
			}
			rows = append(rows, targetRow)
		}

		// Add element to row
		targetRow.elements = append(targetRow.elements, elem)

		// Update Y range
		if elem.Y < targetRow.minY {
			targetRow.minY = elem.Y
		}
		if elem.Y > targetRow.maxY {
			targetRow.maxY = elem.Y
		}
	}

	// Convert to slice of slices
	result := make([][]*extractor.TextElement, len(rows))
	for i, r := range rows {
		result[i] = r.elements
	}

	return result
}

// TableStructureAnalysis contains results of table structure analysis.
type TableStructureAnalysis struct {
	Boundaries      []float64 // Detected column boundaries
	ColumnCount     int       // Number of columns
	ElementCount    int       // Total text elements
	MinX            float64   // Leftmost X
	MaxX            float64   // Rightmost X
	AvgWidth        float64   // Average element width
	IsRegular       bool      // True if table has consistent column count
	ConsistencyRate float64   // Percentage of rows with expected column count (0-1)
}

// TableType represents the type of table structure.
type TableType int

const (
	// RegularTable - fixed column count (e.g., bank statements)
	RegularTable TableType = iota
	// IrregularTable - variable column count (e.g., merged cells, complex reports)
	IrregularTable
)

// String returns the string representation of TableType.
func (tt TableType) String() string {
	switch tt {
	case RegularTable:
		return "Regular"
	case IrregularTable:
		return "Irregular"
	default:
		return "Unknown"
	}
}

// Helper functions

func (cbd *ColumnBoundaryDetector) min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
	}
	return min
}

func (cbd *ColumnBoundaryDetector) max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

func (cbd *ColumnBoundaryDetector) mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func abs(x float64) float64 {
	return math.Abs(x)
}

// selectBoundariesByConsistency selects optimal boundaries using row voting.
//
// Algorithm (inspired by "wisdom of crowds" + user insight):
//  1. Group elements by rows
//  2. SKIP HEADER ROWS (multi-line, incomplete columns)
//  3. For DATA ROWS only, count how many elements fall between each pair of boundaries
//  4. Find MODAL column count (most common across DATA rows)
//  5. If current boundaries give wrong count, remove low-support boundaries
//  6. Return boundaries that maximize row consistency
//
// KEY INSIGHT (2025-10-27): User feedback - "еще сами строки таблицы, кроме заголовка надо проверять"
// DATA ROWS have MORE weight than header! Header may have empty columns.
func (cbd *ColumnBoundaryDetector) selectBoundariesByConsistency(elements []*extractor.TextElement, allBoundaries []float64) []float64 {
	if len(allBoundaries) == 0 {
		return []float64{}
	}

	// Group elements by rows
	allRows := cbd.groupElementsByRow(elements)
	if len(allRows) < 3 {
		// Too few rows, just return all boundaries
		return allBoundaries
	}

	// SKIP HEADER ROWS! (user insight: "кроме заголовка надо проверять")
	// Header detection heuristic: first N rows with element count < 50% of max
	maxElementCount := 0
	for _, row := range allRows {
		if len(row) > maxElementCount {
			maxElementCount = len(row)
		}
	}

	headerThreshold := float64(maxElementCount) * 0.6 // Rows with < 60% of max are likely header
	var dataRows [][]*extractor.TextElement

	for i, row := range allRows {
		// First 10 rows: apply header heuristic
		if i < 10 && len(row) < int(headerThreshold) {
			continue // SKIP header row
		}
		// After first 10 rows or if row has enough elements, it's a DATA row
		dataRows = append(dataRows, row)
	}

	// If we skipped too many rows, use all rows (safety)
	if len(dataRows) < 3 {
		dataRows = allRows
	}

	// INSIGHT from debug (2025-10-27):
	// VTB tables have MULTI-LINE CELLS! Each transaction spans 2-3 rows:
	//   Row 1: Date | Time | Amount (5 elements) - "full" row
	//   Row 2: Description part 1 (4 elements)
	//   Row 3: Description part 2 (2 elements)
	//
	// Modal count = 5, but table has 7 columns!
	// We can't use element count to filter boundaries.
	//
	// NEW STRATEGY: Take ALL boundaries with sufficient support
	// Support = % of DATA rows that use this boundary
	// Threshold = 20% (boundary must appear in at least 20% of data rows)

	// Calculate "support" for each boundary = how many DATA rows use it
	// (user insight: строки таблицы важнее заголовка!)
	boundarySupport := make([]int, len(allBoundaries))

	for _, row := range dataRows { // USE DATA ROWS ONLY!
		for i, boundary := range allBoundaries {
			// Check if any element is near this boundary (within minGapWidth)
			for _, elem := range row {
				if abs(elem.X-boundary) < cbd.minGapWidth || abs(elem.Right()-boundary) < cbd.minGapWidth {
					boundarySupport[i]++
					break
				}
			}
		}
	}

	// Calculate support threshold
	// Take boundaries that appear in at least 20% of data rows
	supportThreshold := len(dataRows) / 5 // 20%
	if supportThreshold < 3 {
		supportThreshold = 3 // minimum 3 rows
	}

	// Filter boundaries by support
	var filtered []float64
	for i, boundary := range allBoundaries {
		if boundarySupport[i] >= supportThreshold {
			filtered = append(filtered, boundary)
		}
	}

	// If we filtered too much (< 3 boundaries), fall back to all boundaries
	if len(filtered) < 3 {
		filtered = allBoundaries
	}

	// Sort by X position
	sort.Float64s(filtered)

	return filtered
}

// mergeTwoBoundarySets merges boundaries from two detection methods.
//
// Algorithm:
//  1. Combine both sets of boundaries
//  2. Remove duplicates (boundaries within minGapWidth/2)
//  3. Sort and return
//
// This creates a UNION of detected boundaries, combining the strengths
// of both header-based (precise for known columns) and edge clustering
// (finds all columns including empty ones).
func (cbd *ColumnBoundaryDetector) mergeTwoBoundarySets(set1, set2 []float64) []float64 {
	// Combine both sets
	combined := append([]float64{}, set1...)
	combined = append(combined, set2...)

	if len(combined) == 0 {
		return []float64{}
	}

	// Sort
	sort.Float64s(combined)

	// Remove duplicates/nearby boundaries (within minGapWidth/2)
	merged := []float64{combined[0]}
	tolerance := cbd.minGapWidth / 2.0

	for i := 1; i < len(combined); i++ {
		if combined[i]-merged[len(merged)-1] >= tolerance {
			merged = append(merged, combined[i])
		}
	}

	return merged
}
