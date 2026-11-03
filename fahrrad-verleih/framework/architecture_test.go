package framework

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const module = "fahrradverleih/"

// forbiddenWords sind Begriffe konkreter Verleiharten (A-R3). Importpfade sind ausgenommen,
// weil der Modulname „fahrradverleih“ lautet.
var forbiddenWords = regexp.MustCompile(`(?i)fahrrad|bike|\bvelo\b|\bski\b|\bskis\b`)

type goFile struct {
	path    string
	imports []string
	texts   []string // Bezeichner, Zeichenketten und Kommentare ohne Importpfade
}

// parseDir liest alle Nicht-Test-Dateien unter dir.
func parseDir(t *testing.T, dir string) []goFile {
	t.Helper()
	var files []goFile
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		gf := goFile{path: path}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			gf.imports = append(gf.imports, p)
		}
		for _, c := range f.Comments {
			gf.texts = append(gf.texts, c.Text())
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ImportSpec:
				return false
			case *ast.Ident:
				gf.texts = append(gf.texts, n.Name)
			case *ast.BasicLit:
				gf.texts = append(gf.texts, n.Value)
			}
			return true
		})
		files = append(files, gf)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("keine Go-Dateien unter %s gefunden", dir)
	}
	return files
}

// AK-22, A-R1: Das Framework importiert keine Plugins, keine Anwendung und kein Web.
func TestFrameworkImportsNoApplicationCode(t *testing.T) {
	for _, f := range parseDir(t, ".") {
		for _, imp := range f.imports {
			if strings.HasPrefix(imp, module) && !strings.HasPrefix(imp, module+"framework/") {
				t.Errorf("%s importiert %s (A-R1)", f.path, imp)
			}
		}
	}
}

// AK-22, A-R3: Im Framework-Code kommen keine Begriffe einer konkreten Verleihart vor.
func TestFrameworkKnowsNoRentalKind(t *testing.T) {
	for _, f := range parseDir(t, ".") {
		for _, text := range f.texts {
			if w := forbiddenWords.FindString(text); w != "" {
				t.Errorf("%s enthält %q in %q (A-R3)", f.path, w, strings.TrimSpace(text))
			}
		}
	}
}

// AK-22, A-R2: Plugins importieren nur framework/domain und framework/plugin.
func TestPluginsImportOnlyDomainAndPlugin(t *testing.T) {
	allowed := map[string]bool{module + "framework/domain": true, module + "framework/plugin": true}
	for _, f := range parseDir(t, filepath.Join("..", "plugins")) {
		for _, imp := range f.imports {
			if strings.HasPrefix(imp, module) && !allowed[imp] {
				t.Errorf("%s importiert %s (A-R2)", f.path, imp)
			}
		}
	}
}
