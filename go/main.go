package main

import (
	"log"

	"github.com/aoldershaw/prototype-sdk-go"
)

func main() {
	proto := prototype.New(
		prototype.WithIcon("mdi:language-go"),
		prototype.WithObject(Module{},
			prototype.WithMessage("build", (Module).Build),
		),
	)
	if err := proto.Execute(); err != nil {
		log.Fatal(err)
	}
}
