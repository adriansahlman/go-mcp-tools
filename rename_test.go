package go_mcp_tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRename(t *testing.T) {
	t.Parallel()

	// Helper function to create a test workspace with Go files
	createTestWorkspace := func(t testing.TB) string {
		tempDir := t.TempDir()

		// Create a go.mod file to make this a proper Go module
		goModContent := "module testmodule\n\ngo 1.21\n"
		goModFile := filepath.Join(tempDir, "go.mod")
		err := os.WriteFile(goModFile, []byte(goModContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Main test file with various symbols to rename
		mainLines := []string{
			"package testpkg",                     // 1
			"",                                    // 2
			"import \"fmt\"",                      // 3
			"",                                    // 4
			"// PersonInterface defines a person", // 5
			"type PersonInterface interface {",    // 6
			"    GetName() string",                // 7
			"    SetAge(int)",                     // 8
			"}",                                   // 9
			"",                                    // 10
			"// Person is a struct representing a person", // 11
			"type Person struct {",                        // 12
			"    Name string",                             // 13
			"    Age  int",                                // 14
			"}",                                           // 15
			"",                                            // 16
			"// GetName returns the person's name",        // 17
			"func (p *Person) GetName() string {",         // 18
			"    return p.Name",                           // 19
			"}",                                           // 20
			"",                                            // 21
			"// SetAge sets the person's age",             // 22
			"func (p *Person) SetAge(age int) {",          // 23
			"    p.Age = age",                             // 24
			"}",                                           // 25
			"",                                            // 26
			"// NewPerson creates a new Person",           // 27
			"func NewPerson(name string, age int) *Person {", // 28
			"    return &Person{",                            // 29
			"        Name: name,",                            // 30
			"        Age:  age,",                             // 31
			"    }",                                          // 32
			"}",                                              // 33
			"",                                               // 34
			"// Constants for testing",                       // 35
			"const (",                                        // 36
			"    DefaultName = \"John\"",                     // 37
			"    DefaultAge  = 30",                           // 38
			")",                                              // 39
			"",                                               // 40
			"// Variables for testing",                       // 41
			"var (",                                          // 42
			"    GlobalCounter int",                          // 43
			"    GlobalMessage string = \"hello\"",           // 44
			")",                                              // 45
			"",                                               // 46
			"// ExampleFunction demonstrates usage",          // 47
			"func ExampleFunction() {",                       // 48
			"    person := NewPerson(DefaultName, DefaultAge)", // 49
			"    fmt.Println(person.GetName())",                // 50
			"    person.SetAge(25)",                            // 51
			"}",                                                // 52
		}
		mainContent := strings.Join(mainLines, "\n")
		mainFile := filepath.Join(tempDir, "main.go")
		err = os.WriteFile(mainFile, []byte(mainContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Secondary file for cross-file renaming tests
		secondaryLines := []string{
			"package testpkg",                      // 1
			"",                                     // 2
			"import \"fmt\"",                       // 3
			"",                                     // 4
			"// Helper uses Person from main.go",   // 5
			"func Helper() {",                      // 6
			"    p := NewPerson(\"Jane\", 25)",     // 7
			"    fmt.Println(p.GetName())",         // 8
			"}",                                    // 9
			"",                                     // 10
			"// ProcessPerson works with Person",   // 11
			"func ProcessPerson(person *Person) {", // 12
			"    person.SetAge(person.Age + 1)",    // 13
			"}",                                    // 14
		}
		secondaryContent := strings.Join(secondaryLines, "\n")
		secondaryFile := filepath.Join(tempDir, "helper.go")
		err = os.WriteFile(secondaryFile, []byte(secondaryContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		return tempDir
	}

	// Helper function to read file content after rename
	readFileContent := func(t testing.TB, filePath string) string {
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file %s: %v", filePath, err)
		}
		return string(content)
	}

	t.Run("successful variable rename", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Rename GlobalCounter to GlobalCount on line 43
		result, err := Rename(mainFile, 43, "GlobalCounter", "GlobalCount")
		if err != nil {
			t.Fatalf("Failed to rename variable: %v", err)
		}

		// gopls rename returns empty output on success
		_ = result

		// Verify the rename was applied
		content := readFileContent(t, mainFile)
		if strings.Contains(content, "GlobalCounter") {
			t.Error("Old variable name 'GlobalCounter' still exists in file")
		}
		if !strings.Contains(content, "GlobalCount") {
			t.Error("New variable name 'GlobalCount' not found in file")
		}
	})

	t.Run("successful function rename", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")
		helperFile := filepath.Join(workspace, "helper.go")

		// Rename NewPerson to CreatePerson on line 28
		result, err := Rename(mainFile, 28, "NewPerson", "CreatePerson")
		if err != nil {
			t.Fatalf("Failed to rename function: %v", err)
		}

		// gopls rename returns empty output on success, but we convert to success message
		_ = result

		// Verify the rename was applied in both files
		mainContent := readFileContent(t, mainFile)
		helperContent := readFileContent(t, helperFile)

		// Check main file
		if strings.Contains(mainContent, "func NewPerson(") {
			t.Error("Old function declaration 'NewPerson' still exists in main file")
		}
		if !strings.Contains(mainContent, "func CreatePerson(") {
			t.Error("New function declaration 'CreatePerson' not found in main file")
		}

		// Check helper file (should be updated due to cross-file usage)
		if strings.Contains(helperContent, "NewPerson(") {
			t.Error("Old function call 'NewPerson' still exists in helper file")
		}
		if !strings.Contains(helperContent, "CreatePerson(") {
			t.Error("New function call 'CreatePerson' not found in helper file")
		}
	})

	t.Run("successful type rename", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")
		helperFile := filepath.Join(workspace, "helper.go")

		// Rename Person to Individual on line 12
		result, err := Rename(mainFile, 12, "Person", "Individual")
		if err != nil {
			t.Fatalf("Failed to rename type: %v", err)
		}

		// gopls rename returns empty output on success, but we convert to success message
		_ = result

		// Verify the rename was applied in both files
		mainContent := readFileContent(t, mainFile)
		helperContent := readFileContent(t, helperFile)

		// Check main file
		if strings.Contains(mainContent, "type Person struct") {
			t.Error("Old type declaration 'Person' still exists in main file")
		}
		if !strings.Contains(mainContent, "type Individual struct") {
			t.Error("New type declaration 'Individual' not found in main file")
		}

		// Check helper file (should be updated due to cross-file usage)
		if strings.Contains(helperContent, "*Person)") {
			t.Error("Old type usage '*Person' still exists in helper file")
		}
		if !strings.Contains(helperContent, "*Individual)") {
			t.Error("New type usage '*Individual' not found in helper file")
		}
	})

	t.Run("successful constant rename", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Rename DefaultName to StandardName on line 37
		result, err := Rename(mainFile, 37, "DefaultName", "StandardName")
		if err != nil {
			t.Fatalf("Failed to rename constant: %v", err)
		}

		// gopls rename returns empty output on success, but we convert to success message
		_ = result

		// Verify the rename was applied
		content := readFileContent(t, mainFile)
		if strings.Contains(content, "DefaultName =") {
			t.Error("Old constant declaration 'DefaultName' still exists")
		}
		if !strings.Contains(content, "StandardName =") {
			t.Error("New constant declaration 'StandardName' not found")
		}
		// Check usage in ExampleFunction
		if strings.Contains(content, "NewPerson(DefaultName,") {
			t.Error("Old constant usage 'DefaultName' still exists")
		}
		if !strings.Contains(content, "NewPerson(StandardName,") {
			t.Error("New constant usage 'StandardName' not found")
		}
	})

	t.Run("successful method rename", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")
		helperFile := filepath.Join(workspace, "helper.go")

		// Rename GetName to GetFullName on line 18
		result, err := Rename(mainFile, 18, "GetName", "GetFullName")
		if err != nil {
			t.Fatalf("Failed to rename method: %v", err)
		}

		// gopls rename returns empty output on success, but we convert to success message
		_ = result

		// Verify the rename was applied in both files
		mainContent := readFileContent(t, mainFile)
		helperContent := readFileContent(t, helperFile)

		// Check main file
		if strings.Contains(mainContent, "func (p *Person) GetName()") {
			t.Error("Old method declaration 'GetName' still exists in main file")
		}
		if !strings.Contains(mainContent, "func (p *Person) GetFullName()") {
			t.Error("New method declaration 'GetFullName' not found in main file")
		}

		// Check helper file
		if strings.Contains(helperContent, "p.GetName()") {
			t.Error("Old method call 'GetName' still exists in helper file")
		}
		if !strings.Contains(helperContent, "p.GetFullName()") {
			t.Error("New method call 'GetFullName' not found in helper file")
		}
	})

	t.Run("same name returns early", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Try to rename Person to Person (same name)
		result, err := Rename(mainFile, 12, "Person", "Person")
		if err != nil {
			t.Fatalf("Unexpected error for same name rename: %v", err)
		}

		expectedMsg := "Symbol 'Person' already has the desired name"
		if result != expectedMsg {
			t.Errorf("Expected message %q, got %q", expectedMsg, result)
		}
	})

	// Edge cases and error conditions
	t.Run("empty file path", func(t *testing.T) {
		t.Parallel()

		_, err := Rename("", 12, "Person", "Individual")
		if err == nil {
			t.Fatal("Expected error for empty file path")
		}
		if !strings.Contains(err.Error(), "file path cannot be empty") {
			t.Errorf("Expected 'file path cannot be empty' error, got: %v", err)
		}
	})

	t.Run("invalid line number zero", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		_, err := Rename(mainFile, 0, "Person", "Individual")
		if err == nil {
			t.Fatal("Expected error for line number 0")
		}
		if !strings.Contains(err.Error(), "line number must be positive") {
			t.Errorf("Expected 'line number must be positive' error, got: %v", err)
		}
	})

	t.Run("invalid line number negative", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		_, err := Rename(mainFile, -5, "Person", "Individual")
		if err == nil {
			t.Fatal("Expected error for negative line number")
		}
		if !strings.Contains(err.Error(), "line number must be positive") {
			t.Errorf("Expected 'line number must be positive' error, got: %v", err)
		}
	})

	t.Run("empty symbol name", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		_, err := Rename(mainFile, 12, "", "Individual")
		if err == nil {
			t.Fatal("Expected error for empty symbol name")
		}
		if !strings.Contains(err.Error(), "symbol name cannot be empty") {
			t.Errorf("Expected 'symbol name cannot be empty' error, got: %v", err)
		}
	})

	t.Run("empty new name", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		_, err := Rename(mainFile, 12, "Person", "")
		if err == nil {
			t.Fatal("Expected error for empty new name")
		}
		if !strings.Contains(err.Error(), "new name cannot be empty") {
			t.Errorf("Expected 'new name cannot be empty' error, got: %v", err)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		t.Parallel()

		nonExistentFile := "/tmp/non_existent_file.go"
		_, err := Rename(nonExistentFile, 12, "Person", "Individual")
		if err == nil {
			t.Fatal("Expected error for non-existent file")
		}
		// This error comes from createGoplsPosition, so check for that
		if !strings.Contains(err.Error(), "file does not exist") {
			t.Errorf("Expected 'file does not exist' error, got: %v", err)
		}
	})

	t.Run("line number exceeds file length", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Try line 1000 when file has much fewer lines
		_, err := Rename(mainFile, 1000, "Person", "Individual")
		if err == nil {
			t.Fatal("Expected error for line number exceeding file length")
		}
		if !strings.Contains(err.Error(), "exceeds file length") {
			t.Errorf("Expected 'exceeds file length' error, got: %v", err)
		}
	})

	t.Run("symbol not found at line", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Try to find "NonExistentSymbol" at line 12 (where Person struct is)
		_, err := Rename(mainFile, 12, "NonExistentSymbol", "NewName")
		if err == nil {
			t.Fatal("Expected error for symbol not found")
		}
		if !strings.Contains(err.Error(), "not found at a word boundary") {
			t.Errorf("Expected 'not found at a word boundary' error, got: %v", err)
		}
	})

	t.Run("symbol at wrong line", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Try to find "Person" at line 2 (empty line) instead of line 12
		_, err := Rename(mainFile, 2, "Person", "Individual")
		if err == nil {
			t.Fatal("Expected error for symbol at wrong line")
		}
		if !strings.Contains(err.Error(), "not found at a word boundary") {
			t.Errorf("Expected 'not found at a word boundary' error, got: %v", err)
		}
	})

	t.Run("partial symbol match does not rename", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Try to rename "Person" as "Per" - should not match partial
		_, err := Rename(mainFile, 12, "Per", "Ind")
		if err == nil {
			t.Fatal("Expected error for partial symbol match")
		}
		if !strings.Contains(err.Error(), "not found at a word boundary") {
			t.Errorf("Expected 'not found at a word boundary' error, got: %v", err)
		}
	})

	t.Run("interface method rename", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Rename GetName method in interface on line 7
		result, err := Rename(mainFile, 7, "GetName", "GetFullName")
		if err != nil {
			t.Fatalf("Failed to rename interface method: %v", err)
		}

		// gopls rename returns empty output on success, but we convert to success message
		_ = result

		// Verify the rename was applied to the interface method only
		content := readFileContent(t, mainFile)

		// Check that the interface method was renamed
		if !strings.Contains(content, "GetFullName() string") {
			t.Error("New interface method 'GetFullName' not found")
		}
		if strings.Contains(content, "    GetName() string") {
			t.Error("Old interface method 'GetName' still exists")
		}

		// Note: gopls does NOT automatically rename implementation methods when renaming interface methods
		// The implementation still has the old name, which is correct behavior
		if !strings.Contains(content, "func (p *Person) GetName()") {
			t.Error(
				"Implementation method 'GetName' should still exist (not automatically renamed)",
			)
		}
	})

	t.Run("implementation method rename", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Rename GetName implementation method on line 18
		result, err := Rename(mainFile, 18, "GetName", "GetFullName")
		if err != nil {
			t.Fatalf("Failed to rename implementation method: %v", err)
		}

		// gopls rename returns empty output on success, but we convert to success message
		_ = result

		// Verify the rename was applied to the implementation method
		content := readFileContent(t, mainFile)
		if strings.Contains(content, "func (p *Person) GetName()") {
			t.Error("Old implementation method 'GetName' still exists")
		}
		if !strings.Contains(content, "func (p *Person) GetFullName()") {
			t.Error("New implementation method 'GetFullName' not found")
		}

		// The interface method should still have the old name when renaming only the implementation
		if !strings.Contains(content, "GetName() string") {
			t.Error(
				"Interface method 'GetName' should still exist (not automatically renamed)",
			)
		}
	})
}
