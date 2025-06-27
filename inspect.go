package go_mcp_tools

import (
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/tools/go/packages"
)

const (
	inspectToolName        = "inspect"
	inspectToolDescription = `Go-aware code analyzer that understands interfaces, implementations, and call hierarchies. Unlike text search tools, this provides semantic understanding of Go code structure. Shows interface implementers, method receivers, and complete call chains with code context.

Supported path formats:
• Directory: /path/to/package
• File: /path/to/file.go
• File with line: /path/to/file.go:42
• File with line and symbol: /path/to/file.go:42:symbolName
• Import path: github.com/user/repo/package
• Import path with symbol: github.com/user/repo/package:symbolName`
)

func AddInspectTool(mcpServer *server.MCPServer) {
	handleInspect := func(
		ctx context.Context,
		request mcp.CallToolRequest,
	) (*mcp.CallToolResult, error) {
		arguments := request.GetArguments()

		pathStr, ok := arguments["path"].(string)
		if !ok || pathStr == "" {
			return nil, fmt.Errorf("path argument is required and must be a string")
		}

		// Parse the path to extract base path, line number, and symbol name
		var path, symbolName string
		var lineNumber int
		// Check if it's a file path (contains .go or starts with /)
		isFilePath := strings.Contains(pathStr, ".go") ||
			strings.HasPrefix(pathStr, "/") ||
			strings.HasPrefix(pathStr, "./") ||
			strings.HasPrefix(pathStr, "../")

		if isFilePath {
			// Parse file path patterns: /path/file.go[:line[:symbol]]

			// Split by colons to extract line and symbol
			parts := strings.Split(pathStr, ":")
			path = parts[0]

			if len(parts) > 1 {
				// Try to parse line number
				if ln, err := strconv.Atoi(parts[1]); err == nil {
					lineNumber = ln

					// If there's a third part, it's the symbol name
					if len(parts) > 2 {
						symbolName = parts[2]
					}
				} else {
					// Not a line number, might be a symbol name
					symbolName = parts[1]
				}
			}
		} else {
			// Parse import path patterns: github.com/user/repo/package[:symbol]

			// Find the last colon that's not part of a port number
			// Look for pattern like :symbol_name (not :digit)
			colonIndex := -1
			for i := len(pathStr) - 1; i >= 0; i-- {
				if pathStr[i] == ':' {
					// Check if what follows looks like a symbol name
					afterColon := pathStr[i+1:]
					if afterColon != "" && !regexp.MustCompile(`^\d+(/|$)`).MatchString(afterColon) {
						// Make sure it's a valid Go identifier pattern
						if regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(afterColon) {
							colonIndex = i
							break
						}
					}
				}
			}

			if colonIndex > 0 {
				path = pathStr[:colonIndex]
				symbolName = pathStr[colonIndex+1:]
			} else {
				path = pathStr
			}
		}

		// Get required and optional arguments
		onlyExported, _ := arguments["only_exported"].(bool)
		workspaceDir, ok := arguments["workspace_dir"].(string)
		if !ok || workspaceDir == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: "Error: workspace_dir is required.",
					},
				},
				IsError: true,
			}, nil
		}

		// Call the inspect function with parsed parameters
		summary, err := Inspect(
			path,
			lineNumber,
			symbolName,
			!onlyExported, // InspectSymbol uses includePrivate, so we invert onlyExported
			workspaceDir,
		)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error inspecting symbol: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: summary,
				},
			},
		}, nil
	}
	mcpServer.AddTool(mcp.NewTool(
		inspectToolName,
		mcp.WithDescription(inspectToolDescription),
		mcp.WithString(
			"path",
			mcp.Description(
				"Path to analyze. Supports multiple formats including line numbers and symbol names embedded in the path string.",
			),
			mcp.Required(),
		),
		mcp.WithString(
			"workspace_dir",
			mcp.Description(
				"Working directory for package resolution and reference finding. Should always be given.",
			),
			mcp.Required(),
		),
		mcp.WithBoolean(
			"only_exported",
			mcp.Description(
				"Whether to show only exported (public) symbols. Useful when summarizing external packages where only public symbols are relevant",
			),
			mcp.DefaultBool(false),
		),
	), handleInspect)
}

