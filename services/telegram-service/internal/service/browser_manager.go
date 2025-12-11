package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/services/telegram-service/internal/models"

	"github.com/playwright-community/playwright-go"
)

type BrowserInstance struct {
	Browser   playwright.Browser
	InUse     bool
	CreatedAt time.Time
	ProxyURL  string
}

type BrowserManager interface {
	Initialize(ctx context.Context) error
	AcquireBrowser(ctx context.Context, proxyConfig *ProxyConfig) (playwright.Browser, playwright.BrowserContext, error)
	ReleaseBrowser(browser playwright.Browser) error
	Shutdown(ctx context.Context) error
	GetPoolStats() PoolStats
}

type browserManager struct {
	pw         *playwright.Playwright
	config     *models.BrowserConfig
	pool       []*BrowserInstance
	poolMu     sync.RWMutex
	logger     logger.Logger
	metrics    MetricsCollector
	shutdownCh chan struct{}
}

type ProxyConfig struct {
	Server   string
	Username string
	Password string
	Bypass   string
}

type PoolStats struct {
	TotalBrowsers    int
	AvailableBrowsers int
	InUseBrowsers    int
}

func NewBrowserManager(config *models.BrowserConfig, metrics MetricsCollector, logger logger.Logger) BrowserManager {
	return &browserManager{
		config:     config,
		pool:       make([]*BrowserInstance, 0, config.PoolSize),
		logger:     logger,
		metrics:    metrics,
		shutdownCh: make(chan struct{}),
	}
}

func (m *browserManager) Initialize(ctx context.Context) error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("failed to start playwright: %w", err)
	}
	m.pw = pw

	// Create initial browser pool
	for i := 0; i < m.config.PoolSize; i++ {
		if err := m.createBrowserInstance(nil); err != nil {
			m.logger.Error("Failed to create browser instance", "index", i, "error", err)
			// Continue with partial pool
		}
	}

	m.logger.Info("Browser manager initialized", "pool_size", len(m.pool))

	// Update metrics if available
	if m.metrics != nil {
		m.metrics.UpdateBrowserPoolSize(len(m.pool))
	}

	// Start pool maintenance goroutine
	go m.maintainPool(ctx)

	return nil
}

func (m *browserManager) createBrowserInstance(proxyConfig *ProxyConfig) error {
	launchOptions := playwright.BrowserTypeLaunchOptions{
		Headless: &m.config.Headless,
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
			"--disable-web-security",
			"--disable-features=IsolateOrigins,site-per-process",
			"--disable-setuid-sandbox",
			"--disable-accelerated-2d-canvas",
			"--no-first-run",
			"--no-zygote",
			"--disable-gpu",
			"--window-size=1920,1080",
			"--start-maximized",
		},
	}

	if proxyConfig != nil && proxyConfig.Server != "" {
		launchOptions.Proxy = &playwright.Proxy{
			Server: proxyConfig.Server,
		}
		if proxyConfig.Username != "" {
			launchOptions.Proxy.Username = &proxyConfig.Username
		}
		if proxyConfig.Password != "" {
			launchOptions.Proxy.Password = &proxyConfig.Password
		}
		if proxyConfig.Bypass != "" {
			launchOptions.Proxy.Bypass = &proxyConfig.Bypass
		}
	}

	browser, err := m.pw.Chromium.Launch(launchOptions)
	if err != nil {
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	instance := &BrowserInstance{
		Browser:   browser,
		InUse:     false,
		CreatedAt: time.Now(),
	}

	if proxyConfig != nil {
		instance.ProxyURL = proxyConfig.Server
	}

	m.poolMu.Lock()
	m.pool = append(m.pool, instance)
	m.poolMu.Unlock()

	return nil
}

