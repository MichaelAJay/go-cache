package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser" 
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExportInfo holds information about an exportable item
type ExportInfo struct {
	Name     string
	Type     string // "type", "const", "var", "func"
	Category string // grouping category
}

func main() {
	fmt.Println("Generating exports.go...")
	
	// Parse interfaces package
	interfacesDir := "interfaces"
	errorsDir := "cache_errors"
	
	exports := []ExportInfo{}
	
	// Parse interfaces package for types, constants, and functions
	interfaceExports, err := parsePackageExports(interfacesDir, "interfaces")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing interfaces package: %v\n", err)
		os.Exit(1)
	}
	exports = append(exports, interfaceExports...)
	
	// Parse cache_errors package for error variables
	errorExports, err := parsePackageExports(errorsDir, "cache_errors")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing cache_errors package: %v\n", err)
		os.Exit(1)
	}
	exports = append(exports, errorExports...)
	
	// Generate exports.go
	err = generateExportsFile(exports)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating exports.go: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("exports.go generated successfully!")
}

func parsePackageExports(dir, pkgName string) ([]ExportInfo, error) {
	var exports []ExportInfo
	
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		
		fileExports, err := parseFileExports(path, pkgName)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		
		exports = append(exports, fileExports...)
		return nil
	})
	
	return exports, err
}

func parseFileExports(filePath, pkgName string) ([]ExportInfo, error) {
	var exports []ExportInfo
	
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			switch d.Tok {
			case token.TYPE:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ast.IsExported(ts.Name.Name) {
						category := categorizeType(ts.Name.Name)
						exports = append(exports, ExportInfo{
							Name:     ts.Name.Name,
							Type:     "type",
							Category: category,
						})
					}
				}
			case token.CONST:
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range vs.Names {
							if ast.IsExported(name.Name) {
								exports = append(exports, ExportInfo{
									Name:     name.Name,
									Type:     "const",
									Category: "constants",
								})
							}
						}
					}
				}
			case token.VAR:
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range vs.Names {
							if ast.IsExported(name.Name) {
								category := "variables"
								if pkgName == "cache_errors" {
									category = "errors"
								}
								exports = append(exports, ExportInfo{
									Name:     name.Name,
									Type:     "var",
									Category: category,
								})
							}
						}
					}
				}
			}
		case *ast.FuncDecl:
			if d.Name != nil && ast.IsExported(d.Name.Name) {
				exports = append(exports, ExportInfo{
					Name:     d.Name.Name,
					Type:     "func",
					Category: "functions",
				})
			}
		}
	}
	
	return exports, nil
}

func categorizeType(typeName string) string {
	switch {
	case strings.Contains(typeName, "Cache") && !strings.Contains(typeName, "Config"):
		return "interfaces"
	case strings.Contains(typeName, "Config") || strings.Contains(typeName, "Options") || strings.Contains(typeName, "Hooks"):
		return "configuration"
	case strings.Contains(typeName, "Metrics"):
		return "metrics"
	case strings.Contains(typeName, "Event") || strings.Contains(typeName, "Reason"):
		return "events"
	default:
		return "types"
	}
}

func generateExportsFile(exports []ExportInfo) error {
	file, err := os.Create("exports.go")
	if err != nil {
		return err
	}
	defer file.Close()
	
	w := bufio.NewWriter(file)
	defer w.Flush()
	
	// Write header
	fmt.Fprintln(w, "package cache")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "//go:generate go run tools/generate_exports.go")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "import (")
	
	// Determine which imports are needed
	needsErrors := false
	needsInterfaces := false
	
	for _, export := range exports {
		if export.Category == "errors" {
			needsErrors = true
		} else {
			needsInterfaces = true
		}
	}
	
	if needsErrors {
		fmt.Fprintln(w, "\tcacheErrors \"github.com/MichaelAJay/go-cache/cache_errors\"")
	}
	if needsInterfaces {
		fmt.Fprintln(w, "\t\"github.com/MichaelAJay/go-cache/interfaces\"")
	}
	
	fmt.Fprintln(w, ")")
	fmt.Fprintln(w)
	
	// Group exports by category
	categories := make(map[string][]ExportInfo)
	for _, export := range exports {
		categories[export.Category] = append(categories[export.Category], export)
	}
	
	// Sort categories for consistent output
	var categoryNames []string
	for cat := range categories {
		categoryNames = append(categoryNames, cat)
	}
	sort.Strings(categoryNames)
	
	// Generate each category
	for _, catName := range categoryNames {
		items := categories[catName]
		sort.Slice(items, func(i, j int) bool {
			return items[i].Name < items[j].Name
		})
		
		writeCategory(w, catName, items)
	}
	
	// Write validation section
	writeValidation(w, exports)
	
	return nil
}

