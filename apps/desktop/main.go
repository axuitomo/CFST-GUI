// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"

	"github.com/axuitomo/CFST-GUI/internal/adapters/desktop"
)

func main() {
	desktop.Run(os.Args[1:], runtimeResources())
}
