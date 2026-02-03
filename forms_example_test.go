package gxpdf_test

import (
	"fmt"
	"log"

	"github.com/coregx/gxpdf"
)

// This file contains testable examples for the public Forms API.
// Run with: go test -v -run Example

func ExampleDocument_HasForm() {
	// Check if a PDF has an interactive form
	doc, err := gxpdf.Open("testdata/sample.pdf")
	if err != nil {
		// Expected if file doesn't exist
		fmt.Println("Has form: false (no file)")
		return
	}
	defer doc.Close()

	hasForm := doc.HasForm()
	fmt.Printf("Has form: %v\n", hasForm)
}

func ExampleDocument_GetFormFields() {
	doc, err := gxpdf.Open("form.pdf")
	if err != nil {
		log.Printf("Could not open: %v", err)
		return
	}
	defer doc.Close()

	fields, err := doc.GetFormFields()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	for _, f := range fields {
		fmt.Printf("%s (%s): %v\n", f.Name(), f.Type(), f.Value())
	}
}

func ExampleDocument_GetFieldValue() {
	doc, err := gxpdf.Open("form.pdf")
	if err != nil {
		log.Printf("Could not open: %v", err)
		return
	}
	defer doc.Close()

	value, err := doc.GetFieldValue("customer_name")
	if err != nil {
		fmt.Printf("Field not found: %v\n", err)
		return
	}

	fmt.Printf("Customer name: %v\n", value)
}

func ExampleFormField() {
	doc, err := gxpdf.Open("form.pdf")
	if err != nil {
		log.Printf("Could not open: %v", err)
		return
	}
	defer doc.Close()

	fields, _ := doc.GetFormFields()
	if len(fields) == 0 {
		return
	}

	// FormField provides type-safe access to field properties
	field := fields[0]

	fmt.Printf("Name: %s\n", field.Name())
	fmt.Printf("Type: %s\n", field.Type())
	fmt.Printf("Value: %v\n", field.Value())
	fmt.Printf("ReadOnly: %v\n", field.IsReadOnly())
	fmt.Printf("Required: %v\n", field.IsRequired())

	// Type-specific checks
	if field.IsTextField() {
		fmt.Println("This is a text field")
	}
	if field.IsButton() {
		fmt.Println("This is a button (checkbox/radio)")
	}
	if field.IsChoice() {
		fmt.Printf("Options: %v\n", field.Options())
	}
}
