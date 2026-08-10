package task

import (
	"context"
	"net"

	"github.com/axuitomo/CFST-GUI/internal/httpcfg"
)

func (e *Engine) currentRequestProfile() httpcfg.Profile {
	captureAddress := ""
	if e.config.Debug {
		captureAddress = e.config.CaptureAddress
	}
	hostHeader := e.config.HostHeader
	sni := e.config.SNI
	if hostHeader == "" {
		hostHeader = httpcfg.URLHostHeader(e.config.TraceURL)
	}
	if sni == "" {
		sni = httpcfg.URLHost(e.config.TraceURL)
	}
	return httpcfg.ResolveWithHeaders(e.config.UserAgent, hostHeader, sni, captureAddress, e.config.InsecureSkipVerify, e.config.RequestHeaders)
}

func (e *Engine) getDialContext(ip *net.IPAddr, profile httpcfg.Profile) func(ctx context.Context, network, address string) (net.Conn, error) {
	dialAddress := profile.DialAddress(ip, e.config.TCPPort)
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, dialAddress)
	}
}