// Inspect analyzes a Go symbol (package, file, function, type, etc.)
// path can be a directory path, file path, or import statement path
// lineNumber and symbolName are optional for file paths to specify a particular symbol
// includePrivate determines whether private symbols are included
// workspaceDir is the working directory for package resolution and reference finding, and is required.
func Inspect(
	path string,
	lineNumber int,
	symbolName string,
	includePrivate bool,
	workspaceDir string,
) (string, error) {
	if workspaceDir == "" {
		return "", fmt.Errorf("workspace_dir is required for file analysis")
	}

	if !filepath.IsAbs(workspaceDir) {
		return "", fmt.Errorf(
			"workspace_dir must be an absolute path, got: %s",
			workspaceDir,
		)
	}

	var result strings.Builder

	// Helper to find and format symbol in declarations
	findSymbol := func(decls []ast.Decl, fset *token.FileSet, symbolName string, lineNumber int) (ast.Node, bool) {
		for _, decl := range decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if (symbolName != "" && d.Name.Name == symbolName) ||
					(lineNumber > 0 && containsLine(fset, d, lineNumber)) {
					return d, true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if (symbolName != "" && s.Name.Name == symbolName) ||
							(lineNumber > 0 && containsLine(fset, s, lineNumber)) {
							return s, true
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							if (symbolName != "" && name.Name == symbolName) ||
								(lineNumber > 0 && containsLine(fset, s, lineNumber)) {
								return s, true
							}
						}
					}
				}
			}
		}
		return nil, false
	}

	// Helper to format any symbol node
	formatSymbolWithContext := func(node ast.Node, fset *token.FileSet, file *ast.File) {
		switch n := node.(type) {
		case *ast.FuncDecl:
			formatFunction(&result, n, fset, true, true, workspaceDir)
		case *ast.TypeSpec:
			// Find the parent GenDecl for this TypeSpec
			var parentGenDecl *ast.GenDecl
			for _, decl := range file.Decls {
				if genDecl, ok := decl.(*ast.GenDecl); ok {
					for _, spec := range genDecl.Specs {
						if spec == n {
							parentGenDecl = genDecl
							break
						}
					}
					if parentGenDecl != nil {
						break
					}
				}
			}
			formatType(&result, n, fset, true, true, true, parentGenDecl, workspaceDir)
		case *ast.ValueSpec:
			// Find the parent GenDecl for this ValueSpec
			var parentGenDecl *ast.GenDecl
			for _, decl := range file.Decls {
				if genDecl, ok := decl.(*ast.GenDecl); ok {
					for _, spec := range genDecl.Specs {
						if spec == n {
							parentGenDecl = genDecl
							break
						}
					}
					if parentGenDecl != nil {
						break
					}
				}
			}
			formatVariable(&result, n, fset, true, true, parentGenDecl, workspaceDir)
		}
	}

	// Handle file paths
	if strings.HasSuffix(path, ".go") {
		resolvedPath, err := resolveFilePath(path, workspaceDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve file path: %w", err)
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, resolvedPath, nil, parser.ParseComments)

		// Handle syntax errors - we can still work with partial AST
		var syntaxErrorMsg string
		if err != nil {
			if errList, ok := err.(scanner.ErrorList); ok {
				// Syntax errors - we have a partial AST, continue with warning
				syntaxErrorMsg = fmt.Sprintf(
					"WARNING: Syntax errors found, analysis may be incomplete:\n%s\n\n",
					errList.Error(),
				)
			} else {
				// Other parsing errors - cannot proceed
				return "", fmt.Errorf("failed to parse file %s: %w", resolvedPath, err)
			}
		}

		// If we have no AST at all, we can't proceed
		if file == nil {
			return "", fmt.Errorf(
				"failed to parse file %s: no AST generated",
				resolvedPath,
			)
		}

		// Case 1: Format entire file
		if lineNumber == 0 && symbolName == "" {
			formatFile(&result, file, fset, includePrivate, true, workspaceDir)
			return syntaxErrorMsg + result.String(), nil
		}

		// Case 2 & 3: Find specific symbol
		if symbol, found := findSymbol(file.Decls, fset, symbolName, lineNumber); found {
			formatSymbolWithContext(symbol, fset, file)
			return syntaxErrorMsg + result.String(), nil
		}

		if symbolName != "" {
			return "", fmt.Errorf("symbol '%s' not found in file", symbolName)
		}
		return "", fmt.Errorf("no symbol found at line %d", lineNumber)
	}

	// Handle package paths
	resolvedPkgPath, err := resolvePackagePath(path, workspaceDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve package path: %w", err)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedModule,
		Dir: workspaceDir,
	}

	pkgs, err := packages.Load(cfg, resolvedPkgPath)
	if err != nil {
		return "", fmt.Errorf("failed to load package %s: %w", resolvedPkgPath, err)
	}

	if len(pkgs) == 0 {
		return "", fmt.Errorf("no packages found for path: %s", resolvedPkgPath)
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return "", fmt.Errorf("package has errors: %v", pkg.Errors)
	}

	// Case 1: Format entire package
	if symbolName == "" {
		formatPackage(&result, pkg, includePrivate, workspaceDir)
		return result.String(), nil
	}

	// Case 2: Find specific symbol in package
	for _, file := range pkg.Syntax {
		if symbol, found := findSymbol(file.Decls, pkg.Fset, symbolName, 0); found {
			formatSymbolWithContext(symbol, pkg.Fset, file)
			return result.String(), nil
		}
	}

	return "", fmt.Errorf("symbol '%s' not found in package", symbolName)
}

