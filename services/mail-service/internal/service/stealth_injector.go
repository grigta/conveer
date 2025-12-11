package service

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/playwright-community/playwright-go"
)

// InjectStealth injects stealth JavaScript to avoid detection
func InjectStealth(page playwright.Page) error {
	stealthJS := `
		// Override navigator.webdriver
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined
		});

		// Override chrome runtime
		if (!window.chrome) {
			window.chrome = {};
		}
		window.chrome.runtime = {
			connect: () => {},
			sendMessage: () => {},
			onMessage: {
				addListener: () => {}
			}
		};

		// Override permissions
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' ?
				Promise.resolve({ state: Notification.permission }) :
				originalQuery(parameters)
		);

		// Override plugins
		Object.defineProperty(navigator, 'plugins', {
			get: () => [
				{ name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer' },
				{ name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai' },
				{ name: 'Native Client', filename: 'internal-nacl-plugin' }
			]
		});

		// Override languages
		Object.defineProperty(navigator, 'languages', {
			get: () => ['en-US', 'en', 'ru-RU', 'ru']
		});

		// Canvas fingerprint randomization
		const originalToDataURL = HTMLCanvasElement.prototype.toDataURL;
		HTMLCanvasElement.prototype.toDataURL = function(type) {
			if (type === 'image/png' && this.width === 280 && this.height === 60) {
				const canvas = document.createElement('canvas');
				canvas.width = this.width;
				canvas.height = this.height;
				const ctx = canvas.getContext('2d');
				ctx.drawImage(this, 0, 0);

				// Add random noise
				const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
				for (let i = 0; i < imageData.data.length; i += 4) {
					imageData.data[i] = imageData.data[i] ^ Math.floor(Math.random() * 10);
				}
				ctx.putImageData(imageData, 0, 0);

				return originalToDataURL.apply(canvas, arguments);
			}
			return originalToDataURL.apply(this, arguments);
		};

		// WebGL fingerprint randomization
		const getParameter = WebGLRenderingContext.prototype.getParameter;
		WebGLRenderingContext.prototype.getParameter = function(parameter) {
			if (parameter === 37445) {
				return 'Intel Inc.';
			}
			if (parameter === 37446) {
				return 'Intel Iris OpenGL Engine';
			}
			return getParameter.apply(this, arguments);
		};
	`

	if err := page.AddInitScript(playwright.Script{
		Content: playwright.String(stealthJS),
	}); err != nil {
		return fmt.Errorf("failed to inject stealth script: %w", err)
	}

	return nil
}

// EmulateHumanBehavior adds random mouse movements and scrolling
func EmulateHumanBehavior(page playwright.Page) error {
	// Random mouse movements
	for i := 0; i < 3; i++ {
		x := float64(rand.Intn(800) + 100)
		y := float64(rand.Intn(600) + 100)

		if err := page.Mouse().Move(x, y); err != nil {
			return fmt.Errorf("failed to move mouse: %w", err)
		}

		time.Sleep(time.Duration(rand.Intn(500)+200) * time.Millisecond)
	}

	// Random scroll
	scrollScript := `
		window.scrollTo({
			top: Math.random() * 500,
			left: 0,
			behavior: 'smooth'
		});
	`

	if _, err := page.Evaluate(scrollScript); err != nil {
		return fmt.Errorf("failed to scroll: %w", err)
	}

	return nil
}

// TypeWithHumanSpeed types text with random delays
func TypeWithHumanSpeed(page playwright.Page, selector string, text string) error {
	// Click on the element first
	if err := page.Click(selector); err != nil {
		return fmt.Errorf("failed to click element: %w", err)
	}

	// Clear existing text
	if err := page.Fill(selector, ""); err != nil {
		return fmt.Errorf("failed to clear field: %w", err)
	}

	// Type each character with random delay
	for _, char := range text {
		if err := page.Type(selector, string(char)); err != nil {
			return fmt.Errorf("failed to type character: %w", err)
		}

		// Random delay between 50-200ms
		delay := rand.Intn(150) + 50
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	return nil
}

// RandomDelay generates a random sleep duration
func RandomDelay(minMs, maxMs int) time.Duration {
	if minMs >= maxMs {
		return time.Duration(minMs) * time.Millisecond
	}

	delay := rand.Intn(maxMs-minMs) + minMs
	return time.Duration(delay) * time.Millisecond
}