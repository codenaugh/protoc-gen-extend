package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestReceiverTypeName(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "pointer receiver",
			src:  `package x; func (m *User) Foo() {}`,
			want: "User",
		},
		{
			name: "value receiver",
			src:  `package x; func (m User) Foo() {}`,
			want: "User",
		},
		{
			name: "no receiver",
			src:  `package x; func Foo() {}`,
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.go", test.src, 0)
			if err != nil {
				t.Fatal(err)
			}

			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				got := receiverTypeName(fn.Recv)
				if got != test.want {
					t.Errorf("receiverTypeName() = %q, want %q", got, test.want)
				}
			}
		})
	}
}
