package appcore

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

// 来源内容决定探测对象，所以无论探测阶段是否跳过校验，来源抓取都必须校验证书。
func TestSourceHTTPClientAlwaysVerifiesCertificates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	for _, verify := range []bool{true, false} {
		cfg := probecore.DefaultProbeConfig()
		// 关掉 DoH/ECH，避免测试里发起真实 DNS 查询。
		cfg.DoHURL = ""
		cfg.ECH = false
		cfg.VerifyTLSCertificate = verify

		if _, _, err := FetchSourceURL(server.URL, cfg, NewSourceHTTPClient(cfg, SourceHTTPClientOptions{})); err == nil {
			t.Fatalf("verify_tls_certificate=%v 时来源客户端接受了自签证书", verify)
		}
	}
}
