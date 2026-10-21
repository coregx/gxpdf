package extractor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coregx/gxpdf/internal/parser"
)

// countingCancelContext reports cancellation from the cancelAt-th Err call.
type countingCancelContext struct {
	checks   int
	cancelAt int
}

func (ctx *countingCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (ctx *countingCancelContext) Done() <-chan struct{}       { return nil }
func (ctx *countingCancelContext) Value(any) any               { return nil }
func (ctx *countingCancelContext) Err() error {
	ctx.checks++
	if ctx.checks >= ctx.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestTextExtractorCancelsInsideFormContent(t *testing.T) {
	formContent := strings.Repeat("BT /F1 12 Tf 10 10 Td (x) Tj ET\n", 2_000)
	reader := formPage(t, "/Fm1 Do", map[string]string{"Fm1": formContent}, []string{"Fm1"})

	ctx := &countingCancelContext{cancelAt: 500}
	elements, err := NewTextExtractorWithContext(reader, ctx).ExtractFromPage(0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExtractFromPage() error = %v, want context.Canceled", err)
	}
	if elements != nil {
		t.Fatalf("ExtractFromPage() returned %d elements after cancellation", len(elements))
	}
	if ctx.checks > ctx.cancelAt+1 {
		t.Fatalf("context checked %d times after cancellation at %d", ctx.checks, ctx.cancelAt)
	}
}

func TestProcessOperatorsCancellationPreservesResourceLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	operators := []*Operator{{Name: "BT"}}

	limitErr := fmt.Errorf("%w: test limit", ErrResourceLimitExceeded)
	extractor := NewTextExtractorWithContext(nil, ctx)
	extractor.limitErr = limitErr
	extractor.processOperators(operators)
	if extractor.limitErr != limitErr {
		t.Fatalf("limitErr = %v, want the original resource-limit error", extractor.limitErr)
	}
}

func TestProcessOperatorsStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	extractor := NewTextExtractorWithContext(nil, ctx)
	extractor.processOperators([]*Operator{{Name: "Tc", Operands: []parser.PdfObject{parser.NewReal(5)}}})
	if extractor.textState.CharSpace != 0 {
		t.Fatalf("CharSpace = %v, want 0: an operator ran after cancellation", extractor.textState.CharSpace)
	}
}
