package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestCanonicalCommandNameIsTaple(t *testing.T) {
	if got := newCLI().Name; got != "taple" {
		t.Fatalf("unexpected product name: %q", got)
	}
}

func TestProjectEnvironmentNamespaceIsTaple(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	legacy := regexp.MustCompile(`\b(?:TAPEFOX|MUSICFOX)_[A-Z][A-Z0-9_]*\b`)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor", "third_party", "bin", "dist", ".devenv":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		testFile := strings.HasSuffix(path, "_test.go")
		testEnvironmentReads := make(map[*ast.BasicLit]bool)
		if testFile {
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (selector.Sel.Name != "Getenv" && selector.Sel.Name != "LookupEnv") {
					return true
				}
				if literal, ok := call.Args[0].(*ast.BasicLit); ok {
					testEnvironmentReads[literal] = true
				}
				return true
			})
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING || (testFile && !testEnvironmentReads[literal]) {
				return true
			}
			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr == nil && legacy.MatchString(value) {
				relative, _ := filepath.Rel(root, path)
				t.Errorf("legacy project environment in %s: %s", relative, legacy.FindString(value))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIssueTemplateUsesActualTapleLogLocations(t *testing.T) {
	data, err := os.ReadFile("../.github/ISSUE_TEMPLATE/1_bug_report.yml")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, expected := range []string{"${TAPLE_ROOT}/log/taple.log", "${XDG_STATE_HOME}/go-musicfox/log/taple.log", ".local/state/go-musicfox/log/taple.log"} {
		if !strings.Contains(content, expected) {
			t.Errorf("issue template does not document %s", expected)
		}
	}
	for _, invalid := range []string{"${TAPLE_ROOT}/taple.log", "OS config directory", "系统配置目录"} {
		if strings.Contains(content, invalid) {
			t.Errorf("issue template recommends an incorrect log location: %s", invalid)
		}
	}
}
