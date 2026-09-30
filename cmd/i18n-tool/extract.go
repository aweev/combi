// combi/cmd/i18n-tool/extract.go
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
)

func extractCmd(args []string) int {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	src := fs.String("src", ".", "source directory to scan for T() and TPlural() calls")
	fs.Parse(args)

	keys, err := extractKeys(*src)
	if err != nil {
		return exitOnError(err)
	}

	for _, key := range keys {
		fmt.Println(key)
	}

	return 0
}

func extractKeys(path string) ([]string, error) {
	fset := token.NewFileSet()
	keySet := make(map[string]struct{})

	walkFn := func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(filePath) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", filePath, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			var fnName string
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				fnName = fun.Sel.Name
			case *ast.Ident:
				fnName = fun.Name
			}

			if fnName != "T" && fnName != "TPlural" {
				return true
			}
			if len(call.Args) == 0 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			key, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			keySet[key] = struct{}{}
			return true
			
		})
		return nil
	}

	if err := filepath.WalkDir(path, walkFn); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}
