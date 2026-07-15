package main

import (
	"flag"
	"fmt"
	"os"

	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	// Standalone mode: `protoc-gen-extend generate ...` runs without
	// protoc (and therefore without any install/PATH setup when
	// invoked as `go run github.com/codenaugh/protoc-gen-extend@<ver>
	// generate ...`). Any other invocation is the classic protoc
	// plugin protocol on stdin/stdout.
	if len(os.Args) > 1 && os.Args[1] == "generate" {
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	var flags flag.FlagSet
	sidecarRoot := flags.String("sidecar_root", "", "root directory to search for sidecar .proto.ext.go files")

	protogen.Options{ParamFunc: flags.Set}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = 1 // FEATURE_PROTO3_OPTIONAL
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			if err := generateExtensions(gen, f, *sidecarRoot); err != nil {
				return err
			}
		}
		return nil
	})
}
