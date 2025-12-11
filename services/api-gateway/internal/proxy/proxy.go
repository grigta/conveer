package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/conveer/conveer/pkg/config"
	"github.com/conveer/conveer/pkg/logger"
	"github.com/gin-gonic/gin"
)

type ProxyClient struct {
	config     *config.Config
	httpClient *http.Client
}

func NewProxyClient(cfg *config.Config) *ProxyClient {
	return &ProxyClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (p *ProxyClient) ProxyToService(c *gin.Context, serviceURL, path string) {
	targetURL, err := p.buildTargetURL(serviceURL, path, c.Request.URL.RawQuery)
	if err != nil {
		logger.Error("Failed to build target URL",
			logger.Field{Key: "error", Value: err.Error()},
			logger.Field{Key: "service", Value: serviceURL},
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	proxyReq, err := p.createProxyRequest(c, targetURL)
	if err != nil {
		logger.Error("Failed to create proxy request",
			logger.Field{Key: "error", Value: err.Error()},
			logger.Field{Key: "url", Value: targetURL},
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	p.copyHeaders(c.Request.Header, proxyReq.Header)

	proxyReq.Header.Set("X-Forwarded-For", c.ClientIP())
	proxyReq.Header.Set("X-Forwarded-Host", c.Request.Host)
	proxyReq.Header.Set("X-Real-IP", c.ClientIP())
	proxyReq.Header.Set("X-Request-ID", generateRequestID())

	resp, err := p.httpClient.Do(proxyReq)
	if err != nil {
		logger.Error("Failed to execute proxy request",
			logger.Field{Key: "error", Value: err.Error()},
			logger.Field{Key: "url", Value: targetURL},
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Service unavailable"})
		return
	}
	defer resp.Body.Close()

	p.copyResponseHeaders(resp.Header, c.Writer.Header())
	c.Status(resp.StatusCode)

	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		logger.Error("Failed to copy response body",
			logger.Field{Key: "error", Value: err.Error()},
		)
	}
}

func (p *ProxyClient) buildTargetURL(serviceURL, path, query string) (string, error) {
	baseURL, err := url.Parse(serviceURL)
	if err != nil {
		return "", fmt.Errorf("invalid service URL: %w", err)
	}

	targetPath := strings.TrimPrefix(path, "/api/v1")
	targetPath = strings.TrimPrefix(targetPath, "/auth")
	targetPath = strings.TrimPrefix(targetPath, "/users")
	targetPath = strings.TrimPrefix(targetPath, "/products")
	targetPath = strings.TrimPrefix(targetPath, "/orders")
	targetPath = strings.TrimPrefix(targetPath, "/notifications")
	targetPath = strings.TrimPrefix(targetPath, "/analytics")
	targetPath = strings.TrimPrefix(targetPath, "/mail")
	targetPath = strings.TrimPrefix(targetPath, "/max")

	// Mail and Max services expect /api prefix
	if strings.Contains(serviceURL, "mail-service") || strings.Contains(serviceURL, "max-service") {
		if !strings.HasPrefix(targetPath, "/api") {
			targetPath = "/api" + targetPath
		}
	}

	baseURL.Path = targetPath
	baseURL.RawQuery = query

	return baseURL.String(), nil
}

func (p *ProxyClient) createProxyRequest(c *gin.Context, targetURL string) (*http.Request, error) {
	var body []byte
	var err error

	if c.Request.Body != nil {
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	proxyReq, err := http.NewRequestWithContext(
		c.Request.Context(),
		c.Request.Method,
		targetURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return proxyReq, nil
}

func (p *ProxyClient) copyHeaders(src http.Header, dst http.Header) {
	for key, values := range src {
		if shouldForwardHeader(key) {
			for _, value := range values {
				dst.Add(key, value)
			}
		}
	}
}

func (p *ProxyClient) copyResponseHeaders(src http.Header, dst http.Header) {
	for key, values := range src {
		if shouldForwardResponseHeader(key) {
			dst.Del(key)
			for _, value := range values {
				dst.Add(key, value)
			}
		}
	}
}

func shouldForwardHeader(key string) bool {
	hopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	key = strings.ToLower(key)
	for _, header := range hopHeaders {
		if strings.ToLower(header) == key {
			return false
		}
	}
	return true
}

func shouldForwardResponseHeader(key string) bool {
	hopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Transfer-Encoding",
	}

	key = strings.ToLower(key)
	for _, header := range hopHeaders {
		if strings.ToLower(header) == key {
			return false
		}
	}
	return true
}

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (p *ProxyClient) ProxyWebSocket(c *gin.Context, serviceURL string, path string) {
	logger.Warn("WebSocket proxy not implemented",
		logger.Field{Key: "service", Value: serviceURL},
		logger.Field{Key: "path", Value: path},
	)
	c.JSON(http.StatusNotImplemented, gin.H{"error": "WebSocket not supported"})
}