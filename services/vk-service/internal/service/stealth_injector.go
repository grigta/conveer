package service

import (
	"fmt"
	"math/rand"
	"time"

	"conveer/pkg/logger"

	"github.com/playwright-community/playwright-go"
)

type StealthInjector interface {
	InjectStealth(page playwright.Page) error
	EmulateHumanBehavior(page playwright.Page) error
	RandomDelay(minMs, maxMs int) time.Duration
	MoveMouseNaturally(page playwright.Page, x, y float64) error
	TypeWithHumanSpeed(element playwright.ElementHandle, text string) error
}

type stealthInjector struct {
	logger logger.Logger
	rand   *rand.Rand
}

func NewStealthInjector(logger logger.Logger) StealthInjector {
	return &stealthInjector{
		logger: logger,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *stealthInjector) InjectStealth(page playwright.Page) error {
	// Override navigator.webdriver
	if err := page.AddInitScript(playwright.Script{
		Content: &navigatorWebdriverOverride,
	}); err != nil {
		return fmt.Errorf("failed to override navigator.webdriver: %w", err)
	}

	// Override navigator.plugins
	if err := page.AddInitScript(playwright.Script{
		Content: &pluginsOverride,
	}); err != nil {
		return fmt.Errorf("failed to override navigator.plugins: %w", err)
	}

	// Override permissions
	if err := page.AddInitScript(playwright.Script{
		Content: &permissionsOverride,
	}); err != nil {
		return fmt.Errorf("failed to override permissions: %w", err)
	}

	// Canvas fingerprint noise
	if err := page.AddInitScript(playwright.Script{
		Content: &canvasNoiseScript,
	}); err != nil {
		return fmt.Errorf("failed to inject canvas noise: %w", err)
	}

	// WebGL fingerprint noise
	if err := page.AddInitScript(playwright.Script{
		Content: &webglNoiseScript,
	}); err != nil {
		return fmt.Errorf("failed to inject WebGL noise: %w", err)
	}

	// Chrome runtime override
	if err := page.AddInitScript(playwright.Script{
		Content: &chromeRuntimeOverride,
	}); err != nil {
		return fmt.Errorf("failed to override chrome runtime: %w", err)
	}

	// Audio context noise
	if err := page.AddInitScript(playwright.Script{
		Content: &audioContextNoise,
	}); err != nil {
		return fmt.Errorf("failed to inject audio context noise: %w", err)
	}

	s.logger.Debug("Stealth scripts injected successfully")
	return nil
}

func (s *stealthInjector) EmulateHumanBehavior(page playwright.Page) error {
	// Random mouse movements
	for i := 0; i < 3; i++ {
		x := s.rand.Float64() * 800
		y := s.rand.Float64() * 600
		if err := s.MoveMouseNaturally(page, x, y); err != nil {
			s.logger.Warn("Failed to move mouse", "error", err)
		}
		time.Sleep(s.RandomDelay(100, 300))
	}

	// Random scrolls
	scrollScript := `
		window.scrollTo({
			top: Math.random() * document.body.scrollHeight * 0.3,
			behavior: 'smooth'
		});
	`
	if _, err := page.Evaluate(scrollScript); err != nil {
		s.logger.Warn("Failed to scroll", "error", err)
	}

	return nil
}

func (s *stealthInjector) RandomDelay(minMs, maxMs int) time.Duration {
	delay := minMs + s.rand.Intn(maxMs-minMs)
	return time.Duration(delay) * time.Millisecond
}

func (s *stealthInjector) MoveMouseNaturally(page playwright.Page, x, y float64) error {
	// Get current mouse position (approximate from last known)
	steps := 5 + s.rand.Intn(10)

	for i := 0; i < steps; i++ {
		// Add slight curve to movement
		progress := float64(i) / float64(steps)
		// Bezier curve for natural movement
		t := progress * progress * (3.0 - 2.0*progress)

		currentX := x * t
		currentY := y * t

		// Add small random jitter
		jitterX := (s.rand.Float64() - 0.5) * 2
		jitterY := (s.rand.Float64() - 0.5) * 2

		if err := page.Mouse().Move(currentX+jitterX, currentY+jitterY); err != nil {
			return fmt.Errorf("failed to move mouse: %w", err)
		}

		time.Sleep(time.Duration(10+s.rand.Intn(20)) * time.Millisecond)
	}

	return page.Mouse().Move(x, y)
}

func (s *stealthInjector) TypeWithHumanSpeed(element playwright.ElementHandle, text string) error {
	for _, char := range text {
		if err := element.Type(string(char)); err != nil {
			return fmt.Errorf("failed to type character: %w", err)
		}

		// Variable typing speed
		baseDelay := 50
		variance := 100
		if s.rand.Float64() < 0.1 { // 10% chance of longer pause
			variance = 300
		}

		delay := baseDelay + s.rand.Intn(variance)
		time.Sleep(time.Duration(delay) * time.Millisecond)

		// Occasional typos and corrections (5% chance)
		if s.rand.Float64() < 0.05 && len(text) > 5 {
			wrongChar := string(rune('a' + s.rand.Intn(26)))
			element.Type(wrongChar)
			time.Sleep(time.Duration(100+s.rand.Intn(200)) * time.Millisecond)
			element.Press("Backspace")
			time.Sleep(time.Duration(50+s.rand.Intn(100)) * time.Millisecond)
		}
	}

	return nil
}

// JavaScript injection scripts
var navigatorWebdriverOverride = `
(() => {
    const newProto = navigator.__proto__;
    delete newProto.webdriver;
    navigator.__proto__ = newProto;

    Object.defineProperty(navigator, 'webdriver', {
        get: () => undefined,
        configurable: true
    });

    // Also handle DocumentFragment for Chrome
    const originalQuery = Document.prototype.querySelector;
    Document.prototype.querySelector = function(...args) {
        if (args[0] === '[webdriver]') return null;
        return originalQuery.apply(this, args);
    };
})();
`

var pluginsOverride = `
(() => {
    const pluginData = [
        {
            name: 'Chrome PDF Plugin',
            filename: 'internal-pdf-viewer',
            description: 'Portable Document Format'
        },
        {
            name: 'Chrome PDF Viewer',
            filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai',
            description: ''
        },
        {
            name: 'Native Client',
            filename: 'internal-nacl-plugin',
            description: ''
        }
    ];

    Object.defineProperty(navigator, 'plugins', {
        get: () => {
            const arr = [];
            pluginData.forEach((p, i) => {
                const plugin = {
                    name: p.name,
                    filename: p.filename,
                    description: p.description,
                    length: 1,
                    item: (i) => ({
                        type: 'application/x-google-chrome-pdf',
                        suffixes: 'pdf',
                        description: 'Portable Document Format',
                        enabledPlugin: plugin
                    }),
                    namedItem: (name) => null,
                    [Symbol.toStringTag]: 'PluginArray'
                };
                arr.push(plugin);
            });
            arr.item = (i) => arr[i];
            arr.namedItem = (name) => arr.find(p => p.name === name);
            arr.refresh = () => {};
            return arr;
        }
    });
})();
`

var permissionsOverride = `
(() => {
    const originalQuery = window.navigator.permissions.query;
    window.navigator.permissions.query = async (parameters) => {
        if (parameters.name === 'notifications') {
            return { state: 'default' };
        }
        return originalQuery(parameters);
    };

    const oldCall = Function.prototype.call;
    Function.prototype.call = function() {
        if (this.toString().indexOf('notifications') !== -1) {
            return 'default';
        }
        return oldCall.apply(this, arguments);
    };
})();
`

var canvasNoiseScript = `
(() => {
    const originalToDataURL = HTMLCanvasElement.prototype.toDataURL;
    const originalToBlob = HTMLCanvasElement.prototype.toBlob;
    const originalGetImageData = CanvasRenderingContext2D.prototype.getImageData;

    const noise = () => (Math.random() - 0.5) * 0.0001;

    HTMLCanvasElement.prototype.toDataURL = function() {
        const context = this.getContext('2d');
        if (context) {
            const imageData = context.getImageData(0, 0, this.width, this.height);
            for (let i = 0; i < imageData.data.length; i += 4) {
                imageData.data[i] += noise() * 255;
                imageData.data[i + 1] += noise() * 255;
                imageData.data[i + 2] += noise() * 255;
            }
            context.putImageData(imageData, 0, 0);
        }
        return originalToDataURL.apply(this, arguments);
    };

    HTMLCanvasElement.prototype.toBlob = function(callback) {
        const context = this.getContext('2d');
        if (context) {
            const imageData = context.getImageData(0, 0, this.width, this.height);
            for (let i = 0; i < imageData.data.length; i += 4) {
                imageData.data[i] += noise() * 255;
                imageData.data[i + 1] += noise() * 255;
                imageData.data[i + 2] += noise() * 255;
            }
            context.putImageData(imageData, 0, 0);
        }
        return originalToBlob.apply(this, arguments);
    };
})();
`

var webglNoiseScript = `
(() => {
    const getParameter = WebGLRenderingContext.prototype.getParameter;
    WebGLRenderingContext.prototype.getParameter = function(parameter) {
        const noise = (Math.random() - 0.5) * 0.00001;

        if (parameter === 37445) {
            return 'Intel Inc.' + noise;
        }
        if (parameter === 37446) {
            return 'Intel Iris OpenGL Engine' + noise;
        }

        return getParameter.apply(this, arguments);
    };

    const getParameter2 = WebGL2RenderingContext.prototype.getParameter;
    WebGL2RenderingContext.prototype.getParameter = function(parameter) {
        const noise = (Math.random() - 0.5) * 0.00001;

        if (parameter === 37445) {
            return 'Intel Inc.' + noise;
        }
        if (parameter === 37446) {
            return 'Intel Iris OpenGL Engine' + noise;
        }

        return getParameter2.apply(this, arguments);
    };
})();
`

var chromeRuntimeOverride = `
(() => {
    window.chrome = {
        runtime: {
            connect: () => {},
            sendMessage: () => {},
            onMessage: {
                addListener: () => {}
            }
        },
        loadTimes: () => ({
            requestTime: Date.now() / 1000,
            startLoadTime: Date.now() / 1000,
            commitLoadTime: Date.now() / 1000 + 0.1,
            finishDocumentLoadTime: Date.now() / 1000 + 0.2,
            finishLoadTime: Date.now() / 1000 + 0.3,
            navigationStart: Date.now() / 1000
        }),
        csi: () => ({
            onloadT: Date.now(),
            pageT: Date.now() + 100,
            startE: Date.now() - 1000,
            tran: 15
        })
    };
})();
`

var audioContextNoise = `
(() => {
    const AudioContext = window.AudioContext || window.webkitAudioContext;
    if (!AudioContext) return;

    const originalCreateOscillator = AudioContext.prototype.createOscillator;
    const originalCreateAnalyser = AudioContext.prototype.createAnalyser;

    AudioContext.prototype.createOscillator = function() {
        const oscillator = originalCreateOscillator.apply(this, arguments);
        const originalConnect = oscillator.connect;
        oscillator.connect = function() {
            oscillator.frequency.value += (Math.random() - 0.5) * 0.01;
            return originalConnect.apply(this, arguments);
        };
        return oscillator;
    };

    AudioContext.prototype.createAnalyser = function() {
        const analyser = originalCreateAnalyser.apply(this, arguments);
        const originalGetFloatFrequencyData = analyser.getFloatFrequencyData;
        analyser.getFloatFrequencyData = function(array) {
            originalGetFloatFrequencyData.apply(this, arguments);
            for (let i = 0; i < array.length; i++) {
                array[i] += (Math.random() - 0.5) * 0.00001;
            }
        };
        return analyser;
    };
})();
`
