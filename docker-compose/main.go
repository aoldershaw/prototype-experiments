package main

import (
	"log"

	"github.com/aoldershaw/prototype-experiments/docker-compose/compose"
	prototype "github.com/aoldershaw/prototype-sdk-go"
)

func main() {
	proto := prototype.New(
		prototype.WithIcon("mdi:docker"),
		prototype.WithObject(compose.Project{},
			prototype.WithMessage("up", (compose.Project).Up),
		),
	)
	if err := proto.Execute(); err != nil {
		log.Fatal(err)
	}
}
