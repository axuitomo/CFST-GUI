package task

import (
	"strings"
	"testing"
)

// 真实响应形态（自有域名 + 一颗边缘 IP 受限的地址实测）：
//
//	GET /cdn-cgi/trace        → 403，17 字节纯文本 "error code: 1034"
//	GET / 带 Accept: text/html → 403，7457 字节品牌页，标题 "Edge IP Restricted"，
//	                            "error code: 1034" 字面量不存在，JS 里是 "errorCode: 1034"（offset 1042）
func TestIsEdgeRestrictedBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"追踪阶段纯文本", "error code: 1034", true},
		{"纯文本带换行", "error code: 1034\n", true},
		{"品牌页标题", `<html><head><title>Edge IP Restricted | a.example | Cloudflare</title></head>`, true},
		{"品牌页内联脚本", `properties:{errorCode: 1034 },helpful:a,version: 1`, true},
		{"大小写不敏感", "EDGE IP RESTRICTED / ERRORCODE: 1034", true},
		{"普通 403", `<html><head><title>403 Forbidden</title></head><body><center><h1>403 Forbidden</h1></center></body></html>`, false},
		{"其它 CF 错误码", "error code: 1020", false},
		{"空正文", "", false},
	}
	for _, tc := range cases {
		if got := isEdgeRestrictedBody([]byte(tc.body)); got != tc.want {
			t.Fatalf("%s: isEdgeRestrictedBody = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// 品牌页里 "errorCode: 1034" 在 offset 1042，嗅探额度必须盖住它，
// 否则文件测速会把这类 403 记成普通 forbidden。
func TestEdgeRestrictedSniffCoversBrandedPageMarker(t *testing.T) {
	const brandedPageMarkerOffset = 1042
	page := strings.Repeat(" ", brandedPageMarkerOffset) + "errorCode: 1034 }"
	sniffed := page
	if len(sniffed) > downloadEdgeRestrictedSniffBytes {
		sniffed = sniffed[:downloadEdgeRestrictedSniffBytes]
	}
	if !isEdgeRestrictedBody([]byte(sniffed)) {
		t.Fatalf("嗅探 %d 字节后没认出品牌页标记（标记在 offset %d）", downloadEdgeRestrictedSniffBytes, brandedPageMarkerOffset)
	}
}
