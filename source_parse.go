package main

import (
	"net"

	"github.com/axuitomo/CFST-GUI/internal/sourceparse"
)

var sourceParseResolver sourceparse.Resolver = net.DefaultResolver