func (m *browserManager) AcquireBrowser(ctx context.Context, proxyConfig *ProxyConfig) (playwright.Browser, playwright.BrowserContext, error) {
	m.poolMu.Lock()
	defer m.poolMu.Unlock()

	// Try to find an available browser with matching proxy
	for _, instance := range m.pool {
		if !instance.InUse && instance.ProxyURL == proxyConfig.Server {
			instance.InUse = true

			// Create new context for the browser
			contextOptions := playwright.BrowserNewContextOptions{
				Viewport: &playwright.Size{
					Width:  1920,
					Height: 1080,
				},
				UserAgent: playwright.String(generateUserAgent()),
				Locale:    playwright.String("en-US"),
				TimezoneID: playwright.String("America/New_York"),
			}

			context, err := instance.Browser.NewContext(contextOptions)
			if err != nil {
				instance.InUse = false
				return nil, nil, fmt.Errorf("failed to create browser context: %w", err)
			}

			if m.metrics != nil {
				m.metrics.IncrementBrowserAcquisitions()
			}

			return instance.Browser, context, nil
		}
	}

	// No available browser found, try to create a new one if pool not full
	if len(m.pool) < m.config.PoolSize {
		if err := m.createBrowserInstance(proxyConfig); err != nil {
			return nil, nil, fmt.Errorf("failed to create new browser instance: %w", err)
		}
		// Retry acquisition with the newly created browser
		return m.AcquireBrowser(ctx, proxyConfig)
	}

	return nil, nil, fmt.Errorf("no available browsers in pool")
}

func (m *browserManager) ReleaseBrowser(browser playwright.Browser) error {
	m.poolMu.Lock()
	defer m.poolMu.Unlock()

	for _, instance := range m.pool {
		if instance.Browser == browser {
			instance.InUse = false
			if m.metrics != nil {
				m.metrics.IncrementBrowserReleases()
			}
			return nil
		}
	}

	return fmt.Errorf("browser not found in pool")
}

func (m *browserManager) maintainPool(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.shutdownCh:
			return
		case <-ticker.C:
			m.cleanupStaleInstances()
			m.ensureMinimumPoolSize()
		}
	}
}

func (m *browserManager) cleanupStaleInstances() {
	m.poolMu.Lock()
	defer m.poolMu.Unlock()

	maxAge := 30 * time.Minute
	now := time.Now()

	var activePool []*BrowserInstance
	for _, instance := range m.pool {
		if !instance.InUse && now.Sub(instance.CreatedAt) > maxAge {
			// Close and remove stale browser
			if err := instance.Browser.Close(); err != nil {
				m.logger.Error("Failed to close stale browser", "error", err)
			}
		} else {
			activePool = append(activePool, instance)
		}
	}

	m.pool = activePool
}

func (m *browserManager) ensureMinimumPoolSize() {
	m.poolMu.RLock()
	currentSize := len(m.pool)
	m.poolMu.RUnlock()

	minSize := m.config.PoolSize / 2
	if minSize < 1 {
		minSize = 1
	}

	if currentSize < minSize {
		toCreate := minSize - currentSize
		for i := 0; i < toCreate; i++ {
			if err := m.createBrowserInstance(nil); err != nil {
				m.logger.Error("Failed to create browser instance during maintenance", "error", err)
			}
		}
	}
}

func (m *browserManager) GetPoolStats() PoolStats {
	m.poolMu.RLock()
	defer m.poolMu.RUnlock()

	stats := PoolStats{
		TotalBrowsers: len(m.pool),
	}

	for _, instance := range m.pool {
		if instance.InUse {
			stats.InUseBrowsers++
		} else {
			stats.AvailableBrowsers++
		}
	}

	return stats
}

func (m *browserManager) Shutdown(ctx context.Context) error {
	close(m.shutdownCh)

	m.poolMu.Lock()
	defer m.poolMu.Unlock()

	for _, instance := range m.pool {
		if err := instance.Browser.Close(); err != nil {
			m.logger.Error("Failed to close browser", "error", err)
		}
	}

	if m.pw != nil {
		if err := m.pw.Stop(); err != nil {
			return fmt.Errorf("failed to stop playwright: %w", err)
		}
	}

	m.logger.Info("Browser manager shutdown complete")
	return nil
}

func generateUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	// Simple random selection
	return userAgents[time.Now().UnixNano()%int64(len(userAgents))]
}
