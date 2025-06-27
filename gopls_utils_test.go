package go_mcp_tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateGoplsPosition(t *testing.T) {
	t.Parallel()

	createTestFile := func(t testing.TB) string {
		// Create temporary test directory
		tempDir := t.TempDir()

		// Test file content with line number tracking
		lines := []string{
			"package testpkg",                       // 1
			"",                                      // 2
			"import \"fmt\"",                        // 3
			"",                                      // 4
			"// MyStruct is a test struct",          // 5
			"type MyStruct struct {",                // 6
			"    Name string",                       // 7
			"    Age  int",                          // 8
			"}",                                     // 9
			"",                                      // 10
			"// NewMyStruct creates a new MyStruct", // 11
			"func NewMyStruct(name string, age int) *MyStruct {", // 12
			"    return &MyStruct{",                              // 13
			"        Name: name,",                                // 14
			"        Age:  age,",                                 // 15
			"    }",                                              // 16
			"}",                                                  // 17
			"",                                                   // 18
			"func testFunc() {",                                  // 19
			"    var MyStruct int",                               // 20 - local variable with same name
			"    fmt.Println(MyStruct)",                          // 21
			"}",                                                  // 22
		}
		testFileContent := strings.Join(lines, "\n")

		testFile := filepath.Join(tempDir, "test.go")
		err := os.WriteFile(testFile, []byte(testFileContent), 0644)
		if err != nil {
			t.Fatal(err)
		}
		return testFile
	}

	testCases := []struct {
		name        string
		line        int
		symbol      string
		expectCol   int
		shouldError bool
	}{
		{
			name:      "struct type declaration",
			line:      6,
			symbol:    "MyStruct",
			expectCol: 6, // "type MyStruct" - M is at position 6 (1-based)
		},
		{
			name:      "function name",
			line:      12,
			symbol:    "NewMyStruct",
			expectCol: 6, // "func NewMyStruct" - N is at position 6
		},
		{
			name:      "field name",
			line:      7,
			symbol:    "Name",
			expectCol: 5, // "    Name string" - N is at position 5
		},
		{
			name:      "variable in assignment",
			line:      14,
			symbol:    "Name",
			expectCol: 9, // "        Name: name," - N is at position 9
		},
		{
			name:      "local variable declaration",
			line:      20,
			symbol:    "MyStruct",
			expectCol: 9, // "    var MyStruct int" - M is at position 9
		},
		{
			name:        "symbol not found",
			line:        1,
			symbol:      "NonExistent",
			shouldError: true,
		},
		{
			name:        "line number too high",
			line:        100,
			symbol:      "MyStruct",
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testFile := createTestFile(t)
			position, err := createGoplsPosition(testFile, tc.line, tc.symbol)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
				return
			}

			// Parse the position string to extract the column
			parts := strings.Split(position, ":")
			if len(parts) != 3 {
				t.Errorf("Expected position format 'file:line:col', got %s", position)
				return
			}

			// Check that the file path is absolute
			absTestFile, _ := filepath.Abs(testFile)
			if parts[0] != absTestFile {
				t.Errorf("Expected absolute file path %s, got %s", absTestFile, parts[0])
			}

			// Check line number - convert expected line to string
			expectedLine := fmt.Sprintf("%d", tc.line)
			if parts[1] != expectedLine {
				t.Errorf("Expected line %s, got %s", expectedLine, parts[1])
			}

			// Check column number
			expectedCol := ""
			switch tc.expectCol {
			case 6:
				expectedCol = "6"
			case 5:
				expectedCol = "5"
			case 9:
				expectedCol = "9"
			}
			if parts[2] != expectedCol {
				t.Errorf("Expected column %s, got %s", expectedCol, parts[2])
			}
		})
	}
}

func TestCreateGoplsPositionWithMultipleOccurrences(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// File with multiple occurrences of the same symbol
	lines := []string{
		"package testpkg",       // 1
		"",                      // 2
		"func test() {",         // 3
		"    var name string",   // 4 - first occurrence of 'name'
		"    name = \"hello\"",  // 5 - second occurrence of 'name'
		"    fmt.Println(name)", // 6 - third occurrence of 'name'
		"}",                     // 7
	}
	content := strings.Join(lines, "\n")

	testFile := filepath.Join(tempDir, "multi.go")
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name      string
		line      int
		symbol    string
		expectCol int
	}{
		{
			name:      "first occurrence",
			line:      4,
			symbol:    "name",
			expectCol: 9, // "    var name string" - n is at position 9
		},
		{
			name:      "second occurrence",
			line:      5,
			symbol:    "name",
			expectCol: 5, // "    name = \"hello\"" - n is at position 5
		},
		{
			name:      "third occurrence",
			line:      6,
			symbol:    "name",
			expectCol: 17, // "    fmt.Println(name)" - n is at position 17
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			position, err := createGoplsPosition(testFile, tc.line, tc.symbol)
			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
				return
			}

			// Parse the position string to extract the column
			parts := strings.Split(position, ":")
			if len(parts) != 3 {
				t.Errorf("Expected position format 'file:line:col', got %s", position)
				return
			}

			expectedCol := ""
			switch tc.expectCol {
			case 9:
				expectedCol = "9"
			case 5:
				expectedCol = "5"
			case 17:
				expectedCol = "17"
			}
			if parts[2] != expectedCol {
				t.Errorf("Expected column %s, got %s", expectedCol, parts[2])
			}
		})
	}
}

func TestCreateGoplsPositionEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("file does not exist", func(t *testing.T) {
		t.Parallel()
		_, err := createGoplsPosition("/non/existent/file.go", 1, "symbol")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	t.Run("empty symbol name", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.go")
		err := os.WriteFile(testFile, []byte("package test\n"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		_, err = createGoplsPosition(testFile, 1, "")
		if err == nil {
			t.Error("Expected error for empty symbol name")
		}
	})

	t.Run("zero line number", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.go")
		err := os.WriteFile(testFile, []byte("package test\n"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		_, err = createGoplsPosition(testFile, 0, "test")
		if err == nil {
			t.Error("Expected error for zero line number")
		}
	})
}