func formatFunction(
	b *strings.Builder,
	fn *ast.FuncDecl,
	fset *token.FileSet,
	includeReferences bool,
	includeCallHierarchy bool,
	workspaceDir string,
) {
	// Get signature start position
	sigStart := fset.Position(fn.Pos())

	// Get body end position (always show signature to body range)
	var bodyEnd token.Position
	if fn.Body != nil {
		bodyEnd = fset.Position(fn.Body.End())
	} else {
		// For function declarations without body (like in interfaces)
		bodyEnd = fset.Position(fn.End())
	}

	// Lines section
	if bodyEnd.Line > sigStart.Line {
		fmt.Fprintf(b, "Lines: %d-%d\n", sigStart.Line, bodyEnd.Line)
	} else {
		fmt.Fprintf(b, "Lines: %d\n", sigStart.Line)
	}

	// Docstring section
	if fn.Doc != nil {
		b.WriteString("Docstring: ")
		b.WriteString(strings.TrimSpace(fn.Doc.Text()))
		b.WriteString("\n")
	}

	// Code section
	b.WriteString("Code:\n")

	var endLine int
	// Just signature - end before opening brace or at function end
	if fn.Body != nil {
		endLine = fset.Position(fn.Body.Pos() - 1).Line
	} else {
		endLine = fset.Position(fn.End()).Line
	}

	// Read the raw source code from the file
	rawSource, err := readSourceLines(sigStart.Filename, sigStart.Line, endLine)
	if err == nil {
		// Remove opening bracket if present at the end
		trimmed := strings.TrimSpace(rawSource)
		if strings.HasSuffix(trimmed, "{") {
			trimmed = strings.TrimSpace(trimmed[:len(trimmed)-1])
		}
		b.WriteString(trimmed)
	} else {
		fmt.Fprintf(b, "// Error reading source: %v", err)
	}

	// Only include references/call hierarchy if the file is within the workspace
	isInWorkspace := isFileInWorkspace(sigStart.Filename, workspaceDir)

	// Include references if requested and file is in workspace
	if includeReferences && isInWorkspace {
		b.WriteString("\n\n")
		formatReferences(b, sigStart.Filename, sigStart.Line, fn.Name.Name)
	}

	// Include call hierarchy if requested and file is in workspace
	if includeCallHierarchy && isInWorkspace {
		b.WriteString("\n\n")
		formatCallHierarchy(b, sigStart.Filename, sigStart.Line, fn.Name.Name)
	}
}

func formatType(
	b *strings.Builder,
	typeSpec *ast.TypeSpec,
	fset *token.FileSet,
	includeReferences bool,
	includeImplementers bool,
	includeMethods bool,
	parentGenDecl *ast.GenDecl,
	workspaceDir string,
) {
	// Get type start and end positions
	start := fset.Position(typeSpec.Pos())
	end := fset.Position(typeSpec.End())

	// Lines section
	if end.Line > start.Line {
		fmt.Fprintf(b, "Lines: %d-%d\n", start.Line, end.Line)
	} else {
		fmt.Fprintf(b, "Lines: %d\n", start.Line)
	}

	// Docstring section - check TypeSpec first, then parentGenDecl if provided
	var docText string
	if typeSpec.Doc != nil {
		docText = typeSpec.Doc.Text()
	} else if parentGenDecl != nil && parentGenDecl.Doc != nil {
		docText = parentGenDecl.Doc.Text()
	}

	if docText != "" {
		b.WriteString("Docstring: ")
		b.WriteString(strings.TrimSpace(docText))
		b.WriteString("\n")
	}

	// Code section
	b.WriteString("Code:\n")

	// Read the raw source code from the file
	rawSource, err := readSourceLines(start.Filename, start.Line, end.Line)
	if err == nil {
		b.WriteString(rawSource)
	} else {
		fmt.Fprintf(b, "// Error reading source: %v", err)
	}

	// Include implementers if requested and type is an interface and file is in workspace
	if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok &&
		interfaceType.Methods != nil {
		isInWorkspace := isFileInWorkspace(start.Filename, workspaceDir)
		if includeImplementers && isInWorkspace {
			b.WriteString("\n\n")
			formatImplementers(b, start.Filename, start.Line, typeSpec.Name.Name)
		}
	}

	// Include methods if requested
	if includeMethods {
		// Find all methods for this type by parsing the file and looking for method receivers
		cachedFile, err := globalFileCache.GetOrParseFile(start.Filename)
		if err == nil {
			file := cachedFile.ast
			fileFset := cachedFile.fset

			// Find methods with this type as receiver
			var methods []*ast.FuncDecl
			for _, decl := range file.Decls {
				if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
					// Check if this method has our type as receiver
					if len(funcDecl.Recv.List) > 0 {
						receiverType := extractReceiverTypeName(
							funcDecl.Recv.List[0].Type,
						)
						if receiverType == typeSpec.Name.Name {
							methods = append(methods, funcDecl)
						}
					}
				}
			}

			// Format each method
			for _, method := range methods {
				b.WriteString("\n\n")
				formatFunction(b, method, fileFset, false, false, workspaceDir)
			}
		}
	}

	// Include references if requested and file is in workspace
	isInWorkspace := isFileInWorkspace(start.Filename, workspaceDir)
	if includeReferences && isInWorkspace {
		b.WriteString("\n\n")
		formatReferences(b, start.Filename, start.Line, typeSpec.Name.Name)
	}
}

