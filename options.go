package gxpdf

// ExtractionMethod specifies the table detection algorithm.
type ExtractionMethod int

const (
	// MethodAuto automatically selects the best method.
	// Uses Lattice if ruling lines are detected, otherwise Stream.
	MethodAuto ExtractionMethod = iota

	// MethodLattice uses ruling lines (borders) to detect tables.
	// Best for tables with visible borders.
	MethodLattice

	// MethodStream uses whitespace analysis to detect tables.
	// Best for tables without borders.
	MethodStream

	// MethodHybrid uses the 4-Pass Hybrid algorithm.
	// Best accuracy for complex tables like bank statements.
	MethodHybrid
)

const (
	methodNameLattice = "Lattice"
	methodNameStream  = "Stream"
)

// String returns the name of the extraction method.
func (m ExtractionMethod) String() string {
	switch m {
	case MethodAuto:
		return "Auto"
	case MethodLattice:
		return methodNameLattice
	case MethodStream:
		return methodNameStream
	case MethodHybrid:
		return "Hybrid"
	default:
		return "Unknown"
	}
}

// ExtractionOptions configures table extraction behavior.
type ExtractionOptions struct {
	// Method specifies the table detection algorithm.
	// Default: MethodAuto
	Method ExtractionMethod

	// Pages specifies which pages to process (0-based indices).
	// Empty slice means all pages.
	Pages []int

	// MinRowHeight is the minimum height for a row in points.
	// Rows shorter than this are merged with adjacent rows.
	// Default: 0 (auto-detect)
	MinRowHeight float64

	// MinColumnWidth is the minimum width for a column in points.
	// Default: 0 (auto-detect)
	MinColumnWidth float64

	// MergeMultilineRows merges cells that span multiple lines.
	// Default: true
	MergeMultilineRows bool
}

// DefaultExtractionOptions returns the default extraction options.
func DefaultExtractionOptions() *ExtractionOptions {
	return &ExtractionOptions{
		Method:             MethodAuto,
		Pages:              nil, // All pages
		MinRowHeight:       0,
		MinColumnWidth:     0,
		MergeMultilineRows: true,
	}
}

// WithMethod sets the extraction method.
func (o *ExtractionOptions) WithMethod(method ExtractionMethod) *ExtractionOptions {
	o.Method = method
	return o
}

// WithPages sets the pages to process.
func (o *ExtractionOptions) WithPages(pages ...int) *ExtractionOptions {
	o.Pages = pages
	return o
}

// WithMergeMultilineRows enables or disables multiline row merging.
func (o *ExtractionOptions) WithMergeMultilineRows(merge bool) *ExtractionOptions {
	o.MergeMultilineRows = merge
	return o
}
