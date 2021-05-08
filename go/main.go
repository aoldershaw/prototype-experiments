package main

import (
	"log"

	"github.com/aoldershaw/prototype-experiments/go/build"
	"github.com/aoldershaw/prototype-experiments/go/module"
	"github.com/aoldershaw/prototype-sdk-go"
)

func main() {
	proto := prototype.New(
		prototype.WithIcon("mdi:language-go"),
		prototype.WithObject(module.Module{},
			prototype.WithMessage("build", build.Build),
		),
	)
	if err := proto.Execute(); err != nil {
		log.Fatal(err)
	}
}