func formatVariable(
	b *strings.Builder,
	valueSpec *ast.ValueSpec,
	fset *token.FileSet,
	includeReferences bool,
	includeScope bool,
	parentGenDecl *ast.GenDecl,
	workspaceDir string,
) {
	// Get variable start and end positions
	start := fset.Position(valueSpec.Pos())
	end := fset.Position(valueSpec.End())

	// Lines section
	if end.Line > start.Line {
		fmt.Fprintf(b, "Lines: %d-%d\n", start.Line, end.Line)
	} else {
		fmt.Fprintf(b, "Lines: %d\n", start.Line)
	}

	// Docstring section - check ValueSpec first, then parentGenDecl if provided
	var docText string
	if valueSpec.Doc != nil {
		docText = valueSpec.Doc.Text()
	} else if parentGenDecl != nil && parentGenDecl.Doc != nil {
		docText = parentGenDecl.Doc.Text()
	}

	if docText != "" {
		b.WriteString("Docstring: ")
		b.WriteString(strings.TrimSpace(docText))
		b.WriteString("\n")
	}

	// Code section
	b.WriteString("Code:\n")

	// Read the raw source code from the file
	rawSource, err := readSourceLines(start.Filename, start.Line, end.Line)
	if err == nil {
		b.WriteString(rawSource)
	} else {
		fmt.Fprintf(b, "// Error reading source: %v", err)
	}

	if includeScope {
		b.WriteString("\n")
		formatScope(b, start.Filename, start.Line)
	}

	// Include references if requested and file is in workspace
	isInWorkspace := isFileInWorkspace(start.Filename, workspaceDir)
	if includeReferences && isInWorkspace {
		// Handle multiple variable names in a single declaration
		for _, name := range valueSpec.Names {
			b.WriteString("\n\n")
			formatReferences(b, start.Filename, start.Line, name.Name)
		}
	}
}

func formatFile(
	b *strings.Builder,
	file *ast.File,
	fset *token.FileSet,
	includePrivate bool,
	includeImports bool,
	workspaceDir string,
) {
	lineWritten := false
	addSeparator := func() {
		if lineWritten {
			b.WriteString("\n\n")
		}
		lineWritten = true
	}

	// Include absolute file path at the top
	if file.Pos().IsValid() {
		addSeparator()
		pos := fset.Position(file.Pos())
		fmt.Fprintf(b, "File: %s", pos.Filename)
	}

	// Include file docstring if present
	if file.Doc != nil {
		addSeparator()
		b.WriteString("File Docstring:\n")
		b.WriteString(strings.TrimSpace(file.Doc.Text()))
	}

	// First pass: handle imports if requested
	if includeImports {
		var imports []string
		for _, decl := range file.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range genDecl.Specs {
					if importSpec, ok := spec.(*ast.ImportSpec); ok {
						// Get import start and end positions
						start := fset.Position(importSpec.Pos())
						end := fset.Position(importSpec.End())

						// Read the raw source code from the file
						rawSource, err := readSourceLines(
							start.Filename,
							start.Line,
							end.Line,
						)
						if err == nil {
							imports = append(imports, strings.TrimSpace(rawSource))
						} else {
							imports = append(imports, fmt.Sprintf("// Error reading source: %v", err))
						}
					}
				}
			}
		}

		// Write all imports under a single header if any were found
		if len(imports) > 0 {
			addSeparator()
			b.WriteString("Imports:\n")
			for _, imp := range imports {
				b.WriteString(imp)
				b.WriteString("\n")
			}
		}
	}

	// Second pass: handle all other declarations
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Only include exported functions/methods or if includePrivate is true
			if includePrivate || ast.IsExported(d.Name.Name) {
				addSeparator()
				formatFunction(b, d, fset, false, false, workspaceDir)
			}

		case *ast.GenDecl:
			// Handle type, var, const declarations (skip imports, already handled)
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.ImportSpec:
					// Skip imports - already handled in first pass
					continue

				case *ast.TypeSpec:
					// Only include exported types or if includePrivate is true
					if includePrivate || ast.IsExported(s.Name.Name) {
						addSeparator()
						formatType(b, s, fset, false, false, false, d, workspaceDir)
					}

				case *ast.ValueSpec:
					// Handle variables and constants
					// Check if any of the names are exported or if includePrivate is true
					shouldInclude := includePrivate
					if !shouldInclude {
						for _, name := range s.Names {
							if ast.IsExported(name.Name) {
								shouldInclude = true
								break
							}
						}
					}

					if shouldInclude {
						addSeparator()
						formatVariable(b, s, fset, false, false, d, workspaceDir)
					}
				}
			}
		}
	}
}

func formatPackage(
	b *strings.Builder,
	pkg *packages.Package,
	includePrivate bool,
	workspaceDir string,
) {
	// Add absolute directory path
	if pkg.Module != nil && pkg.Module.Dir != "" {
		fmt.Fprintf(b, "Directory: %s", pkg.Module.Dir)
		if pkg.PkgPath != pkg.Module.Path {
			// For subpackages, append the relative path
			relPath := strings.TrimPrefix(pkg.PkgPath, pkg.Module.Path)
			if relPath != "" && relPath != "/" {
				relPath = strings.TrimPrefix(relPath, "/")
				fmt.Fprintf(b, "/%s", relPath)
			}
		}
		b.WriteString("\n")
	} else if len(pkg.GoFiles) > 0 {
		// Fallback: use directory of first Go file
		dir := filepath.Dir(pkg.GoFiles[0])
		fmt.Fprintf(b, "Directory: %s\n", dir)
	}

	// Add Go import path
	if pkg.PkgPath != "" {
		fmt.Fprintf(b, "Import Path: %s\n", pkg.PkgPath)
	}

	// Triple line break before file contents
	b.WriteString("\n\n")

	fileWritten := false

	for _, file := range pkg.Syntax {
		if fileWritten {
			b.WriteString("\n---\n")
		}
		fileWritten = true

		formatFile(
			b,
			file,
			pkg.Fset,
			includePrivate,
			false,
			workspaceDir,
		)
	}
}

