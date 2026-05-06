package mobileapi

import (
	"net"

	"github.com/XIU2/CloudflareSpeedTest/internal/sourceparse"
)

var sourceParseResolver sourceparse.Resolver = net.DefaultResolver
