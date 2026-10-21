// Package extractor implements PDF content extraction use cases.
package extractor

import (
	"math"
	"sort"
	"strings"
)

// CellExtractor extracts text content from a rectangular cell region.
//
// The extractor is stateful: it tracks which text elements have been
// assigned to a cell. Each element is assigned to at most one cell
// (first-come-first-served based on extraction order). This prevents
// multi-line text from bleeding into adjacent grid cells.
//
// This is a critical component for table extraction (Phase 2.7).
type CellExtractor struct {
	textElements []*TextElement
	assigned     map[*TextElement]bool
	// gridRows stores Y coordinates of grid row boundaries (sorted ascending).
	// Used for grid-aware visual grouping: elements are only grouped if
	// they don't cross a grid row boundary. This prevents grouping sections
	// from different courses that happen to be at similar X/Y positions.
	// nil = no grid awareness (legacy behavior).
	gridRows []float64
}

// NewCellExtractor creates a new CellExtractor with the given text elements.
func NewCellExtractor(textElements []*TextElement) *CellExtractor {
	return &CellExtractor{
		textElements: textElements,
		assigned:     make(map[*TextElement]bool),
	}
}

// WithGridRows sets grid row boundaries for grid-aware visual grouping.
// When set, FindElementsInBounds will include adjacent-line elements
// that belong to the same visual block, but only if they don't cross
// a grid row boundary.
func (ce *CellExtractor) WithGridRows(rows []float64) *CellExtractor {
	ce.gridRows = rows
	return ce
}

// ExtractCellContent extracts text from a rectangular region (cell bounds).
//
// Algorithm:
//  1. Find all text elements within the cell bounds
//  2. Group text elements by line (based on Y position)
//  3. Sort lines from top to bottom
//  4. Within each line, sort elements left to right
//  5. Join text with appropriate spacing
//
// Parameters:
//   - bounds: The rectangular region to extract text from
//
// Returns the extracted text, or empty string if no text is found.
func (ce *CellExtractor) ExtractCellContent(bounds Rectangle) string {
	elementsInCell := ce.FindElementsInBounds(bounds)
	if len(elementsInCell) == 0 {
		return ""
	}

	// Mark elements as assigned so they won't appear in adjacent cells.
	ce.MarkAssigned(elementsInCell)

	lines := ce.groupByLine(elementsInCell)
	ce.sortLines(lines)
	return ce.buildTextFromLines(lines)
}

// cellBoundsPadding is added to cell bounds when checking text element
// containment. Text positioned near grid lines (within a few points) may
// have its center point outside the strict cell rectangle due to coordinate
// transformation precision, font metrics, or baseline vs bounding-box
// differences. A 2pt padding matches the grid builder tolerance.
const cellBoundsPadding = 2.0

// FindElementsInBounds returns all text elements that are within the bounds.
//
// An element is considered "within" if its center point is inside the bounds
// expanded by cellBoundsPadding on all sides. This handles coordinate
// precision issues at grid boundaries without pulling in text from adjacent cells
// (grid rows are typically 10-15pt apart).
//
// This method is exported for use by other extractors (e.g., table alignment detection).
func (ce *CellExtractor) FindElementsInBounds(bounds Rectangle) []*TextElement {
	var result []*TextElement

	expanded := NewRectangle(
		bounds.X-cellBoundsPadding,
		bounds.Y-cellBoundsPadding,
		bounds.Width+2*cellBoundsPadding,
		bounds.Height+2*cellBoundsPadding,
	)

	for _, elem := range ce.textElements {
		if ce.assigned[elem] {
			continue
		}
		if expanded.Contains(elem.CenterX(), elem.CenterY()) {
			result = append(result, elem)
		}
	}

	// Grid-aware continuation: only expand when initial text looks
	// truncated (ends with comma = incomplete section list).
	// This avoids regressions from pulling adjacent course sections.
	if len(ce.gridRows) > 0 && len(result) > 0 {
		text := ce.buildTextFromFoundElements(result)
		if strings.HasSuffix(strings.TrimSpace(text), ",") {
			result = ce.expandWithContinuations(result, bounds)
		}
	}

	return result
}

