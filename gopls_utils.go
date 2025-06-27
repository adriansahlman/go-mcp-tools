package go_mcp_tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// executeGoplsCommand executes a gopls command with the given arguments
// Returns the trimmed output string or an error with helpful context
func executeGoplsCommand(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("no arguments provided to gopls command")
	}

	// Create the command
	cmd := exec.Command("gopls", args...)

	// Set working directory to the directory of the first file argument if it exists
	// Look for file path in arguments (typically contains .go)
	for _, arg := range args {
		if strings.Contains(arg, ".go:") {
			// Extract file path from position string (file:line:column)
			parts := strings.Split(arg, ":")
			if len(parts) >= 1 {
				cmd.Dir = filepath.Dir(parts[0])
				break
			}
		} else if strings.HasSuffix(arg, ".go") {
			cmd.Dir = filepath.Dir(arg)
			break
		}
	}

	// Execute the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to provide a more helpful error message
		outputStr := strings.TrimSpace(string(output))
		if outputStr == "" {
			return "", fmt.Errorf("gopls command failed: %w", err)
		}
		return "", fmt.Errorf("gopls command failed: %w (%s)", err, outputStr)
	}

	// Return trimmed output
	return strings.TrimSpace(string(output)), nil
}

// createGoplsPosition creates a position string for gopls commands
// It finds the column position of the symbol at the given line and formats it as file:line:column
func createGoplsPosition(
	filePath string,
	lineNumber int,
	symbolName string,
) (string, error) {
	// Validate inputs early
	if lineNumber <= 0 {
		return "", fmt.Errorf(
			"invalid line number: %d (must be greater than 0)",
			lineNumber,
		)
	}

	if symbolName == "" {
		return "", fmt.Errorf("symbol name cannot be empty")
	}

	// helper functions (only needed for this function, therefore self contained)
	// isIdentifierChar checks if a character can be part of a Go identifier
	isIdentifierChar := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_'
	}
	// isWordBoundary checks if the symbol at the given position is at a word boundary
	isWordBoundary := func(line string, index int, symbol string) bool {
		// Check character before the symbol
		if index > 0 {
			prevChar := rune(line[index-1])
			if isIdentifierChar(prevChar) {
				return false
			}
		}

		// Check character after the symbol
		endIndex := index + len(symbol)
		if endIndex < len(line) {
			nextChar := rune(line[endIndex])
			if isIdentifierChar(nextChar) {
				return false
			}
		}

		return true
	}
	// findSymbolColumnPosition finds the column position of a symbol at the given line
	findSymbolColumnPosition := func(
		filePath string,
		lineNumber int,
		symbolName string,
	) (int, error) {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return 0, fmt.Errorf("failed to read file: %w", err)
		}

		lines := strings.Split(string(content), "\n")
		if lineNumber > len(lines) {
			return 0, fmt.Errorf(
				"line number %d exceeds file length (%d lines)",
				lineNumber,
				len(lines),
			)
		}

		// Get the target line (convert to 0-based index)
		targetLine := lines[lineNumber-1]

		// Find the symbol in the line at a word boundary
		symbolIndex := -1

		// Search for all occurrences of the symbol and find the first one at a word boundary
		for i := 0; i <= len(targetLine)-len(symbolName); i++ {
			if targetLine[i:i+len(symbolName)] == symbolName {
				if isWordBoundary(targetLine, i, symbolName) {
					symbolIndex = i
					break
				}
			}
		}

		if symbolIndex == -1 {
			return 0, fmt.Errorf(
				"symbol '%s' not found at a word boundary at line %d",
				symbolName,
				lineNumber,
			)
		}

		// Return 1-based column position
		return symbolIndex + 1, nil
	}

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", filePath)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %w", filePath, err)
	}

	// Find the column position of the symbol at the given line
	columnPos, err := findSymbolColumnPosition(absPath, lineNumber, symbolName)
	if err != nil {
		return "", fmt.Errorf(
			"failed to find symbol '%s' at line %d in %s: %w",
			symbolName,
			lineNumber,
			absPath,
			err,
		)
	}

	// Create position string for gopls (file:line:column format)
	position := fmt.Sprintf("%s:%d:%d", absPath, lineNumber, columnPos)
	return position, nil
}
