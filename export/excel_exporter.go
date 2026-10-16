// Package export provides table export functionality.
package export

import (
	"bytes"
	"fmt"
	"io"

	"github.com/coregx/gxpdf/internal/models/table"
	"github.com/xuri/excelize/v2"
)

// alignCenter is the excelize alignment value for centered cells.
const alignCenter = "center"

// ExcelExporter exports tables to Excel format (XLSX).
//
// Excel export provides rich formatting and layout capabilities.
//
// Features:
//   - Full Excel XLSX format support
//   - Merged cells (row and column spans)
//   - Cell alignment (left, center, right)
//   - Multiple sheets (if needed)
//   - Professional formatting
//
// Limitations:
//   - Binary format (larger than CSV/JSON)
//   - Requires excelize library
//
// Example usage:
//
//	exporter := export.NewExcelExporter()
//	err := exporter.Export(table, file)
type ExcelExporter struct {
	options   *ExportOptions
	sheetName string
}

// excelStyles holds pre-created style IDs for the exporter.
type excelStyles struct {
	header int
	center int
	right  int
}

// NewExcelExporter creates a new Excel exporter with default options.
func NewExcelExporter() *ExcelExporter {
	return &ExcelExporter{
		options:   DefaultExportOptions(),
		sheetName: "Table",
	}
}

// NewExcelExporterWithOptions creates a new Excel exporter with custom options.
func NewExcelExporterWithOptions(options *ExportOptions) *ExcelExporter {
	if options == nil {
		options = DefaultExportOptions()
	}
	return &ExcelExporter{
		options:   options,
		sheetName: "Table",
	}
}

// WithSheetName returns a new ExcelExporter with a custom sheet name.
func (e *ExcelExporter) WithSheetName(name string) *ExcelExporter {
	return &ExcelExporter{
		options:   e.options,
		sheetName: name,
	}
}

// WithMergedCells returns a new ExcelExporter with merged cells enabled/disabled.
func (e *ExcelExporter) WithMergedCells(preserve bool) *ExcelExporter {
	opts := *e.options
	opts.PreserveSpans = preserve
	return &ExcelExporter{
		options:   &opts,
		sheetName: e.sheetName,
	}
}

// Export writes the table to the writer in Excel format.
func (e *ExcelExporter) Export(tbl *table.Table, w io.Writer) error {
	if tbl == nil {
		return fmt.Errorf("table is nil")
	}

	if err := tbl.Validate(); err != nil {
		return fmt.Errorf("invalid table: %w", err)
	}

	// Create new Excel file.
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	// Setup sheet.
	if err := e.setupSheet(f); err != nil {
		return err
	}

	// Create styles.
	styles, err := e.createStyles(f)
	if err != nil {
		return err
	}

	// Write all cells.
	if err := e.writeCells(f, tbl, styles); err != nil {
		return err
	}

	// Auto-fit columns (non-fatal).
	_ = e.autoFitColumns(f, e.sheetName, tbl)

	// Write to writer.
	if err := f.Write(w); err != nil {
		return fmt.Errorf("failed to write Excel file: %w", err)
	}

	return nil
}

// setupSheet creates the sheet and removes the default Sheet1 if needed.
func (e *ExcelExporter) setupSheet(f *excelize.File) error {
	index, err := f.NewSheet(e.sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(index)

	// Delete default "Sheet1" if we created a new sheet.
	if e.sheetName != "Sheet1" {
		_ = f.DeleteSheet("Sheet1")
	}

	return nil
}

// createStyles creates all needed Excel styles.
func (e *ExcelExporter) createStyles(f *excelize.File) (*excelStyles, error) {
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: alignCenter, Vertical: alignCenter},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#E0E0E0"}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create header style: %w", err)
	}

	centerStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: alignCenter, Vertical: "top"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create center style: %w", err)
	}

	rightStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "top"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create right style: %w", err)
	}

	return &excelStyles{header: headerStyle, center: centerStyle, right: rightStyle}, nil
}

