package main

import (
	"flag"

	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	sidecarRoot := flags.String("sidecar_root", "", "root directory to search for sidecar .methods.go files")

	protogen.Options{ParamFunc: flags.Set}.Run(func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			if err := generateMethods(gen, f, *sidecarRoot); err != nil {
				return err
			}
		}
		return nil
	})
}