// expandWithContinuations adds text elements that are visually connected
// to already-found elements but outside cell bounds. Only expands within
// the same grid row band (between two consecutive grid row Y coordinates).
func (ce *CellExtractor) expandWithContinuations(found []*TextElement, bounds Rectangle) []*TextElement {
	foundSet := make(map[*TextElement]bool, len(found))
	for _, e := range found {
		foundSet[e] = true
	}

	// Determine the grid row band for these elements
	gridBandMin, gridBandMax := ce.findGridBand(bounds.Y, bounds.Y+bounds.Height)

	var added []*TextElement
	for _, elem := range ce.textElements {
		if ce.assigned[elem] || foundSet[elem] {
			continue
		}
		// Must be STRICTLY within grid band — no padding
		if elem.CenterY() < gridBandMin || elem.CenterY() > gridBandMax {
			continue
		}
		// Must be in same X column (cell bounds only, minimal padding)
		if elem.CenterX() < bounds.X-1 || elem.CenterX() > bounds.X+bounds.Width+1 {
			continue
		}
		// Must be visually connected to a found element
		if ce.isVisualContinuation(elem, found) {
			added = append(added, elem)
		}
	}

	return append(found, added...)
}

// findGridBand returns the Y range of the grid row band that contains
// the given Y range. Grid rows define bands between consecutive Y values.
func (ce *CellExtractor) findGridBand(yMin, yMax float64) (float64, float64) {
	if len(ce.gridRows) < 2 {
		return yMin, yMax
	}
	bandMin := ce.gridRows[0]
	bandMax := ce.gridRows[len(ce.gridRows)-1]
	for i := 0; i < len(ce.gridRows)-1; i++ {
		if yMin >= ce.gridRows[i]-cellBoundsPadding && yMin <= ce.gridRows[i+1]+cellBoundsPadding {
			bandMin = ce.gridRows[i]
			bandMax = ce.gridRows[i+1]
			break
		}
	}
	return bandMin, bandMax
}

// isVisualContinuation checks if elem is a visual continuation of any
// found element. Strict criteria to prevent pulling course titles from
// adjacent rows:
//   - Same X start (within 3pt)
//   - Adjacent Y (within 1.5× fontSize)
//   - Same font size
//   - Text must look like section codes (short, comma-separated),
//     NOT a course title (multi-word ALL CAPS)
func (ce *CellExtractor) isVisualContinuation(elem *TextElement, found []*TextElement) bool {
	// Don't pull course-title-like text as continuation
	text := strings.TrimSpace(elem.Text)
	if isCourseTitle(text) {
		return false
	}

	for _, f := range found {
		if abs(elem.FontSize-f.FontSize) > 1.0 {
			continue
		}
		if abs(elem.X-f.X) > 3.0 {
			continue
		}
		yDist := abs(elem.Y - f.Y)
		maxDist := f.FontSize * 1.5
		if maxDist < 12 {
			maxDist = 12
		}
		if yDist > 0.5 && yDist <= maxDist {
			return true
		}
	}
	return false
}

// buildTextFromFoundElements quickly joins found element text for truncation check.
func (ce *CellExtractor) buildTextFromFoundElements(elements []*TextElement) string {
	var sb strings.Builder
	for _, e := range elements {
		sb.WriteString(e.Text)
	}
	return sb.String()
}

// isCourseTitle checks if text looks like a course title (multi-word ALL CAPS,
// length > 10 chars, contains spaces). Used to prevent pulling titles as
// visual continuations.
func isCourseTitle(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 5 {
		return false
	}
	if !strings.ContainsRune(s, ' ') {
		return false
	}
	// ALL CAPS with no commas → likely course title
	if s == strings.ToUpper(s) && !strings.ContainsRune(s, ',') {
		return true
	}
	return false
}

// MarkAssigned marks the given text elements as assigned to a cell.
// Once assigned, elements will not appear in subsequent FindElementsInBounds calls.
func (ce *CellExtractor) MarkAssigned(elements []*TextElement) {
	for _, e := range elements {
		ce.assigned[e] = true
	}
}

