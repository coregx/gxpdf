package forms_test

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/application/forms"
)

// This file contains testable examples for the Forms API.
// Run with: go test -v -run Example

func ExampleFieldType() {
	// Field types correspond to PDF specification
	fmt.Println("Text field type:", forms.FieldTypeText)
	fmt.Println("Button field type:", forms.FieldTypeButton)
	fmt.Println("Choice field type:", forms.FieldTypeChoice)
	fmt.Println("Signature field type:", forms.FieldTypeSignature)

	// Output:
	// Text field type: Tx
	// Button field type: Btn
	// Choice field type: Ch
	// Signature field type: Sig
}

func ExampleFieldInfo() {
	// Create a field info struct (normally from Reader)
	field := &forms.FieldInfo{
		Name:         "customer_name",
		Type:         forms.FieldTypeText,
		Value:        "John Doe",
		DefaultValue: "",
		Flags:        2, // Required flag
		Rect:         [4]float64{100, 700, 300, 720},
	}

	fmt.Printf("Field: %s\n", field.Name)
	fmt.Printf("Type: %s\n", field.Type)
	fmt.Printf("Value: %v\n", field.Value)
	fmt.Printf("Required: %v\n", field.Flags&2 != 0)
	fmt.Printf("Width: %.0f\n", field.Rect[2]-field.Rect[0])

	// Output:
	// Field: customer_name
	// Type: Tx
	// Value: John Doe
	// Required: true
	// Width: 200
}

func ExampleFieldInfo_choiceField() {
	// Choice field with dropdown options
	field := &forms.FieldInfo{
		Name:    "country",
		Type:    forms.FieldTypeChoice,
		Value:   "USA",
		Options: []string{"USA", "Canada", "Mexico", "UK", "Germany"},
	}

	fmt.Printf("Field: %s (%s)\n", field.Name, field.Type)
	fmt.Printf("Selected: %v\n", field.Value)
	fmt.Printf("Options: %v\n", field.Options)

	// Output:
	// Field: country (Ch)
	// Selected: USA
	// Options: [USA Canada Mexico UK Germany]
}

func ExampleFieldInfo_checkboxField() {
	// Checkbox field (button type)
	field := &forms.FieldInfo{
		Name:  "agree_terms",
		Type:  forms.FieldTypeButton,
		Value: "Yes", // "Yes" = checked, "Off" = unchecked
		Flags: 0,
	}

	isChecked := field.Value == "Yes" || field.Value == true
	fmt.Printf("Field: %s\n", field.Name)
	fmt.Printf("Checked: %v\n", isChecked)

	// Output:
	// Field: agree_terms
	// Checked: true
}

func ExampleWriter() {
	// Create writer (normally with parser.Reader)
	writer := forms.NewWriter(nil)

	// Check initial state
	fmt.Printf("Has updates: %v\n", writer.HasUpdates())
	fmt.Printf("Update count: %d\n", len(writer.GetUpdates()))

	// Output:
	// Has updates: false
	// Update count: 0
}

func ExampleFlattenInfo() {
	// FlattenInfo contains data needed to render a field as static content
	info := &forms.FlattenInfo{
		FieldName:        "signature",
		PageIndex:        0,
		Rect:             [4]float64{100, 100, 300, 150},
		AppearanceStream: []byte("q 1 0 0 1 0 0 cm /Img1 Do Q"),
	}

	fmt.Printf("Field to flatten: %s\n", info.FieldName)
	fmt.Printf("On page: %d\n", info.PageIndex)
	fmt.Printf("Position: (%.0f, %.0f)\n", info.Rect[0], info.Rect[1])
	fmt.Printf("Has appearance: %v\n", len(info.AppearanceStream) > 0)

	// Output:
	// Field to flatten: signature
	// On page: 0
	// Position: (100, 100)
	// Has appearance: true
}

func ExampleFlattener() {
	// Create flattener (normally with parser.Reader)
	flattener := forms.NewFlattener(nil)

	// Check if flattening is possible
	canFlatten := flattener.CanFlatten()
	fmt.Printf("Can flatten: %v\n", canFlatten)

	// Output:
	// Can flatten: false
}