func writeCategory(w *bufio.Writer, category string, items []ExportInfo) {
	if len(items) == 0 {
		return
	}
	
	// Category header
	fmt.Fprintln(w, "// =============================================================================")
	fmt.Fprintf(w, "// %s\n", strings.ToUpper(category))
	fmt.Fprintln(w, "// =============================================================================")
	
	// Group by type within category
	types := make(map[string][]ExportInfo)
	for _, item := range items {
		types[item.Type] = append(types[item.Type], item)
	}
	
	// Write types first
	if typeItems, ok := types["type"]; ok {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "type (")
		for _, item := range typeItems {
			fmt.Fprintf(w, "\t%s = interfaces.%s\n", item.Name, item.Name)
		}
		fmt.Fprintln(w, ")")
	}
	
	// Write constants
	if constItems, ok := types["const"]; ok {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "const (")
		for _, item := range constItems {
			fmt.Fprintf(w, "\t%s = interfaces.%s\n", item.Name, item.Name)
		}
		fmt.Fprintln(w, ")")
	}
	
	// Write variables (including functions and errors)
	varItems := []ExportInfo{}
	if items, ok := types["var"]; ok {
		varItems = append(varItems, items...)
	}
	if items, ok := types["func"]; ok {
		varItems = append(varItems, items...)
	}
	
	if len(varItems) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "var (")
		for _, item := range varItems {
			if item.Category == "errors" {
				fmt.Fprintf(w, "\t%s = cacheErrors.%s\n", item.Name, item.Name)
			} else {
				fmt.Fprintf(w, "\t%s = interfaces.%s\n", item.Name, item.Name)
			}
		}
		fmt.Fprintln(w, ")")
	}
	
	fmt.Fprintln(w)
}

func writeValidation(w *bufio.Writer, exports []ExportInfo) {
	fmt.Fprintln(w, "// =============================================================================")
	fmt.Fprintln(w, "// COMPILE-TIME VALIDATION")
	fmt.Fprintln(w, "// =============================================================================")
	fmt.Fprintln(w, "// Ensure re-exported types maintain compatibility")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "var (")
	
	// Find some representative function exports for validation
	funcCount := 0
	for _, export := range exports {
		if export.Type == "func" && strings.HasPrefix(export.Name, "With") && funcCount < 3 {
			fmt.Fprintf(w, "\t_ CacheOption = interfaces.%s", export.Name)
			// Use appropriate zero values for different function types
			switch export.Name {
			case "WithTTL", "WithCleanupInterval":
				fmt.Fprintln(w, "(0)")
			case "WithMaxEntries", "WithMaxSize":
				fmt.Fprintln(w, "(0)")
			case "WithMetricsEnabled", "WithDetailedMetrics":
				fmt.Fprintln(w, "(false)")
			default:
				fmt.Fprintln(w, "(nil)")
			}
			funcCount++
		}
	}
	
	// Add constant validation
	constCount := 0
	for _, export := range exports {
		if export.Type == "const" && constCount < 2 {
			if strings.Contains(export.Name, "Cache") {
				fmt.Fprintf(w, "\t_ CacheEvent = interfaces.%s\n", export.Name)
			} else if strings.Contains(export.Name, "Cleanup") {
				fmt.Fprintf(w, "\t_ CleanupReason = interfaces.%s\n", export.Name)
			}
			constCount++
		}
	}
	
	fmt.Fprintln(w, ")")
}