// formatReferences finds and formats references to a symbol using gopls
func formatReferences(
	b *strings.Builder,
	filePath string,
	lineNumber int,
	symbolName string,
) {
	b.WriteString("References:\n")

	if filePath == "" || lineNumber <= 0 || symbolName == "" {
		b.WriteString("Invalid parameters for finding references\n")
		return
	}

	// Create gopls position using utility function
	position, err := createGoplsPosition(filePath, lineNumber, symbolName)
	if err != nil {
		fmt.Fprintf(b, "Failed to find references: %s\n", err.Error())
		return
	}

	// Execute gopls references command using utility function
	outputStr, err := executeGoplsCommand("references", position)
	if err != nil {
		fmt.Fprintf(b, "gopls references failed: %s\n", err.Error())
	}

	if outputStr == "" {
		b.WriteString("No references found\n")
		return
	}

	// Parse gopls output and group references
	functionScopes := make(map[string]bool) // Track functions we've already formatted
	packageFiles := make(map[string]bool)   // Track package-level files

	for line := range strings.SplitSeq(outputStr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse location: /path/to/file.go:line:startCol-endCol
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}

		// File path is everything except the last two parts
		fp := strings.Join(parts[:len(parts)-2], ":")

		// Parse line number
		ln, err := strconv.Atoi(parts[len(parts)-2])
		if err != nil {
			continue
		}

		// Determine the scope for this reference
		scope, err := determineScope(fp, ln)
		if err != nil {
			// Fallback to file-level grouping (package scope)
			packageFiles[fp] = true
		} else if scope != fp {
			// This is a function scope
			functionScopes[scope] = true
		} else {
			// This is package scope
			packageFiles[fp] = true
		}
	}

	if len(functionScopes) == 0 && len(packageFiles) == 0 {
		b.WriteString("No references found\n")
		return
	}

	lineWritten := false
	addSeparator := func() {
		if lineWritten {
			b.WriteString("\n\n")
		}
		lineWritten = true
	}

	// Format function-level references
	for scope := range functionScopes {
		// Parse scope format: /path/to/file.go:line:functionName
		parts := strings.Split(scope, ":")
		if len(parts) < 3 {
			continue
		}

		fp := strings.Join(parts[:len(parts)-2], ":")
		ln, err := strconv.Atoi(parts[len(parts)-2])
		if err != nil {
			continue
		}

		// Parse the file to find the function using AST cache
		cachedFile, err := globalFileCache.GetOrParseFile(fp)
		if err != nil {
			fmt.Fprintf(b, "  Error parsing file %s: %v\n", fp, err)
			continue
		}

		file := cachedFile.ast
		fset := cachedFile.fset

		// Find the function at the specified line
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			funcStart := fset.Position(funcDecl.Pos()).Line
			funcEnd := fset.Position(funcDecl.End()).Line

			if ln < funcStart || ln > funcEnd {
				continue // Not in this function
			}
			// Format the function using a temporary builder
			var tempBuilder strings.Builder
			formatFunction(&tempBuilder, funcDecl, fset, false, false, "")

			// Indent each line of the function output
			functionOutput := tempBuilder.String()
			for line := range strings.SplitSeq(functionOutput, "\n") {
				if line == "" {
					continue
				}
				addSeparator()
				fmt.Fprintf(b, "  %s\n", line)
			}
			break
		}
	}

	// Format package-level references
	for fp := range packageFiles {
		addSeparator()
		fmt.Fprintf(b, "  Package scope: %s\n", fp)
	}
}

// formatImplementers finds and formats implementers of an interface using gopls
func formatImplementers(
	b *strings.Builder,
	filePath string,
	lineNumber int,
	symbolName string,
) {
	b.WriteString("Implementers:\n")

	if filePath == "" || lineNumber <= 0 || symbolName == "" {
		b.WriteString("Invalid parameters for finding implementers\n")
		return
	}

	// Create gopls position using utility function
	position, err := createGoplsPosition(filePath, lineNumber, symbolName)
	if err != nil {
		fmt.Fprintf(b, "Failed to find implementers: %s\n", err.Error())
		return
	}

	// Execute gopls implementation command using utility function
	outputStr, err := executeGoplsCommand("implementation", position)
	if err != nil {
		fmt.Fprintf(b, "gopls implementation failed: %s\n", err.Error())
		return
	}

	if outputStr == "" {
		b.WriteString("No implementers found\n")
		return
	}

	// Parse gopls output to get implementer locations
	lines := strings.Split(outputStr, "\n")
	implementers := make(map[string][]int)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse location: /path/to/file.go:line:startCol-endCol
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}

		// File path is everything except the last two parts
		fp := strings.Join(parts[:len(parts)-2], ":")

		// Parse line number
		ln, err := strconv.Atoi(parts[len(parts)-2])
		if err != nil {
			continue
		}

		implementers[fp] = append(implementers[fp], ln)
	}

	if len(implementers) == 0 {
		b.WriteString("No implementers found\n")
		return
	}

	lineWritten := false
	addSeparator := func() {
		if lineWritten {
			b.WriteString("\n\n")
		}
		lineWritten = true
	}

	// Format each implementer using formatType
	for fp, lns := range implementers {
		for _, ln := range lns {
			// Parse the file to find the type at the implementer location using AST cache
			cachedFile, err := globalFileCache.GetOrParseFile(fp)
			if err != nil {
				addSeparator()
				fmt.Fprintf(b, "  Error parsing file %s: %v\n", fp, err)
				continue
			}

			file := cachedFile.ast
			fset := cachedFile.fset

			// Find the type declaration at the specified line
			typeSpec := findTypeAtLine(file, fset, ln)
			if typeSpec == nil {
				addSeparator()
				fmt.Fprintf(b, "  No type found at %s:%d\n", fp, ln)
				continue
			}
			// Format the type using a temporary builder
			var tempBuilder strings.Builder
			formatType(&tempBuilder, typeSpec, fset, false, false, false, nil, "")

			// Indent each line of the type output
			typeOutput := tempBuilder.String()
			for line := range strings.SplitSeq(typeOutput, "\n") {
				if line == "" {
					continue
				}
				addSeparator()
				fmt.Fprintf(b, "  %s\n", line)
			}
		}
	}
}