// textLine represents text elements on one legacy Y or transformed baseline.
type textLine struct {
	y        float64        // Average Y position of line (for final sorting)
	minY     float64        // Minimum Y in line
	maxY     float64        // Maximum Y in line
	elements []*TextElement // Elements in this line
	affine   bool           // Elements carry compatible transformed flow geometry
	dirX     float64        // Reading direction for affine element ordering
	dirY     float64
	originX  float64 // Average transformed baseline origin
	originY  float64
}

// groupByLine groups text elements into lines based on Y position.
//
// Elements are considered on the same line if their Y positions are
// within a threshold.
//
// Threshold = 1.5x average font size:
//   - For 10pt font: 15pt tolerance
//   - This accommodates multi-line cells (typical line spacing = 1.2-1.5x font size)
//   - Previous threshold (0.5x) worked for Alfa-Bank (single-line cells)
//     but failed for VTB (multi-line cells with 12-15pt spacing)
//
// See: ANALYSIS_VTB_TABLE_MULTI_LINE_CELLS.md for detailed analysis
func (ce *CellExtractor) groupByLine(elements []*TextElement) []*textLine {
	if len(elements) == 0 {
		return nil
	}
	if allElementsHaveAffineFlow(elements) {
		return groupByAffineLine(elements)
	}

	// Calculate average font size for threshold
	avgFontSize := ce.calculateAverageFontSize(elements)
	threshold := avgFontSize * 0.3 // Group elements on same baseline (< 3pt for 10pt font, tighter for Alfa-Bank)

	// Group elements by line
	var lines []*textLine

	for _, elem := range elements {
		// Find line with similar Y position
		// Check if element Y is within threshold of the line's Y range [minY, maxY]
		var targetLine *textLine
		for _, line := range lines {
			// Check if element is within threshold of existing line
			// Use the closest edge of the line's Y range
			minDist := abs(elem.Y - line.minY)
			maxDist := abs(elem.Y - line.maxY)
			closestDist := minDist
			if maxDist < minDist {
				closestDist = maxDist
			}

			if closestDist < threshold {
				targetLine = line
				break
			}
		}

		// Create new line if not found
		if targetLine == nil {
			targetLine = &textLine{
				y:        elem.Y,
				minY:     elem.Y,
				maxY:     elem.Y,
				elements: []*TextElement{},
			}
			lines = append(lines, targetLine)
		}

		// Add element to line
		targetLine.elements = append(targetLine.elements, elem)

		// Update line Y range and average
		if elem.Y < targetLine.minY {
			targetLine.minY = elem.Y
		}
		if elem.Y > targetLine.maxY {
			targetLine.maxY = elem.Y
		}

		// Update average Y for sorting
		sum := 0.0
		for _, e := range targetLine.elements {
			sum += e.Y
		}
		targetLine.y = sum / float64(len(targetLine.elements))
	}

	return lines
}

func allElementsHaveAffineFlow(elements []*TextElement) bool {
	for _, element := range elements {
		if element == nil || element.flow == nil {
			return false
		}
	}
	return true
}

// groupByAffineLine groups extractor-created runs using the same transformed
// baseline relation as mergeAdjacentElements and AssembleText. Mixed/manual
// elements intentionally use the legacy X/Y path.
func groupByAffineLine(elements []*TextElement) []*textLine {
	var lines []*textLine
	for _, element := range elements {
		var target *textLine
		for _, line := range lines {
			sameLine, _, _, _, comparable := affineTextRelation(line.elements[0], element)
			if comparable && sameLine {
				target = line
				break
			}
		}
		if target == nil {
			target = &textLine{
				y: element.Y, minY: element.Y, maxY: element.Y,
				elements: []*TextElement{}, affine: true,
				dirX: element.flow.directionX, dirY: element.flow.directionY,
			}
			lines = append(lines, target)
		}
		target.elements = append(target.elements, element)
		count := float64(len(target.elements))
		target.originX += (element.flow.originX - target.originX) / count
		target.originY += (element.flow.originY - target.originY) / count
	}
	return lines
}

