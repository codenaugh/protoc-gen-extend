package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// sidecarContent is the parsed, validated content of a sidecar file,
// ready to emit into a generated package.
type sidecarContent struct {
	blocks  []string
	imports []string
}

// parseSidecarFile parses a sidecar .proto.ext.go file, validates that
// method receivers reference messages defined in the proto file
// (knownMessages, keyed by generated Go type name), and returns the
// source blocks ready to emit. protoName and messageNames are used for
// error messages only.
func parseSidecarFile(path string, knownMessages map[string]bool, messageNames []string, protoName string) (*sidecarContent, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	content := &sidecarContent{}

	// Collect imports from the sidecar (excluding the package declaration).
	for _, imp := range astFile.Imports {
		content.imports = append(content.imports, strings.TrimSpace(string(src[imp.Pos()-1:imp.End()-1])))
	}

	// declBlock cuts a declaration's source, including its doc comment
	// when present (decl.Pos() alone would drop it).
	declBlock := func(docPos, declPos, end token.Pos) string {
		start := declPos
		if docPos.IsValid() {
			start = docPos
		}
		return string(src[start-1 : end-1])
	}

	// Extract declarations: functions, methods, constants, and variables.
	for _, decl := range astFile.Decls {
		// Handle const/var/type declarations.
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			// Skip import declarations (handled above).
			if genDecl.Tok == token.IMPORT {
				continue
			}
			docPos := token.NoPos
			if genDecl.Doc != nil {
				docPos = genDecl.Doc.Pos()
			}
			content.blocks = append(content.blocks, declBlock(docPos, genDecl.Pos(), genDecl.End()))
			continue
		}

		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		docPos := token.NoPos
		if fn.Doc != nil {
			docPos = fn.Doc.Pos()
		}

		if fn.Recv == nil {
			// Top-level function (no receiver) — include it as-is.
			content.blocks = append(content.blocks, declBlock(docPos, fn.Pos(), fn.End()))
			continue
		}

		// Validate the receiver type is a known proto message.
		recvType := receiverTypeName(fn.Recv)
		if recvType == "" {
			continue
		}
		if !knownMessages[recvType] {
			return nil, fmt.Errorf(
				"method %s has receiver *%s, but %s is not a message in %s\n  available messages: %s",
				fn.Name.Name, recvType, recvType, protoName,
				strings.Join(messageNames, ", "),
			)
		}

		content.blocks = append(content.blocks, declBlock(docPos, fn.Pos(), fn.End()))
	}

	return content, nil
}

// receiverTypeName extracts the type name from a method receiver. It handles
// both pointer (*User) and value (User) receivers.
func receiverTypeName(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	expr := fields.List[0].Type

	// Pointer receiver: *User
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}

	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}
