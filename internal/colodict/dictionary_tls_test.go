package colodict

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultUpdateHTTPClientVerifiesCertificates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ip_prefix,colo\n"))
	}))
	defer server.Close()

	client := defaultUpdateHTTPClient()
	defer client.CloseIdleConnections()
	if _, err := client.Get(server.URL); err == nil {
		t.Fatal("COLO 词典客户端接受了自签证书，证书校验被关闭")
	}
}