// sortLines spatially orders compatible affine lines along their canonical
// perpendicular and retains the legacy top-to-bottom rule for axis-aligned
// manually constructed elements. Runs within each line follow their own
// transformed baseline direction.
func (ce *CellExtractor) sortLines(lines []*textLine) {
	if len(lines) == 0 || !lines[0].affine {
		sort.SliceStable(lines, func(i, j int) bool {
			return lines[i].y > lines[j].y // Top to bottom
		})
	} else {
		sortAffineLineCohorts(lines)
	}

	// Sort legacy elements left to right and affine elements along their
	// transformed baseline, including reflected and rotated reading directions.
	for _, line := range lines {
		sort.SliceStable(line.elements, func(i, j int) bool {
			if line.affine {
				left := line.elements[i].flow
				right := line.elements[j].flow
				leftPosition := quantizeAffinePosition(left.originX*line.dirX + left.originY*line.dirY)
				rightPosition := quantizeAffinePosition(right.originX*line.dirX + right.originY*line.dirY)
				return leftPosition < rightPosition
			}
			return line.elements[i].X < line.elements[j].X
		})
	}
}

const (
	affineOrientationDot         = 0.995
	affinePositionScale          = 1_000_000.0     // one micro-point
	affineAngleScale             = 1_000_000_000.0 // one nanoradian
	affineAxialAnglePeriod int64 = 3_141_592_654   // round(pi * affineAngleScale)
)

type affinePageOrderKey struct {
	top, left, bottom, right int64
}

type affineLineEntry struct {
	line         *textLine
	angleKey     int64
	unwrappedKey int64
	paintOrder   int
}

type affineLineCohort struct {
	entries          []affineLineEntry
	normalX, normalY float64
	orientationKey   int64
	pageKey          affinePageOrderKey
	paintOrder       int
}

// sortAffineLineCohorts treats baseline orientation as an axis (theta modulo
// pi), so reflections and angles on either side of an arbitrary sign seam are
// equivalent. Lines are sorted around the axial circle after its largest gap,
// then greedily partitioned into permutation-independent cohorts whose endpoint
// separation never exceeds the compatibility tolerance. A cohort perpendicular
// is derived from its endpoint midpoint and oriented to agree with page-space
// top-to-bottom/left-to-right geometry. All ordering keys are quantized; paint
// order is used only after the geometric and orientation keys tie exactly.
func sortAffineLineCohorts(lines []*textLine) {
	if len(lines) < 2 {
		return
	}
	cohorts := buildAffineLineCohorts(lines)
	sort.SliceStable(cohorts, func(i, j int) bool {
		left, right := cohorts[i], cohorts[j]
		if order := compareAffinePageKeys(left.pageKey, right.pageKey); order != 0 {
			return order < 0
		}
		if left.orientationKey != right.orientationKey {
			return left.orientationKey < right.orientationKey
		}
		return left.paintOrder < right.paintOrder
	})

	index := 0
	for _, cohort := range cohorts {
		for _, entry := range cohort.entries {
			lines[index] = entry.line
			index++
		}
	}
}

func buildAffineLineCohorts(lines []*textLine) []*affineLineCohort {
	entries := make([]affineLineEntry, len(lines))
	for index, line := range lines {
		entries[index] = affineLineEntry{
			line: line, angleKey: affineAxialAngleKey(line.dirX, line.dirY), paintOrder: index,
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].angleKey != entries[j].angleKey {
			return entries[i].angleKey < entries[j].angleKey
		}
		if order := compareAffinePageKeys(affineLinePageKey(entries[i].line), affineLinePageKey(entries[j].line)); order != 0 {
			return order < 0
		}
		return entries[i].paintOrder < entries[j].paintOrder
	})
	entries = unwrapAffineOrientations(entries)

	cohorts := make([]*affineLineCohort, 0, len(entries))
	for start := 0; start < len(entries); {
		end := start + 1
		for end < len(entries) && affineEntriesCompatible(entries[start], entries[end]) {
			end++
		}
		cohorts = append(cohorts, newAffineLineCohort(entries[start:end]))
		start = end
	}
	return cohorts
}

