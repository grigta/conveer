package service

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Fingerprint represents browser fingerprint
type Fingerprint struct {
	UserAgent      string   `json:"user_agent"`
	ViewportWidth  int      `json:"viewport_width"`
	ViewportHeight int      `json:"viewport_height"`
	Timezone       string   `json:"timezone"`
	Locale         string   `json:"locale"`
	Platform       string   `json:"platform"`
	ScreenWidth    int      `json:"screen_width"`
	ScreenHeight   int      `json:"screen_height"`
	WebGLVendor    string   `json:"webgl_vendor"`
	WebGLRenderer  string   `json:"webgl_renderer"`
	Languages      []string `json:"languages"`
}

// GenerateFingerprint creates a realistic browser fingerprint
func GenerateFingerprint() Fingerprint {
	rand.Seed(time.Now().UnixNano())

	// Chrome versions
	chromeVersions := []string{"110", "111", "112", "113", "114", "115", "116", "117", "118", "119", "120"}
	chromeVersion := chromeVersions[rand.Intn(len(chromeVersions))]

	// Platforms
	platforms := []struct {
		name      string
		userAgent string
	}{
		{
			"Windows",
			fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", chromeVersion),
		},
		{
			"MacOS",
			fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", chromeVersion),
		},
		{
			"Linux",
			fmt.Sprintf("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36", chromeVersion),
		},
	}

	platform := platforms[rand.Intn(len(platforms))]

	// Screen resolutions
	resolutions := []struct {
		width  int
		height int
	}{
		{1920, 1080},
		{1366, 768},
		{1536, 864},
		{1440, 900},
		{1280, 720},
		{2560, 1440},
	}

	resolution := resolutions[rand.Intn(len(resolutions))]

	// Viewport (slightly smaller than screen)
	viewportWidth := resolution.width - rand.Intn(100)
	viewportHeight := resolution.height - rand.Intn(200) - 100

	// Timezones
	timezones := []string{
		"Europe/Moscow",
		"Europe/Minsk",
		"Europe/Kiev",
		"Asia/Yekaterinburg",
		"Asia/Novosibirsk",
	}

	// Locales
	locales := []string{
		"ru-RU",
		"en-US",
		"ru",
	}

	// WebGL vendors and renderers
	webglVendors := []struct {
		vendor   string
		renderer string
	}{
		{"Intel Inc.", "Intel Iris OpenGL Engine"},
		{"Google Inc. (NVIDIA)", "ANGLE (NVIDIA GeForce GTX 1050 Ti Direct3D11 vs_5_0 ps_5_0)"},
		{"Google Inc. (AMD)", "ANGLE (AMD Radeon RX 580 Direct3D11 vs_5_0 ps_5_0)"},
		{"Google Inc. (Intel)", "ANGLE (Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0)"},
	}

	webgl := webglVendors[rand.Intn(len(webglVendors))]

	return Fingerprint{
		UserAgent:      platform.userAgent,
		ViewportWidth:  viewportWidth,
		ViewportHeight: viewportHeight,
		Timezone:       timezones[rand.Intn(len(timezones))],
		Locale:         locales[rand.Intn(len(locales))],
		Platform:       platform.name,
		ScreenWidth:    resolution.width,
		ScreenHeight:   resolution.height,
		WebGLVendor:    webgl.vendor,
		WebGLRenderer:  webgl.renderer,
		Languages:      []string{"ru-RU", "ru", "en-US", "en"},
	}
}

// ApplyFingerprint applies fingerprint to browser context
func ApplyFingerprint(page playwright.Page, fingerprint Fingerprint) error {
	// Inject WebGL overrides
	webglScript := fmt.Sprintf(`
		const getParameter = WebGLRenderingContext.prototype.getParameter;
		WebGLRenderingContext.prototype.getParameter = function(parameter) {
			if (parameter === 37445) {
				return '%s';
			}
			if (parameter === 37446) {
				return '%s';
			}
			return getParameter.apply(this, arguments);
		};

		const getParameter2 = WebGL2RenderingContext.prototype.getParameter;
		WebGL2RenderingContext.prototype.getParameter = function(parameter) {
			if (parameter === 37445) {
				return '%s';
			}
			if (parameter === 37446) {
				return '%s';
			}
			return getParameter2.apply(this, arguments);
		};
	`, fingerprint.WebGLVendor, fingerprint.WebGLRenderer, fingerprint.WebGLVendor, fingerprint.WebGLRenderer)

	if err := page.AddInitScript(playwright.Script{
		Content: playwright.String(webglScript),
	}); err != nil {
		return fmt.Errorf("failed to apply WebGL fingerprint: %w", err)
	}

	// Override screen dimensions
	screenScript := fmt.Sprintf(`
		Object.defineProperty(screen, 'width', { get: () => %d });
		Object.defineProperty(screen, 'height', { get: () => %d });
		Object.defineProperty(screen, 'availWidth', { get: () => %d });
		Object.defineProperty(screen, 'availHeight', { get: () => %d });
	`, fingerprint.ScreenWidth, fingerprint.ScreenHeight, fingerprint.ScreenWidth, fingerprint.ScreenHeight-40)

	if err := page.AddInitScript(playwright.Script{
		Content: playwright.String(screenScript),
	}); err != nil {
		return fmt.Errorf("failed to apply screen fingerprint: %w", err)
	}

	return nil
}