// formatScope formats scope hierarchy information for a given file position
func formatScope(
	b *strings.Builder,
	filePath string,
	lineNumber int,
) {
	b.WriteString("Scope:\n")

	if filePath == "" || lineNumber <= 0 {
		b.WriteString("Invalid parameters for determining scope\n")
		return
	}

	// Parse the file to find scope information using AST cache
	cachedFile, err := globalFileCache.GetOrParseFile(filePath)
	if err != nil {
		fmt.Fprintf(b, "Error parsing file: %v\n", err)
		return
	}

	file := cachedFile.ast
	fset := cachedFile.fset

	// Build scope hierarchy from package to current position
	hierarchy := buildScopeHierarchyAtLine(file, fset, lineNumber)

	for i, scope := range hierarchy {
		indent := strings.Repeat("  ", i)
		fmt.Fprintf(b, "%s%s\n", indent, scope)
	}
}

// determineScope determines the scope (file or function) for a reference
func determineScope(filePath string, lineNumber int) (string, error) {
	// Parse the file to find the containing function using AST cache
	cachedFile, err := globalFileCache.GetOrParseFile(filePath)
	if err != nil {
		return filePath, err // fallback to file scope
	}

	file := cachedFile.ast
	fset := cachedFile.fset

	// Find if the line is within a function
	for _, decl := range file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			funcStart := fset.Position(funcDecl.Pos()).Line
			funcEnd := fset.Position(funcDecl.End()).Line

			if lineNumber >= funcStart && lineNumber <= funcEnd {
				// Reference is within this function
				return fmt.Sprintf(
					"%s:%d:%s",
					filePath,
					funcStart,
					funcDecl.Name.Name,
				), nil
			}
		}
	}

	// Reference is at package level
	return filePath, nil
}

// findTypeAtLine finds a type declaration at or near the specified line
func findTypeAtLine(file *ast.File, fset *token.FileSet, targetLine int) *ast.TypeSpec {
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					start := fset.Position(typeSpec.Pos()).Line
					end := fset.Position(typeSpec.End()).Line

					// Check if the target line is within this type declaration
					if targetLine >= start && targetLine <= end {
						return typeSpec
					}
				}
			}
		}
	}
	return nil
}

// buildScopeHierarchyAtLine builds scope hierarchy for a specific line in a file
func buildScopeHierarchyAtLine(
	file *ast.File,
	fset *token.FileSet,
	targetLine int,
) []string {
	hierarchy := []string{}

	// Start with package scope
	packageScope := fmt.Sprintf("package %s", file.Name.Name)
	hierarchy = append(hierarchy, packageScope)

	// Check if inside a type declaration
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					typeStart := fset.Position(typeSpec.Pos()).Line
					typeEnd := fset.Position(typeSpec.End()).Line

					if targetLine >= typeStart && targetLine <= typeEnd {
						typeScope := fmt.Sprintf("type %s (lines %d-%d)",
							typeSpec.Name.Name, typeStart, typeEnd)
						hierarchy = append(hierarchy, typeScope)
					}
				}
			}
		}
	}

	// Check if inside a function or method
	for _, decl := range file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			funcStart := fset.Position(funcDecl.Pos()).Line
			funcEnd := fset.Position(funcDecl.End()).Line

			if targetLine >= funcStart && targetLine <= funcEnd {
				var funcScope string
				if funcDecl.Recv != nil {
					// Method
					recvType := extractReceiverTypeSimple(funcDecl.Recv.List[0].Type)
					funcScope = fmt.Sprintf("method %s.%s (lines %d-%d)",
						recvType, funcDecl.Name.Name, funcStart, funcEnd)
				} else {
					// Function
					funcScope = fmt.Sprintf("function %s (lines %d-%d)",
						funcDecl.Name.Name, funcStart, funcEnd)
				}
				hierarchy = append(hierarchy, funcScope)

				// Check for block scopes within the function
				if funcDecl.Body != nil {
					blockHierarchy := findBlockScopesAtLine(
						funcDecl.Body,
						fset,
						targetLine,
					)
					hierarchy = append(hierarchy, blockHierarchy...)
				}
			}
		}
	}

	return hierarchy
}

