package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 更新元数据与安装包共用这个客户端：它必须校验证书，否则 manifest 里的 SHA256 也来自同一条可伪造通道。
func TestUpdateHTTPClientVerifiesCertificates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	if _, err := httpClientForUpdates.Get(server.URL); err == nil {
		t.Fatal("更新客户端接受了自签证书，证书校验被关闭")
	}
}
