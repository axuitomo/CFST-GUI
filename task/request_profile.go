package task

import (
	"context"
	"net"

	"github.com/XIU2/CloudflareSpeedTest/internal/httpcfg"
	"github.com/XIU2/CloudflareSpeedTest/utils"
)

var (
	UserAgent          = httpcfg.DefaultUserAgent
	HostHeader         = ""
	SNI                = ""
	CaptureAddress     = ""
	InsecureSkipVerify = true
)

func currentRequestProfile() httpcfg.Profile {
	captureAddress := ""
	if utils.Debug {
		captureAddress = CaptureAddress
	}
	return httpcfg.Resolve(UserAgent, HostHeader, SNI, captureAddress, InsecureSkipVerify)
}

func getDialContext(ip *net.IPAddr, profile httpcfg.Profile) func(ctx context.Context, network, address string) (net.Conn, error) {
	dialAddress := profile.DialAddress(ip, TCPPort)
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, dialAddress)
	}
}