// extractReceiverTypeSimple extracts receiver type name in a simple format
func extractReceiverTypeSimple(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	}
	return "unknown"
}

// findBlockScopesAtLine finds block scopes (if, for, switch, etc.) at a specific line
func findBlockScopesAtLine(stmt ast.Stmt, fset *token.FileSet, targetLine int) []string {
	var scopes []string

	ast.Inspect(stmt, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		start := fset.Position(n.Pos()).Line
		end := fset.Position(n.End()).Line

		// Only consider nodes that contain our target line
		if targetLine < start || targetLine > end {
			return false
		}

		switch node := n.(type) {
		case *ast.IfStmt:
			if start <= targetLine && targetLine <= end {
				scopes = append(scopes, fmt.Sprintf("if (lines %d-%d)", start, end))
			}
		case *ast.ForStmt:
			if start <= targetLine && targetLine <= end {
				scopes = append(scopes, fmt.Sprintf("for (lines %d-%d)", start, end))
			}
		case *ast.RangeStmt:
			if start <= targetLine && targetLine <= end {
				scopes = append(scopes, fmt.Sprintf("range (lines %d-%d)", start, end))
			}
		case *ast.SwitchStmt:
			if start <= targetLine && targetLine <= end {
				scopes = append(scopes, fmt.Sprintf("switch (lines %d-%d)", start, end))
			}
		case *ast.TypeSwitchStmt:
			if start <= targetLine && targetLine <= end {
				scopes = append(scopes, fmt.Sprintf("type-switch (lines %d-%d)", start, end))
			}
		case *ast.SelectStmt:
			if start <= targetLine && targetLine <= end {
				scopes = append(scopes, fmt.Sprintf("select (lines %d-%d)", start, end))
			}
		case *ast.BlockStmt:
			// Only add generic block if it's not part of another construct
			if start <= targetLine && targetLine <= end && !isPartOfConstruct(node, n) {
				scopes = append(scopes, fmt.Sprintf("block (lines %d-%d)", start, end))
			}
		}
		return true
	})

	return scopes
}

// isPartOfConstruct checks if a block statement is part of a larger construct
func isPartOfConstruct(block *ast.BlockStmt, parent ast.Node) bool {
	// To properly detect if a block is part of a construct, we need to check
	// if it appears as the body of control structures. Since ast.Inspect doesn't
	// give us the direct parent-child relationship in the way we need, we'll
	// implement a more sophisticated check.

	// This function will return true if the block is part of a construct
	// (and should be skipped in scope reporting), false if it's standalone

	// We'll use a visitor pattern to find if this specific block instance
	// is referenced as a body in any control structure
	isPartOfConstruct := false

	// Create a visitor that checks if our block is used as a body
	ast.Inspect(parent, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		switch node := n.(type) {
		case *ast.IfStmt:
			if node.Body == block || node.Else == block {
				isPartOfConstruct = true
				return false
			}
		case *ast.ForStmt:
			if node.Body == block {
				isPartOfConstruct = true
				return false
			}
		case *ast.RangeStmt:
			if node.Body == block {
				isPartOfConstruct = true
				return false
			}
		case *ast.SwitchStmt:
			if node.Body == block {
				isPartOfConstruct = true
				return false
			}
		case *ast.TypeSwitchStmt:
			if node.Body == block {
				isPartOfConstruct = true
				return false
			}
		case *ast.SelectStmt:
			if node.Body == block {
				isPartOfConstruct = true
				return false
			}
		case *ast.FuncDecl:
			if node.Body == block {
				isPartOfConstruct = true
				return false
			}
		case *ast.FuncLit:
			if node.Body == block {
				isPartOfConstruct = true
				return false
			}
		case *ast.CaseClause:
			// Check if block is in the body of a case
			for _, stmt := range node.Body {
				if stmt == block {
					isPartOfConstruct = true
					return false
				}
			}
		case *ast.CommClause:
			// Check if block is in the body of a communication clause
			for _, stmt := range node.Body {
				if stmt == block {
					isPartOfConstruct = true
					return false
				}
			}
		}
		return true
	})

	return isPartOfConstruct
}

