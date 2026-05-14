package main

import (
	"os"

	"github.com/axuitomo/CFST-GUI/internal/app"
)

func main() {
	app.Run(os.Args[1:], runtimeResources())
}
