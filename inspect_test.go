package go_mcp_tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspect(t *testing.T) {
	t.Parallel()

	// Helper function to create a test workspace with multiple files
	createTestWorkspace := func(t testing.TB) string {
		tempDir := t.TempDir()

		// Create a go.mod file to make this a proper Go module
		goModContent := "module testmodule\n\ngo 1.21\n"
		goModFile := filepath.Join(tempDir, "go.mod")
		err := os.WriteFile(goModFile, []byte(goModContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Main test file with various declarations
		mainLines := []string{
			"package testpkg",                       // 1
			"",                                      // 2
			"import \"fmt\"",                        // 3
			"",                                      // 4
			"// TestInterface is a test interface",  // 5
			"type TestInterface interface {",        // 6
			"    Method1() string",                  // 7
			"    Method2(int) error",                // 8
			"}",                                     // 9
			"",                                      // 10
			"// MyStruct is a test struct",          // 11
			"type MyStruct struct {",                // 12
			"    Name string",                       // 13
			"    Age  int",                          // 14
			"}",                                     // 15
			"",                                      // 16
			"// Method1 implements TestInterface",   // 17
			"func (m *MyStruct) Method1() string {", // 18
			"    return m.Name",                     // 19
			"}",                                     // 20
			"",                                      // 21
			"// Method2 implements TestInterface",   // 22
			"func (m *MyStruct) Method2(val int) error {",   // 23
			"    if val < 0 {",                              // 24
			"        return fmt.Errorf(\"negative value\")", // 25
			"    }",                                 // 26
			"    return nil",                        // 27
			"}",                                     // 28
			"",                                      // 29
			"// NewMyStruct creates a new MyStruct", // 30
			"func NewMyStruct(name string, age int) *MyStruct {", // 31
			"    return &MyStruct{",                              // 32
			"        Name: name,",                                // 33
			"        Age:  age,",                                 // 34
			"    }",                                              // 35
			"}",                                                  // 36
			"",                                                   // 37
			"// Constants for testing",                           // 38
			"const (",                                            // 39
			"    DefaultName = \"default\"",                      // 40
			"    DefaultAge  = 25",                               // 41
			")",                                                  // 42
			"",                                                   // 43
			"// Variables for testing",                           // 44
			"var (",                                              // 45
			"    GlobalCounter int",                              // 46
			"    GlobalMessage string = \"hello\"",               // 47
			")",                                                  // 48
		}
		mainContent := strings.Join(mainLines, "\n")
		mainFile := filepath.Join(tempDir, "main.go")
		err = os.WriteFile(mainFile, []byte(mainContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Secondary file for package testing
		secondaryLines := []string{
			"package testpkg",                      // 1
			"",                                     // 2
			"import \"fmt\"",                       // 3
			"",                                     // 4
			"// Helper is a helper function",       // 5
			"func Helper() {",                      // 6
			"    fmt.Println(\"helper function\")", // 7
			"}",                                    // 8
			"",                                     // 9
			"// privateHelper is not exported",     // 10
			"func privateHelper() int {",           // 11
			"    return 42",                        // 12
			"}",                                    // 13
		}
		secondaryContent := strings.Join(secondaryLines, "\n")
		secondaryFile := filepath.Join(tempDir, "helper.go")
		err = os.WriteFile(secondaryFile, []byte(secondaryContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		return tempDir
	}

	t.Run("inspect entire file", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		result, err := Inspect(mainFile, 0, "", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect file: %v", err)
		}

		// Check that all major elements are present
		if !strings.Contains(result, "TestInterface") {
			t.Error("Expected TestInterface in file inspection")
		}
		if !strings.Contains(result, "MyStruct") {
			t.Error("Expected MyStruct in file inspection")
		}
		if !strings.Contains(result, "NewMyStruct") {
			t.Error("Expected NewMyStruct in file inspection")
		}
		if !strings.Contains(result, "DefaultName") {
			t.Error("Expected DefaultName in file inspection")
		}
		if !strings.Contains(result, "GlobalCounter") {
			t.Error("Expected GlobalCounter in file inspection")
		}
	})

	t.Run("inspect entire file excluding private", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		result, err := Inspect(mainFile, 0, "", false, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect file: %v", err)
		}

		// Should contain public symbols
		if !strings.Contains(result, "TestInterface") {
			t.Error("Expected TestInterface in public-only inspection")
		}
		if !strings.Contains(result, "MyStruct") {
			t.Error("Expected MyStruct in public-only inspection")
		}
		if !strings.Contains(result, "NewMyStruct") {
			t.Error("Expected NewMyStruct in public-only inspection")
		}
	})

	t.Run("inspect specific function by name", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		result, err := Inspect(mainFile, 0, "NewMyStruct", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect function by name: %v", err)
		}

		if !strings.Contains(result, "NewMyStruct") {
			t.Error("Expected NewMyStruct in function inspection")
		}
		if !strings.Contains(result, "func NewMyStruct(name string, age int) *MyStruct") {
			t.Error("Expected function signature in inspection")
		}
		if !strings.Contains(result, "References:") {
			t.Error("Expected References section in function inspection")
		}
	})

	t.Run("inspect specific type by name", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		result, err := Inspect(mainFile, 0, "MyStruct", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect type by name: %v", err)
		}

		if !strings.Contains(result, "MyStruct") {
			t.Error("Expected MyStruct in type inspection")
		}
		if !strings.Contains(result, "type MyStruct struct") {
			t.Error("Expected type declaration in inspection")
		}
		if !strings.Contains(result, "References:") {
			t.Error("Expected References section in type inspection")
		}
	})

	t.Run("inspect interface by name", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		result, err := Inspect(mainFile, 0, "TestInterface", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect interface by name: %v", err)
		}

		if !strings.Contains(result, "TestInterface") {
			t.Error("Expected TestInterface in interface inspection")
		}
		if !strings.Contains(result, "type TestInterface interface") {
			t.Error("Expected interface declaration in inspection")
		}
		if !strings.Contains(result, "Implementers:") {
			t.Error("Expected Implementers section for interface")
		}
		if !strings.Contains(result, "References:") {
			t.Error("Expected References section in interface inspection")
		}
	})

	t.Run("inspect variable by name", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		result, err := Inspect(mainFile, 0, "GlobalCounter", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect variable by name: %v", err)
		}

		if !strings.Contains(result, "GlobalCounter") {
			t.Error("Expected GlobalCounter in variable inspection")
		}
		if !strings.Contains(result, "Scope:") {
			t.Error("Expected Scope section in variable inspection")
		}
		if !strings.Contains(result, "References:") {
			t.Error("Expected References section in variable inspection")
		}
	})

	t.Run("inspect symbol by line number", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Line 12 is where MyStruct type is declared
		result, err := Inspect(mainFile, 12, "", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect symbol by line: %v", err)
		}

		if !strings.Contains(result, "MyStruct") {
			t.Error("Expected MyStruct when inspecting line 12")
		}
		if !strings.Contains(result, "type MyStruct struct") {
			t.Error("Expected type declaration when inspecting line 12")
		}
	})

	t.Run("inspect symbol by line and name combination", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Line 31 with function name should match NewMyStruct
		result, err := Inspect(mainFile, 31, "NewMyStruct", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect symbol by line and name: %v", err)
		}

		if !strings.Contains(result, "NewMyStruct") {
			t.Error("Expected NewMyStruct when inspecting line 31 with name")
		}
	})

	t.Run("inspect entire package", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)

		result, err := Inspect(".", 0, "", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect package: %v", err)
		}

		// Should contain symbols from both files
		if !strings.Contains(result, "MyStruct") {
			t.Error("Expected MyStruct in package inspection")
		}
		if !strings.Contains(result, "Helper") {
			t.Error("Expected Helper function in package inspection")
		}
		if !strings.Contains(result, "privateHelper") {
			t.Error(
				"Expected privateHelper function in package inspection (includePrivate=true)",
			)
		}
	})

	t.Run("inspect package excluding private", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)

		result, err := Inspect(".", 0, "", false, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect package: %v", err)
		}

		// Should contain public symbols
		if !strings.Contains(result, "MyStruct") {
			t.Error("Expected MyStruct in public package inspection")
		}
		if !strings.Contains(result, "Helper") {
			t.Error("Expected Helper function in public package inspection")
		}
		// Should not contain private symbols
		if strings.Contains(result, "privateHelper") {
			t.Error("Did not expect privateHelper function in public package inspection")
		}
	})

	t.Run("inspect symbol in package", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)

		result, err := Inspect(".", 0, "Helper", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect symbol in package: %v", err)
		}

		if !strings.Contains(result, "Helper") {
			t.Error("Expected Helper function in package symbol inspection")
		}
		if !strings.Contains(result, "func Helper()") {
			t.Error("Expected Helper function signature in package symbol inspection")
		}
	})

	t.Run("invalid parameters", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Empty workspace directory
		_, err := Inspect(mainFile, 0, "", true, "")
		if err == nil {
			t.Error("Expected error for empty workspace directory")
		}

		// Non-absolute workspace directory
		_, err = Inspect(mainFile, 0, "", true, "relative/path")
		if err == nil {
			t.Error("Expected error for non-absolute workspace directory")
		}

		// Non-existent file
		nonExistentFile := filepath.Join(workspace, "nonexistent.go")
		_, err = Inspect(nonExistentFile, 0, "", true, workspace)
		if err == nil {
			t.Error("Expected error for non-existent file")
		}

		// Invalid line number with no symbol name
		_, err = Inspect(mainFile, 999, "", true, workspace)
		if err == nil {
			t.Error("Expected error for invalid line number")
		}

		// Symbol not found
		_, err = Inspect(mainFile, 0, "NonExistentSymbol", true, workspace)
		if err == nil {
			t.Error("Expected error for non-existent symbol")
		}
	})

	t.Run("malformed Go file", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()

		// Create a malformed Go file
		malformedLines := []string{
			"package testpkg", // 1
			"",                // 2
			"func BadFunc( {", // 3 - syntax error
			"    return",      // 4
			"}",               // 5
		}
		malformedContent := strings.Join(malformedLines, "\n")
		malformedFile := filepath.Join(tempDir, "malformed.go")
		err := os.WriteFile(malformedFile, []byte(malformedContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		output, err := Inspect(malformedFile, 0, "", true, tempDir)
		if err != nil {
			t.Fatalf("Unexpected error for malformed Go file: %v", err)
		}
		if !strings.Contains(output, "WARNING: Syntax errors found") {
			t.Error("Expected warning about syntax errors in output")
		}
	})

	t.Run("method inspection", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		result, err := Inspect(mainFile, 0, "Method1", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect method: %v", err)
		}

		if !strings.Contains(result, "Method1") {
			t.Error("Expected Method1 in method inspection")
		}
		if !strings.Contains(result, "func (m *MyStruct) Method1() string") {
			t.Error("Expected method signature in inspection")
		}
		if !strings.Contains(result, "References:") {
			t.Error("Expected References section in method inspection")
		}
	})

	t.Run("constant inspection", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		result, err := Inspect(mainFile, 0, "DefaultName", true, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect constant: %v", err)
		}

		if !strings.Contains(result, "DefaultName") {
			t.Error("Expected DefaultName in constant inspection")
		}
		if !strings.Contains(result, "Scope:") {
			t.Error("Expected Scope section in constant inspection")
		}
		if !strings.Contains(result, "References:") {
			t.Error("Expected References section in constant inspection")
		}
	})

	t.Run("line number edge cases", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)
		mainFile := filepath.Join(workspace, "main.go")

		// Test line 1 (package declaration)
		_, err := Inspect(mainFile, 1, "", true, workspace)
		if err == nil {
			t.Error("Expected error when trying to inspect package line")
		}

		// Test empty line
		_, err = Inspect(mainFile, 2, "", true, workspace)
		if err == nil {
			t.Error("Expected error when trying to inspect empty line")
		}

		// Test comment line
		_, err = Inspect(mainFile, 5, "", true, workspace)
		if err == nil {
			t.Error("Expected error when trying to inspect comment line")
		}
	})

	t.Run("package with no Go files", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()

		// Create directory with no Go files
		emptyDir := filepath.Join(tempDir, "empty")
		err := os.Mkdir(emptyDir, 0755)
		if err != nil {
			t.Fatal(err)
		}

		_, err = Inspect("./empty", 0, "", true, tempDir)
		if err == nil {
			t.Error("Expected error for directory with no Go files")
		}
	})

	t.Run("import path inspection", func(t *testing.T) {
		t.Parallel()
		workspace := createTestWorkspace(t)

		// Test standard library package inspection
		result, err := Inspect("fmt", 0, "", false, workspace)
		if err != nil {
			t.Fatalf("Failed to inspect fmt package: %v", err)
		}

		if !strings.Contains(result, "func") {
			t.Error("Expected function declarations in fmt package inspection")
		}
	})
}