// readSourceLines reads the specified lines from a source file and returns the raw content
func readSourceLines(filename string, startLine, endLine int) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	var lines []string
	scanner := bufio.NewScanner(file)
	lineNum := 1

	for scanner.Scan() {
		if lineNum >= startLine && lineNum <= endLine {
			lines = append(lines, scanner.Text())
		}
		if lineNum > endLine {
			break
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}

// resolveFilePath resolves a file path relative to the workspace directory
// This handles both absolute and relative file paths consistently
func resolveFilePath(filePath string, workspaceDir string) (string, error) {
	// Convert workspace directory to absolute path
	absWorkspaceDir, err := filepath.Abs(workspaceDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve workspace directory: %w", err)
	}

	// If file path is already absolute, check if it exists
	if filepath.IsAbs(filePath) {
		if _, err := os.Stat(filePath); err == nil {
			return filePath, nil
		}
		return "", fmt.Errorf("absolute file path does not exist: %s", filePath)
	}

	// Handle relative paths
	var candidatePaths []string

	// Try relative to workspace directory
	if strings.HasPrefix(filePath, "./") || strings.HasPrefix(filePath, "../") {
		// Explicit relative path
		candidatePaths = append(candidatePaths, filepath.Join(absWorkspaceDir, filePath))
	} else {
		// Bare filename or path - try multiple locations
		candidatePaths = append(candidatePaths,
			filepath.Join(absWorkspaceDir, filePath),
			filepath.Join(absWorkspaceDir, "./"+filePath),
		)
	}

	// Find the first path that exists
	for _, path := range candidatePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf(
		"file not found: %s (searched relative to %s)",
		filePath,
		absWorkspaceDir,
	)
}

// make relative paths starting with "./" or with "../" absolute
func resolvePackagePath(pkgPath string, workspaceDir string) (string, error) {
	slashPath := filepath.ToSlash(pkgPath)
	if strings.HasPrefix(slashPath, "./") || strings.HasPrefix(slashPath, "../") {
		// Relative path, resolve it against the workspace directory
		absPath, err := resolveFilePath(slashPath, workspaceDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve relative package path: %w", err)
		}
		return absPath, nil
	}
	return pkgPath, nil
}

// isFileInWorkspace checks if a file path is within the workspace directory
func isFileInWorkspace(filePath, workspaceDir string) bool {
	if workspaceDir == "" {
		return false
	}

	// Convert both paths to absolute paths for reliable comparison
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}

	absWorkspaceDir, err := filepath.Abs(workspaceDir)
	if err != nil {
		return false
	}

	// Check if the file path starts with the workspace directory
	rel, err := filepath.Rel(absWorkspaceDir, absFilePath)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", the file is outside the workspace
	return !strings.HasPrefix(rel, "..")
}

// containsLine checks if a node contains the specified line number
func containsLine(fset *token.FileSet, node ast.Node, line int) bool {
	start := fset.Position(node.Pos()).Line
	end := fset.Position(node.End()).Line
	return line >= start && line <= end
}

// extractReceiverTypeName extracts the type name from a receiver expression
func extractReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// fileCache provides a thread-safe cache for parsed Go files
type fileCache struct {
	mu    sync.RWMutex
	files map[string]*cachedFile
}

// cachedFile represents a cached parsed Go file
type cachedFile struct {
	ast      *ast.File
	fset     *token.FileSet
	modTime  time.Time
	filePath string
}

// Global file cache instance
var globalFileCache = &fileCache{
	files: make(map[string]*cachedFile),
}

// GetOrParseFile retrieves a cached file or parses it if not cached/outdated
func (cache *fileCache) GetOrParseFile(filePath string) (*cachedFile, error) {
	cache.mu.RLock()
	cached, exists := cache.files[filePath]
	cache.mu.RUnlock()

	// Check if we have a valid cached version
	if exists {
		stat, err := os.Stat(filePath)
		if err == nil && !stat.ModTime().After(cached.modTime) {
			return cached, nil
		}
	}

	// Need to parse the file
	fset := token.NewFileSet()
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		// Check if it's a syntax error (scanner.ErrorList) - we can still work with partial AST
		if _, ok := err.(scanner.ErrorList); !ok {
			// Not a syntax error - cannot proceed
			return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
		}
		// For syntax errors, we continue with the partial AST (file might still be valid)
	}

	// If we have no AST at all, we can't proceed
	if file == nil {
		return nil, fmt.Errorf("failed to parse file %s: no AST generated", filePath)
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	cached = &cachedFile{
		ast:      file,
		fset:     fset,
		modTime:  stat.ModTime(),
		filePath: filePath,
	}

	// Cache the parsed file
	cache.mu.Lock()
	cache.files[filePath] = cached
	cache.mu.Unlock()

	return cached, nil
}

// ClearCache removes all cached files (useful for testing or memory management)
func (cache *fileCache) ClearCache() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.files = make(map[string]*cachedFile)
}

// RemoveFile removes a specific file from the cache
func (cache *fileCache) RemoveFile(filePath string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.files, filePath)
}

// GetCacheStats returns information about the cache state
func (cache *fileCache) GetCacheStats() map[string]any {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return map[string]any{
		"cached_files": len(cache.files),
		"files": func() []string {
			files := make([]string, 0, len(cache.files))
			for path := range cache.files {
				files = append(files, path)
			}
			return files
		}(),
	}
}

// formatCallHierarchy finds and formats call hierarchy for a symbol using gopls
func formatCallHierarchy(
	b *strings.Builder,
	filePath string,
	lineNumber int,
	symbolName string,
) {
	b.WriteString("Call Hierarchy:\n")

	if filePath == "" || lineNumber <= 0 || symbolName == "" {
		b.WriteString("Invalid parameters for finding call hierarchy\n")
		return
	}

	// Create gopls position using utility function
	position, err := createGoplsPosition(filePath, lineNumber, symbolName)
	if err != nil {
		fmt.Fprintf(b, "Failed to find call hierarchy: %s\n", err.Error())
		return
	}

	// Execute gopls call_hierarchy command using utility function
	outputStr, err := executeGoplsCommand("call_hierarchy", position)
	if err != nil {
		fmt.Fprintf(b, "gopls call_hierarchy failed: %s\n", err.Error())
		return
	}

	if outputStr == "" {
		b.WriteString("No call hierarchy found\n")
		return
	}

	// Output the raw gopls call hierarchy result
	b.WriteString(outputStr)
	b.WriteString("\n")
}
