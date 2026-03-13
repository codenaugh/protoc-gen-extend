package main

import (
	"flag"

	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
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
