package probecore

import (
	"net"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestLimitFinalProbeResultsUsesWeightedOrderAndKeepsRowsAligned(t *testing.T) {
	raw := []utils.CloudflareIPData{
		probeCoreTestData("1.1.1.1", 10*time.Millisecond, 10),
		probeCoreTestData("1.1.1.2", 20*time.Millisecond, 100),
		probeCoreTestData("1.1.1.3", 10*time.Millisecond, 50),
	}
	rows := []ProbeRow{
		{IP: "1.1.1.1", TestPort: 443},
		{IP: "1.1.1.2", TestPort: 2053},
		{IP: "1.1.1.3", TestPort: 8443},
	}

	selectedRaw, selectedRows := LimitFinalProbeResults(raw, rows, 2, utils.DownloadSpeedMetricAverage)
	if len(selectedRaw) != 2 || len(selectedRows) != 2 {
		t.Fatalf("selected counts = %d/%d, want 2/2", len(selectedRaw), len(selectedRows))
	}
	if selectedRaw[0].IP.String() != "1.1.1.2" || selectedRows[0].IP != "1.1.1.2" || selectedRows[0].TestPort != 2053 {
		t.Fatalf("first selected raw/row = %#v/%#v, want 1.1.1.2 with port 2053", selectedRaw[0].IP, selectedRows[0])
	}
	if selectedRaw[1].IP.String() != "1.1.1.3" || selectedRows[1].IP != "1.1.1.3" || selectedRows[1].TestPort != 8443 {
		t.Fatalf("second selected raw/row = %#v/%#v, want 1.1.1.3 with port 8443", selectedRaw[1].IP, selectedRows[1])
	}
}

func TestSelectTopProbeRowsByMetricUsesWeightedOrder(t *testing.T) {
	rows := []ProbeRow{
		{IP: "1.1.1.1", DelayMS: 10, DownloadSpeedMB: 10, MaxDownloadSpeedMB: 10, LossRate: 0.01},
		{IP: "1.1.1.2", DelayMS: 20, DownloadSpeedMB: 100, MaxDownloadSpeedMB: 100, LossRate: 0.01},
		{IP: "1.1.1.3", DelayMS: 10, DownloadSpeedMB: 50, MaxDownloadSpeedMB: 50, LossRate: 0.01},
	}

	selected := SelectTopProbeRowsByMetric(rows, 2, utils.DownloadSpeedMetricAverage)
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
	if selected[0].IP != "1.1.1.2" {
		t.Fatalf("first selected IP = %q, want 1.1.1.2", selected[0].IP)
	}
	if selected[1].IP != "1.1.1.3" {
		t.Fatalf("second selected IP = %q, want 1.1.1.3", selected[1].IP)
	}
}

func probeCoreTestData(ip string, delay time.Duration, speedMB float64) utils.CloudflareIPData {
	return utils.CloudflareIPData{
		PingData: &utils.PingData{
			IP:       &net.IPAddr{IP: net.ParseIP(ip)},
			Sended:   4,
			Received: 4,
			Delay:    delay,
		},
		DownloadSpeed: speedMB * 1024 * 1024,
	}
}
