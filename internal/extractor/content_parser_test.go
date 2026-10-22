package extractor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContentParser(t *testing.T) {
	content := []byte("BT ET")
	parser := NewContentParser(content)

	assert.NotNil(t, parser)
	assert.NotNil(t, parser.lexer)
}

func TestContentParser_ParseOperators_Simple(t *testing.T) {
	content := []byte("BT ET")
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Equal(t, 2, len(operators))

	assert.Equal(t, "BT", operators[0].Name)
	assert.Equal(t, 0, len(operators[0].Operands))

	assert.Equal(t, "ET", operators[1].Name)
	assert.Equal(t, 0, len(operators[1].Operands))
}

func TestContentParser_ParseOperators_WithOperands(t *testing.T) {
	content := []byte("100 200 Td")
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Equal(t, 1, len(operators))

	op := operators[0]
	assert.Equal(t, "Td", op.Name)
	assert.Equal(t, 2, len(op.Operands))
}

func TestContentParser_ParseOperators_TextShowing(t *testing.T) {
	content := []byte("(Hello, World!) Tj")
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Equal(t, 1, len(operators))

	op := operators[0]
	assert.Equal(t, "Tj", op.Name)
	assert.Equal(t, 1, len(op.Operands))
}

func TestContentParser_ParseOperators_SetFont(t *testing.T) {
	content := []byte("/F1 12 Tf")
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Equal(t, 1, len(operators))

	op := operators[0]
	assert.Equal(t, "Tf", op.Name)
	assert.Equal(t, 2, len(op.Operands))
}

func TestContentParser_ParseOperators_SetTextMatrix(t *testing.T) {
	content := []byte("1 0 0 1 100 200 Tm")
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Equal(t, 1, len(operators))

	op := operators[0]
	assert.Equal(t, "Tm", op.Name)
	assert.Equal(t, 6, len(op.Operands))
}

func TestContentParser_ParseOperators_MultipleOperators(t *testing.T) {
	content := []byte(`
		BT
		/F1 12 Tf
		100 200 Td
		(Hello) Tj
		ET
	`)
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Equal(t, 5, len(operators))

	assert.Equal(t, "BT", operators[0].Name)
	assert.Equal(t, "Tf", operators[1].Name)
	assert.Equal(t, "Td", operators[2].Name)
	assert.Equal(t, "Tj", operators[3].Name)
	assert.Equal(t, "ET", operators[4].Name)
}

func TestContentParser_ParseOperators_RealNumbers(t *testing.T) {
	content := []byte("100.5 200.75 Td")
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Equal(t, 1, len(operators))

	op := operators[0]
	assert.Equal(t, "Td", op.Name)
	assert.Equal(t, 2, len(op.Operands))
}

func TestContentParser_ParseOperators_NegativeNumbers(t *testing.T) {
	content := []byte("-100 -200 Td")
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Equal(t, 1, len(operators))

	op := operators[0]
	assert.Equal(t, "Td", op.Name)
	assert.Equal(t, 2, len(op.Operands))
}

func TestContentParser_ParseOperators_Empty(t *testing.T) {
	content := []byte("")
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	assert.Equal(t, 0, len(operators))
}

func TestContentParser_ParseOperators_WhitespaceVariations(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"spaces", "100 200 Td"},
		{"tabs", "100\t200\tTd"},
		{"newlines", "100\n200\nTd"},
		{"mixed", "100  \t\n  200  \t\n  Td"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewContentParser([]byte(tt.content))
			operators, err := parser.ParseOperators()
			require.NoError(t, err)
			require.Equal(t, 1, len(operators))
			assert.Equal(t, "Td", operators[0].Name)
			assert.Equal(t, 2, len(operators[0].Operands))
		})
	}
}

func TestOperator_String(t *testing.T) {
	op := NewOperator("Tj", nil)
	str := op.String()

	assert.Contains(t, str, "Tj")
	assert.Contains(t, str, "operands=0")
}

func TestContentParser_ParseOperators_Comments(t *testing.T) {
	content := []byte(`
		% This is a comment
		BT
		% Another comment
		ET
	`)
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Equal(t, 2, len(operators))

	assert.Equal(t, "BT", operators[0].Name)
	assert.Equal(t, "ET", operators[1].Name)
}

func TestContentParser_ParseOperators_ComplexExample(t *testing.T) {
	content := []byte(`
		BT
		/F1 12 Tf
		1 0 0 1 100 700 Tm
		(Hello, ) Tj
		(World!) Tj
		0 -14 Td
		(Next line) Tj
		ET
	`)
	parser := NewContentParser(content)

	operators, err := parser.ParseOperators()
	require.NoError(t, err)
	require.Greater(t, len(operators), 0)

	// Verify we got the operators
	names := make([]string, len(operators))
	for i, op := range operators {
		names[i] = op.Name
	}

	assert.Contains(t, names, "BT")
	assert.Contains(t, names, "Tf")
	assert.Contains(t, names, "Tm")
	assert.Contains(t, names, "Tj")
	assert.Contains(t, names, "Td")
	assert.Contains(t, names, "ET")
}

// TestContentParser_SkipsInlineImage covers an inline image in a content
// stream: its binary payload must not be tokenized as operators, and parsing
// must continue with the operators that follow it.
func TestContentParser_SkipsInlineImage(t *testing.T) {
	content := []byte("q BI /W 2 /H 2 /BPC 8 /CS /G ID \x78\x9c\x01\x02\xff\nEI Q BT /F1 12 Tf ET")
	parser := NewContentParser(content)

	ops, err := parser.ParseOperators()
	require.NoError(t, err)

	names := make([]string, 0, len(ops))
	for _, op := range ops {
		names = append(names, op.Name)
	}
	assert.Equal(t, []string{"q", "BI", "ID", "Q", "BT", "Tf", "ET"}, names)
}