// writeCells writes all table cells to the Excel file.
func (e *ExcelExporter) writeCells(f *excelize.File, tbl *table.Table, styles *excelStyles) error {
	for r := 0; r < tbl.RowCount; r++ {
		for c := 0; c < tbl.ColCount; c++ {
			cell := tbl.GetCell(r, c)
			if cell == nil {
				continue
			}

			if err := e.writeCell(f, r, c, cell, styles); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeCell writes a single cell to the Excel file.
func (e *ExcelExporter) writeCell(f *excelize.File, r, c int, cell *table.Cell, styles *excelStyles) error {
	// Excel uses 1-based indexing.
	cellName, err := excelize.CoordinatesToCellName(c+1, r+1)
	if err != nil {
		return fmt.Errorf("invalid cell coordinates (%d,%d): %w", r, c, err)
	}

	// Set cell value.
	if err := f.SetCellValue(e.sheetName, cellName, cell.Text); err != nil {
		return fmt.Errorf("failed to set cell %s: %w", cellName, err)
	}

	// Apply style.
	styleID := e.selectStyle(r, cell, styles)
	if styleID > 0 {
		if err := f.SetCellStyle(e.sheetName, cellName, cellName, styleID); err != nil {
			return fmt.Errorf("failed to set cell style %s: %w", cellName, err)
		}
	}

	// Handle merged cells if option is enabled.
	if e.options.PreserveSpans && cell.IsMerged() {
		if err := e.mergeCells(f, e.sheetName, r, c, cell); err != nil {
			return fmt.Errorf("failed to merge cells at (%d,%d): %w", r, c, err)
		}
	}

	return nil
}

// selectStyle returns the appropriate style ID for a cell.
func (e *ExcelExporter) selectStyle(row int, cell *table.Cell, styles *excelStyles) int {
	if row == 0 {
		return styles.header
	}
	switch cell.TextAlign {
	case table.AlignCenter:
		return styles.center
	case table.AlignRight:
		return styles.right
	default:
		return 0
	}
}

// mergeCells merges cells for a cell with row/col span.
func (e *ExcelExporter) mergeCells(f *excelize.File, sheet string, row, col int, cell *table.Cell) error {
	// Excel uses 1-based indexing.
	startCell, err := excelize.CoordinatesToCellName(col+1, row+1)
	if err != nil {
		return err
	}

	endCell, err := excelize.CoordinatesToCellName(col+cell.ColSpan, row+cell.RowSpan)
	if err != nil {
		return err
	}

	return f.MergeCell(sheet, startCell, endCell)
}

// autoFitColumns adjusts column widths based on content.
func (e *ExcelExporter) autoFitColumns(f *excelize.File, sheet string, tbl *table.Table) error {
	for c := 0; c < tbl.ColCount; c++ {
		width := e.calculateColumnWidth(tbl, c)

		colName, err := excelize.ColumnNumberToName(c + 1)
		if err != nil {
			continue
		}

		if err := f.SetColWidth(sheet, colName, colName, width); err != nil {
			return err
		}
	}
	return nil
}

// calculateColumnWidth calculates the optimal width for a column.
func (e *ExcelExporter) calculateColumnWidth(tbl *table.Table, col int) float64 {
	const minWidth, maxWidth = 10.0, 50.0

	width := minWidth
	for r := 0; r < tbl.RowCount; r++ {
		cell := tbl.GetCell(r, col)
		if cell != nil {
			cellWidth := float64(len(cell.Text)) * 1.2
			if cellWidth > width {
				width = cellWidth
			}
		}
	}

	if width > maxWidth {
		return maxWidth
	}
	return width
}

// ExportToString is not applicable for Excel (binary format).
//
// Returns an error indicating binary formats should use Export() with a buffer.
func (e *ExcelExporter) ExportToString(tbl *table.Table) (string, error) {
	return "", fmt.Errorf("Excel format is binary; use Export() with a bytes.Buffer instead")
}

// ExportToBytes exports the table to Excel format as bytes.
//
// This is a convenience method for getting Excel content as a byte slice.
func (e *ExcelExporter) ExportToBytes(tbl *table.Table) ([]byte, error) {
	var buf bytes.Buffer
	if err := e.Export(tbl, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ContentType returns the MIME content type for Excel.
func (e *ExcelExporter) ContentType() string {
	return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

// FileExtension returns the file extension for Excel.
func (e *ExcelExporter) FileExtension() string {
	return ".xlsx"
}
