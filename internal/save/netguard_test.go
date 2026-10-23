package save

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// README and SECURITY.md say the editor's own code makes no outbound network connection: no telemetry, no update check.
// This test makes the claim fail a build: shipped code may listen on the loopback address, but it may not dial out or use
// an HTTP client (register 2026-09-30, F-22: a check that forbids outbound network use).
func TestShippedCodeMakesNoOutboundConnections(t *testing.T) {
	root := filepath.Join("..", "..")
	forbidden := map[string]map[string]bool{
		"http": set("Get", "Head", "Post", "PostForm", "DefaultClient", "Client", "NewRequest", "NewRequestWithContext", "Transport", "DefaultTransport"),
		"net":  set("Dial", "DialTimeout", "Dialer", "DialTCP", "DialUDP", "DialIP", "DialUnix", "LookupHost", "LookupAddr", "LookupIP", "LookupCNAME", "LookupMX", "LookupTXT", "LookupSRV", "Resolver", "DefaultResolver"),
		"tls":  set("Dial", "DialWithDialer", "Dialer"),
		"rpc":  set("Dial", "DialHTTP", "DialHTTPPath"),
		"smtp": set("Dial", "SendMail"),
	}
	// The one exception: the check that "localhost" resolves only to loopback addresses (audit NET-4) asks the operating
	// system's resolver, which answers a name from the hosts file; the address it is given is never contacted.
	allowed := map[string]bool{filepath.Join("internal", "web", "listen.go") + " net.LookupIP": true}
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", "node_modules", "web-tests":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, filepath.Join("internal", "testsave")) {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		checked++
		rel, _ := filepath.Rel(root, path)
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && forbidden[id.Name][sel.Sel.Name] && !allowed[rel+" "+id.Name+"."+sel.Sel.Name] {
				t.Errorf("%s uses %s.%s: shipped code must not open outbound connections", rel, id.Name, sel.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 20 {
		t.Fatalf("only %d production files were checked: the walk is not seeing the tree", checked)
	}
}

func set(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}
