package cloudflarecore

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 请求头里带 API token，默认客户端必须校验证书。
func TestClientVerifiesCertificatesByDefault(t *testing.T) {
	// 默认协议是 auto，会先花 10 秒等 QUIC 握手超时；这个断言只关心 TLS 校验。
	t.Setenv("CFST_HTTP_PROTOCOL", "h1")

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
	}))
	defer server.Close()

	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, Token: "token"})
	res, err := client.httpClient.Get(server.URL)
	if err == nil {
		_ = res.Body.Close()
		t.Fatal("Cloudflare 客户端接受了自签证书，证书校验被关闭")
	}
}
