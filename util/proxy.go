package util

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/net/proxy"
)

// GetProxyDialer 获取代理拨号器(用于WebSocket)
func GetProxyDialer() (proxy.Dialer, error) {
	proxyURL := getProxyURL()
	if proxyURL == "" {
		// 没有代理配置,使用直连
		return proxy.Direct, nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		log.Printf("⚠️ 解析代理URL失败: %v，使用直连", err)
		return proxy.Direct, nil
	}

	if parsedURL.Scheme == "socks5" {
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
		if err != nil {
			log.Printf("⚠️ 创建SOCKS5代理失败: %v，使用直连", err)
			return proxy.Direct, err
		}
		log.Printf("✓ WebSocket已配置SOCKS5代理: %s", parsedURL.Host)
		return dialer, nil
	}

	// HTTP代理不能直接用于WebSocket,返回错误
	log.Printf("⚠️ WebSocket暂不支持HTTP代理,仅支持SOCKS5代理")
	return proxy.Direct, nil
}

// CreateHTTPClientWithProxy 创建支持SOCKS5代理的HTTP客户端
func CreateHTTPClientWithProxy() *http.Client {
	proxyURL := getProxyURL()

	// 如果没有配置代理，使用默认HTTP客户端
	if proxyURL == "" {
		log.Printf("ℹ️ 未检测到代理配置，使用直连")
		return &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// 解析代理URL
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		log.Printf("⚠️ 解析代理URL失败: %v，使用直连", err)
		return &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	log.Printf("🔧 检测到代理配置: %s://%s", parsedURL.Scheme, parsedURL.Host)

	// 根据代理类型创建不同的客户端
	if parsedURL.Scheme == "socks5" {
		// 创建SOCKS5代理拨号器
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
		if err != nil {
			log.Printf("⚠️ 创建SOCKS5代理失败: %v，使用直连", err)
			return &http.Client{
				Timeout: 30 * time.Second,
			}
		}

		// 创建自定义Transport
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			TLSHandshakeTimeout:   60 * time.Second, // 增加到60秒以应对慢速代理
			ResponseHeaderTimeout: 60 * time.Second, // 增加到60秒
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       90 * time.Second,  // 连接空闲超时
			MaxIdleConns:          100,               // 最大空闲连接数
			MaxIdleConnsPerHost:   10,                // 每个主机最大空闲连接
		}

		log.Printf("✓ 已配置SOCKS5代理: %s", parsedURL.Host)
		return &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
	} else if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
		// HTTP/HTTPS代理
		transport := &http.Transport{
			Proxy:                 http.ProxyURL(parsedURL),
			TLSHandshakeTimeout:   60 * time.Second, // 增加到60秒以应对慢速代理
			ResponseHeaderTimeout: 60 * time.Second, // 增加到60秒
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       90 * time.Second,  // 连接空闲超时
			MaxIdleConns:          100,               // 最大空闲连接数
			MaxIdleConnsPerHost:   10,                // 每个主机最大空闲连接
		}

		log.Printf("✓ 已配置HTTP代理: %s", parsedURL.Host)
		return &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
	}

	// 未知代理类型，使用直连
	log.Printf("⚠️ 未知的代理类型: %s，使用直连", parsedURL.Scheme)
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

// getProxyURL 获取代理URL(优先级: ALL_PROXY > HTTPS_PROXY > HTTP_PROXY)
func getProxyURL() string {
	// 检查是否配置了代理(优先级: ALL_PROXY > HTTPS_PROXY > HTTP_PROXY)
	proxyURL := os.Getenv("ALL_PROXY")
	if proxyURL == "" {
		proxyURL = os.Getenv("all_proxy")
	}
	if proxyURL == "" {
		proxyURL = os.Getenv("HTTPS_PROXY")
	}
	if proxyURL == "" {
		proxyURL = os.Getenv("https_proxy")
	}
	if proxyURL == "" {
		proxyURL = os.Getenv("HTTP_PROXY")
	}
	if proxyURL == "" {
		proxyURL = os.Getenv("http_proxy")
	}
	return proxyURL
}