func unwrapAffineOrientations(entries []affineLineEntry) []affineLineEntry {
	if len(entries) < 2 {
		entries[0].unwrappedKey = entries[0].angleKey
		return entries
	}
	largestGap, start := int64(-1), 0
	for index := range entries {
		next := (index + 1) % len(entries)
		nextKey := entries[next].angleKey
		if next == 0 {
			nextKey += affineAxialAnglePeriod
		}
		gap := nextKey - entries[index].angleKey
		if gap > largestGap || gap == largestGap && entries[next].angleKey < entries[start].angleKey {
			largestGap, start = gap, next
		}
	}
	unwrapped := make([]affineLineEntry, 0, len(entries))
	for offset := range entries {
		index := (start + offset) % len(entries)
		entry := entries[index]
		if index < start {
			entry.unwrappedKey = entry.angleKey + affineAxialAnglePeriod
		} else {
			entry.unwrappedKey = entry.angleKey
		}
		unwrapped = append(unwrapped, entry)
	}
	return unwrapped
}

func affineEntriesCompatible(first, candidate affineLineEntry) bool {
	toleranceKey := quantizeAffineAngle(math.Acos(affineOrientationDot))
	if candidate.unwrappedKey-first.unwrappedKey > toleranceKey {
		return false
	}
	dot := first.line.dirX*candidate.line.dirX + first.line.dirY*candidate.line.dirY
	absoluteDot := math.Min(1, math.Abs(dot))
	return quantizeAffineAngle(math.Acos(absoluteDot)) <= toleranceKey
}

func newAffineLineCohort(entries []affineLineEntry) *affineLineCohort {
	cohort := &affineLineCohort{
		entries: append([]affineLineEntry(nil), entries...),
		pageKey: affinePageOrderKey{
			top: math.MinInt64, left: math.MaxInt64, bottom: math.MaxInt64, right: math.MinInt64,
		},
		paintOrder: entries[0].paintOrder,
	}
	firstKey := entries[0].unwrappedKey
	lastKey := entries[len(entries)-1].unwrappedKey
	centerKey := firstKey + (lastKey-firstKey)/2
	centerAngle := float64(centerKey) / affineAngleScale
	cohort.normalX, cohort.normalY = -math.Sin(centerAngle), math.Cos(centerAngle)
	cohort.orientationKey = centerKey % affineAxialAnglePeriod

	for _, entry := range entries {
		cohort.pageKey = unionAffinePageKeys(cohort.pageKey, affineLinePageKey(entry.line))
		if entry.paintOrder < cohort.paintOrder {
			cohort.paintOrder = entry.paintOrder
		}
	}
	orientAffineCohortNormal(cohort)
	sort.SliceStable(cohort.entries, func(i, j int) bool {
		left, right := cohort.entries[i], cohort.entries[j]
		leftPosition := affineLineProjectionKey(left.line, cohort.normalX, cohort.normalY)
		rightPosition := affineLineProjectionKey(right.line, cohort.normalX, cohort.normalY)
		if leftPosition != rightPosition {
			return leftPosition > rightPosition
		}
		if order := compareAffinePageKeys(affineLinePageKey(left.line), affineLinePageKey(right.line)); order != 0 {
			return order < 0
		}
		return left.paintOrder < right.paintOrder
	})
	return cohort
}

func orientAffineCohortNormal(cohort *affineLineCohort) {
	if len(cohort.entries) < 2 {
		return
	}
	minEntry, maxEntry := cohort.entries[0], cohort.entries[0]
	minProjection := affineLineProjectionKey(minEntry.line, cohort.normalX, cohort.normalY)
	maxProjection := minProjection
	for _, entry := range cohort.entries[1:] {
		projection := affineLineProjectionKey(entry.line, cohort.normalX, cohort.normalY)
		if projection < minProjection {
			minEntry, minProjection = entry, projection
		}
		if projection > maxProjection {
			maxEntry, maxProjection = entry, projection
		}
	}
	if minProjection != maxProjection &&
		compareAffinePageKeys(affineLineOriginPageKey(maxEntry.line), affineLineOriginPageKey(minEntry.line)) > 0 {
		cohort.normalX, cohort.normalY = -cohort.normalX, -cohort.normalY
	}
}

