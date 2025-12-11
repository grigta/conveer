package service

import (
	"fmt"

	"github.com/playwright-community/playwright-go"
)

type StealthInjector interface {
	InjectStealth(page playwright.Page) error
	DisableWebdriverFlag(page playwright.Page) error
	OverrideNavigatorProperties(page playwright.Page) error
}

type stealthInjector struct{}

func NewStealthInjector() StealthInjector {
	return &stealthInjector{}
}

func (s *stealthInjector) InjectStealth(page playwright.Page) error {
	// Inject all stealth modifications
	if err := s.DisableWebdriverFlag(page); err != nil {
		return err
	}

	if err := s.OverrideNavigatorProperties(page); err != nil {
		return err
	}

	if err := s.hideAutomationIndicators(page); err != nil {
		return err
	}

	if err := s.patchChromeRuntime(page); err != nil {
		return err
	}

	return nil
}

func (s *stealthInjector) DisableWebdriverFlag(page playwright.Page) error {
	script := `
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined
		});

		// Remove automation extension
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => {
			if (parameters.name === 'notifications') {
				return Promise.resolve({ state: Notification.permission });
			}
			return originalQuery(parameters);
		};
	`

	_, err := page.AddInitScript(playwright.Script{Content: &script})
	return err
}

func (s *stealthInjector) OverrideNavigatorProperties(page playwright.Page) error {
	script := `
		// Override navigator.plugins
		Object.defineProperty(navigator, 'plugins', {
			get: () => {
				const arr = [
					{
						description: "Portable Document Format",
						filename: "internal-pdf-viewer",
						length: 1,
						name: "Chrome PDF Plugin"
					},
					{
						description: "Portable Document Format",
						filename: "mhjfbmdgcfjbbpaeojofohoefgiehjai",
						length: 1,
						name: "Chrome PDF Viewer"
					},
					{
						description: "Native Client Executable",
						filename: "internal-nacl-plugin",
						length: 2,
						name: "Native Client"
					}
				];
				arr.item = i => arr[i];
				arr.namedItem = name => arr.find(p => p.name === name);
				arr.refresh = () => {};
				return arr;
			}
		});

		// Override navigator.languages
		Object.defineProperty(navigator, 'languages', {
			get: () => ['en-US', 'en']
		});

		// Override navigator.platform
		const platform = navigator.platform;
		Object.defineProperty(navigator, 'platform', {
			get: () => platform || 'Win32'
		});

		// Override navigator.hardwareConcurrency
		Object.defineProperty(navigator, 'hardwareConcurrency', {
			get: () => 4 + Math.floor(Math.random() * 4)
		});

		// Override navigator.deviceMemory
		Object.defineProperty(navigator, 'deviceMemory', {
			get: () => [4, 8, 16][Math.floor(Math.random() * 3)]
		});
	`

	_, err := page.AddInitScript(playwright.Script{Content: &script})
	return err
}

func (s *stealthInjector) hideAutomationIndicators(page playwright.Page) error {
	script := `
		// Remove Chrome automation extension
		const originalToString = Function.prototype.toString;
		Function.prototype.toString = function() {
			if (this === window.navigator.permissions.query) {
				return 'function query() { [native code] }';
			}
			return originalToString.call(this);
		};

		// Hide Chrome command line flag
		Object.defineProperty(navigator, 'webdriver', {
			get: () => false
		});

		// Remove headless indicator
		Object.defineProperty(navigator, 'headless', {
			get: () => false
		});

		// Mock chrome object
		if (!window.chrome) {
			window.chrome = {
				runtime: {
					connect: () => {},
					sendMessage: () => {},
					onMessage: { addListener: () => {} }
				},
				loadTimes: () => ({
					requestTime: Date.now() / 1000,
					startLoadTime: Date.now() / 1000,
					commitLoadTime: Date.now() / 1000,
					finishDocumentLoadTime: Date.now() / 1000,
					finishLoadTime: Date.now() / 1000,
					firstPaintTime: Date.now() / 1000,
					firstPaintAfterLoadTime: 0,
					navigationType: "Other",
					wasFetchedViaSpdy: false,
					wasNpnNegotiated: true,
					npnNegotiatedProtocol: "h2",
					wasAlternateProtocolAvailable: false,
					connectionInfo: "h2"
				}),
				csi: () => ({
					onloadT: Date.now(),
					pageT: Date.now() - Math.random() * 1000,
					startE: Date.now() - Math.random() * 2000,
					tran: Math.floor(Math.random() * 20)
				}),
				app: {
					isInstalled: false,
					getDetails: () => null,
					getIsInstalled: () => false,
					runningState: () => 'cannot_run'
				}
			};
		}
	`

	_, err := page.AddInitScript(playwright.Script{Content: &script})
	return err
}

func (s *stealthInjector) patchChromeRuntime(page playwright.Page) error {
	script := `
		// Patch chrome.runtime if it exists
		if (window.chrome && window.chrome.runtime) {
			const originalRuntime = window.chrome.runtime;

			window.chrome.runtime = new Proxy(originalRuntime, {
				get(target, prop) {
					if (prop === 'id') {
						return undefined;
					}
					return target[prop];
				}
			});
		}

		// Override Permissions API
		const originalQuery = window.navigator.permissions ? window.navigator.permissions.query : undefined;
		if (originalQuery) {
			window.navigator.permissions.query = (parameters) => {
				if (parameters.name === 'notifications') {
					return Promise.resolve({ state: Notification.permission });
				}
				return originalQuery.call(window.navigator.permissions, parameters);
			};
		}

		// WebGL vendor spoofing
		const getParameter = WebGLRenderingContext.prototype.getParameter;
		WebGLRenderingContext.prototype.getParameter = function(parameter) {
			if (parameter === 37445) {
				return 'Intel Inc.';
			}
			if (parameter === 37446) {
				return 'Intel Iris OpenGL Engine';
			}
			return getParameter.call(this, parameter);
		};

		const getParameter2 = WebGL2RenderingContext.prototype.getParameter;
		WebGL2RenderingContext.prototype.getParameter = function(parameter) {
			if (parameter === 37445) {
				return 'Intel Inc.';
			}
			if (parameter === 37446) {
				return 'Intel Iris OpenGL Engine';
			}
			return getParameter2.call(this, parameter);
		};
	`

	_, err := page.AddInitScript(playwright.Script{Content: &script})
	if err != nil {
		return fmt.Errorf("failed to patch chrome runtime: %w", err)
	}

	return nil
}