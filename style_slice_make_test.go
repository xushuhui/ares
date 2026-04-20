package ares

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestSliceInitializationUsesMake(t *testing.T) {
	const root = "."

	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		fileNode, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}

		ast.Inspect(fileNode, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}

			arrType, ok := lit.Type.(*ast.ArrayType)
			if !ok {
				return true
			}

			if arrType.Len != nil {
				return true
			}

			position := fset.Position(lit.Pos())
			t.Errorf("slice literal is not allowed, use make instead: %s:%d", position.Filename, position.Line)
			return true
		})

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk project files: %v", err)
	}
}
