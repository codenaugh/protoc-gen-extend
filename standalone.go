package main

import (
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
)

// runGenerate is the standalone mode: it walks sidecarRoot for
// *.proto.ext.go files, parses the matching .proto files directly
// (no protoc involved), validates receivers, and writes the generated
// *_ext.pb.go files. This lets callers run the whole step as
//
//	go run github.com/codenaugh/protoc-gen-extend@<version> generate ...
//
// with no install and no PATH setup.
func runGenerate(args []string) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	sidecarRoot := flags.String("sidecar_root", ".", "root directory to search for sidecar .proto.ext.go files")
	out := flags.String("out", "", "output root directory (required)")
	module := flags.String("module", "", "module prefix stripped from go_package to compute output paths (mirrors protoc-gen-go's module= option)")
	sourceRelative := flags.Bool("paths_source_relative", false, "place output next to the proto's own path relative to out (mirrors paths=source_relative)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("generate: -out is required")
	}
	if *module == "" && !*sourceRelative {
		return fmt.Errorf("generate: one of -module or -paths_source_relative is required")
	}
	if *module != "" && *sourceRelative {
		return fmt.Errorf("generate: -module and -paths_source_relative are mutually exclusive")
	}

	var sidecars []string
	err := filepath.WalkDir(*sidecarRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".proto.ext.go") {
			sidecars = append(sidecars, p)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("generate: walking %s: %w", *sidecarRoot, err)
	}

	for _, sidecar := range sidecars {
		if err := generateStandalone(sidecar, *sidecarRoot, *out, *module, *sourceRelative); err != nil {
			return err
		}
	}
	return nil
}

// generateStandalone processes one sidecar file against its proto.
func generateStandalone(sidecarPath, sidecarRoot, out, module string, sourceRelative bool) error {
	protoPath := strings.TrimSuffix(sidecarPath, ".ext.go")
	if _, err := os.Stat(protoPath); err != nil {
		return fmt.Errorf("generate: sidecar %s has no matching proto file %s", sidecarPath, filepath.Base(protoPath))
	}

	info, err := parseProto(protoPath)
	if err != nil {
		return err
	}
	if info.goPackage == "" {
		return fmt.Errorf("generate: %s has no go_package option; required to place output", protoPath)
	}

	content, err := parseSidecarFile(sidecarPath, info.messages, info.messageNames, filepath.Base(protoPath))
	if err != nil {
		return fmt.Errorf("parsing sidecar %s: %w", sidecarPath, err)
	}
	if len(content.blocks) == 0 {
		return nil
	}

	// Resolve the output directory.
	importPath, pkgName := splitGoPackage(info.goPackage)
	var outDir string
	if sourceRelative {
		rel, err := filepath.Rel(sidecarRoot, filepath.Dir(protoPath))
		if err != nil {
			return err
		}
		outDir = filepath.Join(out, rel)
	} else {
		rel := strings.TrimPrefix(importPath, strings.TrimSuffix(module, "/")+"/")
		if rel == importPath {
			return fmt.Errorf("generate: go_package %q of %s does not start with module prefix %q", importPath, protoPath, module)
		}
		outDir = filepath.Join(out, filepath.FromSlash(rel))
	}

	base := strings.TrimSuffix(filepath.Base(protoPath), ".proto")
	outPath := filepath.Join(outDir, base+"_ext.pb.go")

	var b strings.Builder
	p := func(args ...interface{}) {
		for _, a := range args {
			fmt.Fprint(&b, a)
		}
		b.WriteByte('\n')
	}
	writeExtFile(p, filepath.Base(sidecarPath), pkgName, content)

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return fmt.Errorf("generate: formatting output for %s: %w", sidecarPath, err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, src, 0o644)
}

// protoInfo is what the standalone path needs from a proto file:
// its message names (as generated Go type names) and go_package.
type protoInfo struct {
	messages     map[string]bool
	messageNames []string
	goPackage    string
}

// parseProto syntactically parses a single .proto file — no import
// resolution, since message names and options are all we need.
func parseProto(protoPath string) (*protoInfo, error) {
	f, err := os.Open(protoPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	errHandler := reporter.NewHandler(nil)
	fileNode, err := parser.Parse(protoPath, f, errHandler)
	if err != nil {
		return nil, fmt.Errorf("generate: parsing %s: %w", protoPath, err)
	}

	info := &protoInfo{messages: make(map[string]bool)}
	for _, decl := range fileNode.Decls {
		switch n := decl.(type) {
		case *ast.MessageNode:
			collectMessages(n, "", info)
		case *ast.OptionNode:
			if optionName(n) == "go_package" {
				if s, ok := n.Val.Value().(string); ok {
					info.goPackage = s
				}
			}
		}
	}
	return info, nil
}

// collectMessages records a message and its nested messages under their
// generated Go type names (Parent_Nested).
func collectMessages(msg *ast.MessageNode, prefix string, info *protoInfo) {
	goName := goCamelCase(msg.Name.Val)
	if prefix != "" {
		goName = prefix + "_" + goName
	}
	info.messages[goName] = true
	info.messageNames = append(info.messageNames, goName)

	for _, decl := range msg.Decls {
		if nested, ok := decl.(*ast.MessageNode); ok {
			collectMessages(nested, goName, info)
		}
	}
}

// optionName renders a (possibly dotted) option name node as a string.
func optionName(n *ast.OptionNode) string {
	var parts []string
	for _, part := range n.Name.Parts {
		parts = append(parts, string(part.Name.AsIdentifier()))
	}
	return strings.Join(parts, ".")
}

// splitGoPackage splits a go_package option into import path and
// package name ("path;name" form, else the last path element).
func splitGoPackage(goPackage string) (importPath, pkgName string) {
	if i := strings.Index(goPackage, ";"); i >= 0 {
		return goPackage[:i], goPackage[i+1:]
	}
	name := path.Base(goPackage)
	// Sanitize the same way protoc-gen-go does for derived names.
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return '_'
	}, name)
	return goPackage, name
}

// goCamelCase converts a proto identifier to the generated Go name,
// following protoc-gen-go's rules (underscores capitalize the next
// letter; leading digits are preserved behind an underscore).
func goCamelCase(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.' && i+1 < len(s) && isASCIILower(s[i+1]):
			// Skip over '.' in ".{{lowercase}}".
		case c == '.':
			b = append(b, '_')
		case c == '_' && (i == 0 || s[i-1] == '.'):
			// Convert initial '_' to ensure we start with a capital letter.
			b = append(b, 'X')
		case c == '_' && i+1 < len(s) && isASCIILower(s[i+1]):
			// Skip over '_' in "_{{lowercase}}".
		case isASCIIDigit(c):
			b = append(b, c)
		default:
			// Assume we have a letter now - if not, it's a bogus identifier.
			// The next word is a sequence of characters that must start upper case.
			if isASCIILower(c) {
				c -= 'a' - 'A' // convert lowercase to uppercase
			}
			b = append(b, c)

			// Accept lower case sequence that follows.
			for ; i+1 < len(s) && isASCIILower(s[i+1]); i++ {
				b = append(b, s[i+1])
			}
		}
	}
	return string(b)
}

func isASCIILower(c byte) bool { return 'a' <= c && c <= 'z' }
func isASCIIDigit(c byte) bool { return '0' <= c && c <= '9' }
