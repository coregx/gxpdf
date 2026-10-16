// Package commands implements the gxpdf CLI commands.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	// formatJSON is the output format name for JSON.
	formatJSON = "json"
	// formatCSV is the output format name for CSV.
	formatCSV = "csv"
	// unknownValue is the default placeholder for unset build-time variables.
	unknownValue = "unknown"
	// devVersion is the default Version when it is not injected at build time.
	devVersion = "dev"
)

var (
	// Version, GitCommit and BuildDate are injected by release builds
	// (-ldflags -X). When unset, the version command falls back to the
	// module version and VCS details recorded by the Go toolchain.
	Version = devVersion
	// GitCommit is the git commit hash (set at build time).
	GitCommit = unknownValue
	// BuildDate is the commit date of the build ({{.CommitDate}} in
	// .goreleaser.yml; vcs.time from the toolchain otherwise).
	BuildDate = unknownValue

	// Global flags.
	outputFormat string
	verbose      bool
)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "gxpdf",
	Short: "GxPDF - Enterprise-grade PDF processing tool",
	Long: `GxPDF is a powerful PDF processing tool for Go.

Features:
  - Table extraction with 100% accuracy on bank statements
  - Text extraction with position information
  - PDF merge, split, rotate operations
  - Encryption and decryption (AES-256, RC4)
  - Watermarking and annotations

Examples:
  gxpdf tables invoice.pdf --format csv
  gxpdf info document.pdf
  gxpdf merge doc1.pdf doc2.pdf -o combined.pdf
  gxpdf encrypt secret.pdf -p password

Documentation: https://github.com/coregx/gxpdf`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags.
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "text", "Output format: text, json, csv")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Add subcommands.
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(tablesCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(textCmd)
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(splitCmd)
	rootCmd.AddCommand(encryptCmd)
	rootCmd.AddCommand(decryptCmd)
}

// printVerbosef prints a message if verbose mode is enabled.
func printVerbosef(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format+"\n", args...)
	}
}
