// Package main demonstrates AES encryption for PDF documents.
//
// This example shows how to create encrypted PDFs using both AES-128 and AES-256 algorithms.
package main

import (
	"fmt"
	"os"

	"github.com/coregx/gxpdf/creator"
)

// Demo passwords used for every example document.
const (
	userPassword  = "user123"
	ownerPassword = "owner123"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Example 1: AES-128 encryption (recommended).
	fmt.Println("Creating PDF with AES-128 encryption...")
	if err := createAES128PDF(); err != nil {
		return fmt.Errorf("create AES-128 PDF: %w", err)
	}
	fmt.Println("Created: output-aes128.pdf")

	// Example 2: AES-256 encryption (most secure).
	fmt.Println("\nCreating PDF with AES-256 encryption...")
	if err := createAES256PDF(); err != nil {
		return fmt.Errorf("create AES-256 PDF: %w", err)
	}
	fmt.Println("Created: output-aes256.pdf")

	// Example 3: RC4-128 encryption (legacy, for compatibility).
	fmt.Println("\nCreating PDF with RC4-128 encryption (legacy)...")
	if err := createRC4PDF(); err != nil {
		return fmt.Errorf("create RC4 PDF: %w", err)
	}
	fmt.Println("Created: output-rc4.pdf")

	return nil
}

func createAES128PDF() error {
	c := creator.New()

	// Set encryption with AES-128.
	if err := c.SetEncryption(creator.EncryptionOptions{
		UserPassword:  userPassword,
		OwnerPassword: ownerPassword,
		Permissions:   creator.PermissionPrint | creator.PermissionCopy,
		Algorithm:     creator.EncryptionAES128,
	}); err != nil {
		return err
	}

	// Add content.
	if err := addSampleContent(c, "AES-128 Encrypted PDF"); err != nil {
		return err
	}

	// Write to file.
	return c.WriteToFile("output-aes128.pdf")
}

func createAES256PDF() error {
	c := creator.New()

	// Set encryption with AES-256 (most secure).
	if err := c.SetEncryption(creator.EncryptionOptions{
		UserPassword:  userPassword,
		OwnerPassword: ownerPassword,
		Permissions:   creator.PermissionAll,
		Algorithm:     creator.EncryptionAES256,
	}); err != nil {
		return err
	}

	// Add content.
	if err := addSampleContent(c, "AES-256 Encrypted PDF"); err != nil {
		return err
	}

	// Write to file.
	return c.WriteToFile("output-aes256.pdf")
}

func createRC4PDF() error {
	c := creator.New()

	// Set encryption with RC4-128 (legacy, for compatibility).
	if err := c.SetEncryption(creator.EncryptionOptions{
		UserPassword:  userPassword,
		OwnerPassword: ownerPassword,
		Permissions:   creator.PermissionPrint,
		Algorithm:     creator.EncryptionRC4_128,
	}); err != nil {
		return err
	}

	// Add content.
	if err := addSampleContent(c, "RC4-128 Encrypted PDF (Legacy)"); err != nil {
		return err
	}

	// Write to file.
	return c.WriteToFile("output-rc4.pdf")
}

func addSampleContent(c *creator.Creator, title string) error {
	// Create a new page.
	page, err := c.NewPage()
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}

	// Add title.
	if err := page.AddText(title, 50, 750, creator.HelveticaBold, 24); err != nil {
		return fmt.Errorf("add title: %w", err)
	}

	// Add description.
	desc := "This is an encrypted PDF document created with GxPDF."
	if err := page.AddText(desc, 50, 700, creator.Helvetica, 12); err != nil {
		return fmt.Errorf("add description: %w", err)
	}

	// Add instructions.
	instructions := "User password: " + userPassword + " | Owner password: " + ownerPassword
	if err := page.AddText(instructions, 50, 650, creator.Helvetica, 10); err != nil {
		return fmt.Errorf("add instructions: %w", err)
	}

	return nil
}