func affineAxialAngleKey(directionX, directionY float64) int64 {
	angle := math.Atan2(directionY, directionX)
	if angle < 0 {
		angle += math.Pi
	}
	if angle >= math.Pi {
		angle -= math.Pi
	}
	key := quantizeAffineAngle(angle)
	if key >= affineAxialAnglePeriod {
		return 0
	}
	return key
}

func affineLineProjectionKey(line *textLine, normalX, normalY float64) int64 {
	return quantizeAffinePosition(line.originX*normalX + line.originY*normalY)
}

func affineLineOriginPageKey(line *textLine) affinePageOrderKey {
	x := quantizeAffinePosition(line.originX)
	y := quantizeAffinePosition(line.originY)
	return affinePageOrderKey{top: y, left: x, bottom: y, right: x}
}

func affineLinePageKey(line *textLine) affinePageOrderKey {
	key := affinePageOrderKey{top: math.MinInt64, left: math.MaxInt64, bottom: math.MaxInt64, right: math.MinInt64}
	for _, element := range line.elements {
		elementKey := affinePageOrderKey{
			top: quantizeAffinePosition(element.Top()), left: quantizeAffinePosition(element.Left()),
			bottom: quantizeAffinePosition(element.Bottom()), right: quantizeAffinePosition(element.Right()),
		}
		key = unionAffinePageKeys(key, elementKey)
	}
	return key
}

func unionAffinePageKeys(left, right affinePageOrderKey) affinePageOrderKey {
	return affinePageOrderKey{
		top: affineMaxInt64(left.top, right.top), left: affineMinInt64(left.left, right.left),
		bottom: affineMinInt64(left.bottom, right.bottom), right: affineMaxInt64(left.right, right.right),
	}
}

func affineMinInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func affineMaxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func compareAffinePageKeys(left, right affinePageOrderKey) int {
	for _, pair := range [][2]int64{
		{right.top, left.top},       // top descending
		{left.left, right.left},     // left ascending
		{right.bottom, left.bottom}, // bottom descending
		{left.right, right.right},   // right ascending
	} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

func quantizeAffinePosition(value float64) int64 {
	return quantizeAffineValue(value, affinePositionScale)
}

func quantizeAffineAngle(value float64) int64 {
	return quantizeAffineValue(value, affineAngleScale)
}

func quantizeAffineValue(value, scale float64) int64 {
	scaled := value * scale
	if scaled >= math.MaxInt64 {
		return math.MaxInt64
	}
	if scaled <= math.MinInt64 {
		return math.MinInt64
	}
	return int64(math.Round(scaled))
}

// buildTextFromLines constructs the final text from sorted lines.
//
// Text elements within a line are joined with spaces.
// Lines are joined with newlines.
func (ce *CellExtractor) buildTextFromLines(lines []*textLine) string {
	var result strings.Builder
	previousLine := ""
	for _, line := range lines {
		lineText := ""
		if line.affine {
			lineText = AssembleText(line.elements)
		} else {
			var legacy strings.Builder
			var previous *TextElement
			previousText := ""
			for _, element := range line.elements {
				if element == nil || element.Text == "" {
					continue
				}
				separator := byte(0)
				if previous != nil && element.X-previous.Right() > 2.0 {
					separator = ' '
				}
				appendTextRun(&legacy, separator, previousText, element.Text)
				previous, previousText = element, element.Text
			}
			lineText = legacy.String()
		}
		if lineText == "" {
			continue
		}
		appendTextRun(&result, '\n', previousLine, lineText)
		previousLine = lineText
	}

	return strings.TrimSpace(result.String())
}

// calculateAverageFontSize calculates the average font size of elements.
func (ce *CellExtractor) calculateAverageFontSize(elements []*TextElement) float64 {
	if len(elements) == 0 {
		return 12.0 // Default
	}

	sum := 0.0
	for _, elem := range elements {
		sum += elem.FontSize
	}
	return sum / float64(len(elements))
}

// abs returns the absolute value of x.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
