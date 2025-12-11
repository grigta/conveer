package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"conveer/pkg/logger"

	"github.com/playwright-community/playwright-go"
)

type BrowserConfig struct {
	PoolSize       int
	Headless       bool
	UserDataDir    string
	DefaultTimeout time.Duration
}

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
	config     *BrowserConfig
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

func NewBrowserManager(config *BrowserConfig, metrics MetricsCollector, logger logger.Logger) BrowserManager {
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
	// Try to find an available browser with matching proxy
	m.poolMu.Lock()
	defer m.poolMu.Unlock()

	for _, instance := range m.pool {
		if !instance.InUse {
			// Check if proxy matches (if specified)
			if proxyConfig == nil || proxyConfig.Server == "" || instance.ProxyURL == proxyConfig.Server {
				instance.InUse = true

				// Create new context with specific configuration
				contextOptions := playwright.BrowserNewContextOptions{
					AcceptDownloads: playwright.Bool(false),
					IgnoreHTTPSErrors: playwright.Bool(true),
				}

				context, err := instance.Browser.NewContext(contextOptions)
				if err != nil {
					instance.InUse = false
					return nil, nil, fmt.Errorf("failed to create browser context: %w", err)
				}

				// Set default timeout
				if m.config.DefaultTimeout > 0 {
					context.SetDefaultTimeout(float64(m.config.DefaultTimeout.Milliseconds()))
				}

				m.logger.Debug("Browser acquired from pool", "proxy", instance.ProxyURL)
				return instance.Browser, context, nil
			}
		}
	}

	// No available browser with matching proxy, create new one
	if len(m.pool) < m.config.PoolSize*2 { // Allow temporary expansion
		if err := m.createBrowserInstance(proxyConfig); err != nil {
			return nil, nil, fmt.Errorf("failed to create new browser instance: %w", err)
		}

		// Get the newly created browser
		newInstance := m.pool[len(m.pool)-1]
		newInstance.InUse = true

		contextOptions := playwright.BrowserNewContextOptions{
			AcceptDownloads: playwright.Bool(false),
			IgnoreHTTPSErrors: playwright.Bool(true),
		}

		context, err := newInstance.Browser.NewContext(contextOptions)
		if err != nil {
			newInstance.InUse = false
			return nil, nil, fmt.Errorf("failed to create browser context: %w", err)
		}

		if m.config.DefaultTimeout > 0 {
			context.SetDefaultTimeout(float64(m.config.DefaultTimeout.Milliseconds()))
		}

		m.logger.Debug("New browser created and acquired", "proxy", newInstance.ProxyURL)
		return newInstance.Browser, context, nil
	}

	return nil, nil, fmt.Errorf("no available browsers in pool")
}

func (m *browserManager) ReleaseBrowser(browser playwright.Browser) error {
	m.poolMu.Lock()
	defer m.poolMu.Unlock()

	for _, instance := range m.pool {
		if instance.Browser == browser {
			instance.InUse = false
			m.logger.Debug("Browser released to pool", "proxy", instance.ProxyURL)
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
			m.cleanupStale()
		}
	}
}

func (m *browserManager) cleanupStale() {
	m.poolMu.Lock()
	defer m.poolMu.Unlock()

	now := time.Now()
	var toRemove []int

	for i, instance := range m.pool {
		// Remove browsers older than 30 minutes that are not in use
		if !instance.InUse && now.Sub(instance.CreatedAt) > 30*time.Minute {
			if err := instance.Browser.Close(); err != nil {
				m.logger.Error("Failed to close stale browser", "error", err)
			}
			toRemove = append(toRemove, i)
		}
	}

	// Remove from pool
	for i := len(toRemove) - 1; i >= 0; i-- {
		idx := toRemove[i]
		m.pool = append(m.pool[:idx], m.pool[idx+1:]...)
	}

	if len(toRemove) > 0 {
		m.logger.Info("Cleaned up stale browsers", "removed", len(toRemove))
	}

	// Ensure minimum pool size
	initialSize := len(m.pool)
	for len(m.pool) < m.config.PoolSize {
		if err := m.createBrowserInstance(nil); err != nil {
			m.logger.Error("Failed to replenish browser pool", "error", err)
			break
		}
	}

	// Update metrics if pool size changed
	if m.metrics != nil && len(m.pool) != initialSize {
		m.metrics.UpdateBrowserPoolSize(len(m.pool))
	}
}

func (m *browserManager) Shutdown(ctx context.Context) error {
	close(m.shutdownCh)

	m.poolMu.Lock()
	defer m.poolMu.Unlock()

	var errors []error

	// Close all browsers
	for _, instance := range m.pool {
		if err := instance.Browser.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close browser: %w", err))
			m.logger.Error("Failed to close browser", "error", err)
		}
	}

	// Stop playwright
	if m.pw != nil {
		if err := m.pw.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop playwright: %w", err))
		}
	}

	m.pool = nil

	// Update metrics to reflect empty pool
	if m.metrics != nil {
		m.metrics.UpdateBrowserPoolSize(0)
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	m.logger.Info("Browser manager shut down successfully")
	return nil
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
