package main

import (
	"os"

	"github.com/axuitomo/CFST-GUI/internal/adapters/webui"
)

func main() {
	webui.Run(os.Args[1:], runtimeResources())
}
