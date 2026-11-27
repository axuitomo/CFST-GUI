package mobileapi

import (
	"net"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/sourceparse"
)

var sourceParseResolver sourceparse.Resolver = net.DefaultResolver
