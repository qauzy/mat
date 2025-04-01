package http

import (
	"context"
	"crypto/tls"
	"github.com/qauzy/mat/cloudflare"
	"github.com/qauzy/mat/x"
	"io"
	"net"
	"net/http"
	URL "net/url"
	"runtime"
	"strings"
	"time"

	"github.com/qauzy/mat/component/ca"
	C "github.com/qauzy/mat/constant"
	"github.com/qauzy/mat/listener/inner"
)

func HttpRequest(ctx context.Context, url, method string, header map[string][]string, body io.Reader) (*http.Response, error) {
	return HttpRequestWithProxy(ctx, url, method, header, body, "")
}

func HttpRequestWithProxy(ctx context.Context, url, method string, header map[string][]string, body io.Reader, specialProxy string) (*http.Response, error) {
	method = strings.ToUpper(method)
	urlRes, err := URL.Parse(url)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, urlRes.String(), body)
	for k, v := range header {
		for _, v := range v {
			req.Header.Add(k, v)
		}
	}

	if _, ok := header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", C.UA)
	}
	req.Header.Set("X-Version", x.VERSION)
	req.Header.Set("X-UUID", x.MachineData.PlatformUUID+"-"+x.MachineData.BoardSerialNumber+"-L")

	if err != nil {
		return nil, err
	}

	if user := urlRes.User; user != nil {
		password, _ := user.Password()
		req.SetBasicAuth(user.Username(), password)
	}

	req = req.WithContext(ctx)

	transport := &http.Transport{
		// from http.DefaultTransport
		DisableKeepAlives:     runtime.GOOS == "android",
		MaxIdleConns:          100,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 3 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if conn, err := inner.HandleTcp(address, specialProxy); err == nil {
				return conn, nil
			} else {
				d := net.Dialer{
					Timeout:   30 * time.Second, // 增加TCP连接超时时间
					KeepAlive: 60 * time.Second, // 增加KeepAlive时间
				}
				return d.DialContext(ctx, network, address)
			}
		},
		TLSClientConfig: ca.GetGlobalTLSConfig(&tls.Config{}),
	}

	client := http.Client{
		Transport: transport,
		Timeout:   60 * time.Second, // 增加请求的整体超时时间
	}
	return client.Do(req)
}

func HttpRequestWithBetterCloudflare(ctx context.Context, url, method string, header map[string][]string, body io.Reader, specialProxy string) (*http.Response, error) {
	if !strings.Contains(url, "aider.email") && !strings.Contains(url, "aider.host") && !strings.Contains(url, "iseek.icu") {
		return HttpRequestWithProxy(ctx, url, method, header, body, specialProxy)
	}
	method = strings.ToUpper(method)
	urlRes, err := URL.Parse(url)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, urlRes.String(), body)
	for k, v := range header {
		for _, v := range v {
			req.Header.Add(k, v)
		}
	}

	if _, ok := header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", C.UA)
	}
	req.Header.Set("X-Version", x.VERSION)
	req.Header.Set("X-UUID", x.MachineData.PlatformUUID+"-"+x.MachineData.BoardSerialNumber+"-L")

	if err != nil {
		return nil, err
	}

	if user := urlRes.User; user != nil {
		password, _ := user.Password()
		req.SetBasicAuth(user.Username(), password)
	}

	req = req.WithContext(ctx)

	return cloudflare.ExecuteCloudflareRequest(req)
}
