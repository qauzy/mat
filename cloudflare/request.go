package cloudflare

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/http/cookiejar"
	"time"
)

func BuildCloudflareClient(ip *net.IPAddr) *http.Client {

	transport := &http.Transport{
		DialContext:           getCloudflareDialContext(ip),
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       30 * time.Second, // 空闲（keep-alive）连接在关闭之前保持空闲的时长
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 10 * time.Second,
		DisableKeepAlives:     false,
		MaxIdleConns:          512,
		MaxIdleConnsPerHost:   256,
	}
	// Cookie handle
	jar, _ := cookiejar.New(nil)

	return &http.Client{
		Timeout:   time.Second * 15,
		Transport: transport,
		Jar:       jar,
	}
}

func getCloudflareDialContext(ip *net.IPAddr) func(ctx context.Context, network, address string) (net.Conn, error) {
	var fakeSourceAddr string
	if isIPv4(ip.String()) {
		fakeSourceAddr = fmt.Sprintf("%s:%d", ip.String(), TCPPort)
	} else {
		fakeSourceAddr = fmt.Sprintf("[%s]:%d", ip.String(), TCPPort)
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, fakeSourceAddr)
	}
}

func ExecuteCloudflareRequest(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	ips := loadIPRanges()
	for _, ip := range ips {
		logrus.Infof("[provider] try cloudflare remote host %s", ip.String())
		client := BuildCloudflareClient(ip)
		resp, err := client.Do(req)
		if err == nil {
			logrus.Infof("[provider] try %s with cloudflare remote host %s success", fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.Host, req.URL.Path), ip.String())
			return resp, nil
		}
	}

	return nil, fmt.Errorf("try all cloudflare host failed")
}
