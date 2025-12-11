package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/playwright-community/playwright-go"
)

// BrowserConfig represents browser configuration
type BrowserConfig struct {
	ProxyURL    string
	Fingerprint Fingerprint
	Headless    bool
}

// BrowserManager manages a pool of browser instances
type BrowserManager struct {
	pw        *playwright.Playwright
	pool      []playwright.Browser
	poolMutex sync.Mutex
	poolSize  int
	headless  bool
}

// NewBrowserManager creates a new browser manager
func NewBrowserManager(poolSize int, headless bool) (*BrowserManager, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to start playwright: %w", err)
	}

	manager := &BrowserManager{
		pw:       pw,
		pool:     make([]playwright.Browser, 0, poolSize),
		poolSize: poolSize,
		headless: headless,
	}

	// Initialize pool
	if err := manager.Initialize(); err != nil {
		return nil, err
	}

	return manager, nil
}

// Initialize creates initial browser pool
func (m *BrowserManager) Initialize() error {
	for i := 0; i < m.poolSize; i++ {
		browser, err := m.createBrowser(nil)
		if err != nil {
			return fmt.Errorf("failed to create browser %d: %w", i, err)
		}
		m.pool = append(m.pool, browser)
	}
	return nil
}

// AcquireBrowser gets a browser from pool or creates new one
func (m *BrowserManager) AcquireBrowser(ctx context.Context, config *BrowserConfig) (playwright.Browser, error) {
	m.poolMutex.Lock()
	defer m.poolMutex.Unlock()

	// Try to get from pool
	if len(m.pool) > 0 {
		browser := m.pool[len(m.pool)-1]
		m.pool = m.pool[:len(m.pool)-1]

		// Configure with proxy if needed
		if config != nil && config.ProxyURL != "" {
			// Close and recreate with proxy
			browser.Close()
			return m.createBrowser(config)
		}

		return browser, nil
	}

	// Create new browser
	return m.createBrowser(config)
}

// ReleaseBrowser returns browser to pool
func (m *BrowserManager) ReleaseBrowser(browser playwright.Browser) {
	m.poolMutex.Lock()
	defer m.poolMutex.Unlock()

	// Check pool size
	if len(m.pool) >= m.poolSize {
		// Close excess browser
		browser.Close()
		return
	}

	// Clear browser state
	contexts := browser.Contexts()
	for _, ctx := range contexts {
		ctx.Close()
	}

	m.pool = append(m.pool, browser)
}

// Shutdown closes all browsers
func (m *BrowserManager) Shutdown() error {
	m.poolMutex.Lock()
	defer m.poolMutex.Unlock()

	for _, browser := range m.pool {
		browser.Close()
	}

	if err := m.pw.Stop(); err != nil {
		return fmt.Errorf("failed to stop playwright: %w", err)
	}

	return nil
}

// createBrowser creates a new browser instance
func (m *BrowserManager) createBrowser(config *BrowserConfig) (playwright.Browser, error) {
	opts := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(m.headless),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-web-security",
			"--disable-features=IsolateOrigins,site-per-process",
		},
	}

	if config != nil && config.ProxyURL != "" {
		opts.Proxy = &playwright.Proxy{
			Server: playwright.String(config.ProxyURL),
		}
	}

	browser, err := m.pw.Chromium.Launch(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	// Apply fingerprint if provided
	if config != nil && config.Fingerprint.UserAgent != "" {
		contextOpts := playwright.BrowserNewContextOptions{
			UserAgent: playwright.String(config.Fingerprint.UserAgent),
			Viewport: &playwright.Size{
				Width:  config.Fingerprint.ViewportWidth,
				Height: config.Fingerprint.ViewportHeight,
			},
			Locale:       playwright.String(config.Fingerprint.Locale),
			TimezoneId:   playwright.String(config.Fingerprint.Timezone),
			ExtraHTTPHeaders: map[string]string{
				"Accept-Language": config.Fingerprint.Locale,
			},
		}

		_, err := browser.NewContext(contextOpts)
		if err != nil {
			browser.Close()
			return nil, fmt.Errorf("failed to create context: %w", err)
		}
	}

	return browser, nil
}