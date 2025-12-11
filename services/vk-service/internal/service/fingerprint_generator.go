package service

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/playwright-community/playwright-go"
)

type Fingerprint struct {
	UserAgent           string                 `json:"user_agent"`
	Viewport            Viewport               `json:"viewport"`
	Timezone            string                 `json:"timezone"`
	Locale              string                 `json:"locale"`
	Platform            string                 `json:"platform"`
	HardwareConcurrency int                    `json:"hardware_concurrency"`
	DeviceMemory        int                    `json:"device_memory"`
	ColorDepth          int                    `json:"color_depth"`
	ScreenResolution    ScreenResolution       `json:"screen_resolution"`
	Languages           []string               `json:"languages"`
	WebGLVendor         string                 `json:"webgl_vendor"`
	WebGLRenderer       string                 `json:"webgl_renderer"`
	Fonts               []string               `json:"fonts"`
	DNT                 string                 `json:"dnt"`
	Plugins             []PluginData           `json:"plugins"`
	Extra               map[string]interface{} `json:"extra"`
}

type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ScreenResolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type PluginData struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Filename    string `json:"filename"`
}

type FingerprintGenerator interface {
	GenerateFingerprint() *Fingerprint
	ApplyFingerprint(context playwright.BrowserContext, fingerprint *Fingerprint) error
	GenerateRandomProfile() RandomProfile
}

type fingerprintGenerator struct {
	rand *rand.Rand
}

type RandomProfile struct {
	FirstName string
	LastName  string
	BirthDate time.Time
	Gender    string
}

func NewFingerprintGenerator() FingerprintGenerator {
	return &fingerprintGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (g *fingerprintGenerator) GenerateFingerprint() *Fingerprint {
	viewport := g.getRandomViewport()
	screen := g.getScreenForViewport(viewport)

	fingerprint := &Fingerprint{
		UserAgent:           g.getRandomUserAgent(),
		Viewport:            viewport,
		Timezone:            g.getRandomTimezone(),
		Locale:              g.getRandomLocale(),
		Platform:            g.getRandomPlatform(),
		HardwareConcurrency: g.getRandomHardwareConcurrency(),
		DeviceMemory:        g.getRandomDeviceMemory(),
		ColorDepth:          24,
		ScreenResolution:    screen,
		Languages:           g.getRandomLanguages(),
		WebGLVendor:         g.getRandomWebGLVendor(),
		WebGLRenderer:       g.getRandomWebGLRenderer(),
		Fonts:               g.getRandomFonts(),
		DNT:                 g.getRandomDNT(),
		Plugins:             g.getRandomPlugins(),
		Extra:               make(map[string]interface{}),
	}

	// Add some randomized extra properties
	fingerprint.Extra["maxTouchPoints"] = g.rand.Intn(5)
	fingerprint.Extra["cookieEnabled"] = true
	fingerprint.Extra["onLine"] = true
	fingerprint.Extra["doNotTrack"] = fingerprint.DNT

	return fingerprint
}

func (g *fingerprintGenerator) ApplyFingerprint(context playwright.BrowserContext, fingerprint *Fingerprint) error {
	// Set viewport
	if err := context.SetViewportSize(fingerprint.Viewport.Width, fingerprint.Viewport.Height); err != nil {
		return fmt.Errorf("failed to set viewport: %w", err)
	}

	// Set user agent
	if err := context.AddInitScript(playwright.Script{
		Content: &[]string{fmt.Sprintf(`
			Object.defineProperty(navigator, 'userAgent', {
				get: () => '%s'
			});
		`, fingerprint.UserAgent)}[0],
	}); err != nil {
		return fmt.Errorf("failed to set user agent: %w", err)
	}

	// Set locale
	if err := context.AddInitScript(playwright.Script{
		Content: &[]string{fmt.Sprintf(`
			Object.defineProperty(navigator, 'language', {
				get: () => '%s'
			});
			Object.defineProperty(navigator, 'languages', {
				get: () => %v
			});
		`, fingerprint.Locale, g.formatLanguages(fingerprint.Languages))}[0],
	}); err != nil {
		return fmt.Errorf("failed to set locale: %w", err)
	}

	// Set platform
	if err := context.AddInitScript(playwright.Script{
		Content: &[]string{fmt.Sprintf(`
			Object.defineProperty(navigator, 'platform', {
				get: () => '%s'
			});
		`, fingerprint.Platform)}[0],
	}); err != nil {
		return fmt.Errorf("failed to set platform: %w", err)
	}

	// Set hardware concurrency
	if err := context.AddInitScript(playwright.Script{
		Content: &[]string{fmt.Sprintf(`
			Object.defineProperty(navigator, 'hardwareConcurrency', {
				get: () => %d
			});
		`, fingerprint.HardwareConcurrency)}[0],
	}); err != nil {
		return fmt.Errorf("failed to set hardware concurrency: %w", err)
	}

	// Set device memory
	if err := context.AddInitScript(playwright.Script{
		Content: &[]string{fmt.Sprintf(`
			Object.defineProperty(navigator, 'deviceMemory', {
				get: () => %d
			});
		`, fingerprint.DeviceMemory)}[0],
	}); err != nil {
		return fmt.Errorf("failed to set device memory: %w", err)
	}

	// Set screen properties
	if err := context.AddInitScript(playwright.Script{
		Content: &[]string{fmt.Sprintf(`
			Object.defineProperty(screen, 'width', {
				get: () => %d
			});
			Object.defineProperty(screen, 'height', {
				get: () => %d
			});
			Object.defineProperty(screen, 'availWidth', {
				get: () => %d
			});
			Object.defineProperty(screen, 'availHeight', {
				get: () => %d
			});
			Object.defineProperty(screen, 'colorDepth', {
				get: () => %d
			});
			Object.defineProperty(screen, 'pixelDepth', {
				get: () => %d
			});
		`, fingerprint.ScreenResolution.Width, fingerprint.ScreenResolution.Height,
			fingerprint.ScreenResolution.Width, fingerprint.ScreenResolution.Height-40,
			fingerprint.ColorDepth, fingerprint.ColorDepth)}[0],
	}); err != nil {
		return fmt.Errorf("failed to set screen properties: %w", err)
	}

	return nil
}

func (g *fingerprintGenerator) getRandomUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
	return userAgents[g.rand.Intn(len(userAgents))]
}

func (g *fingerprintGenerator) getRandomViewport() Viewport {
	viewports := []Viewport{
		{1920, 1080},
		{1366, 768},
		{1440, 900},
		{1536, 864},
		{1280, 720},
		{1600, 900},
		{1680, 1050},
		{2560, 1440},
	}
	return viewports[g.rand.Intn(len(viewports))]
}

func (g *fingerprintGenerator) getScreenForViewport(viewport Viewport) ScreenResolution {
	// Screen is usually same as viewport or slightly larger
	return ScreenResolution{
		Width:  viewport.Width,
		Height: viewport.Height,
	}
}

func (g *fingerprintGenerator) getRandomTimezone() string {
	timezones := []string{
		"Europe/Moscow",
		"Europe/Kiev",
		"Europe/Minsk",
		"Asia/Yekaterinburg",
		"Asia/Novosibirsk",
		"Europe/Samara",
		"Asia/Krasnoyarsk",
	}
	return timezones[g.rand.Intn(len(timezones))]
}

func (g *fingerprintGenerator) getRandomLocale() string {
	locales := []string{
		"ru-RU",
		"en-US",
		"uk-UA",
		"be-BY",
	}
	return locales[g.rand.Intn(len(locales))]
}

func (g *fingerprintGenerator) getRandomPlatform() string {
	platforms := []string{
		"Win32",
		"MacIntel",
		"Linux x86_64",
	}
	return platforms[g.rand.Intn(len(platforms))]
}

func (g *fingerprintGenerator) getRandomHardwareConcurrency() int {
	options := []int{4, 8, 16, 12, 6}
	return options[g.rand.Intn(len(options))]
}

func (g *fingerprintGenerator) getRandomDeviceMemory() int {
	options := []int{4, 8, 16, 32}
	return options[g.rand.Intn(len(options))]
}

func (g *fingerprintGenerator) getRandomLanguages() []string {
	languageSets := [][]string{
		{"ru-RU", "ru", "en-US", "en"},
		{"ru-RU", "ru"},
		{"en-US", "en"},
		{"uk-UA", "uk", "ru-RU", "ru", "en-US", "en"},
	}
	return languageSets[g.rand.Intn(len(languageSets))]
}

func (g *fingerprintGenerator) getRandomWebGLVendor() string {
	vendors := []string{
		"Intel Inc.",
		"Google Inc.",
		"NVIDIA Corporation",
		"ATI Technologies Inc.",
		"Intel Corporation",
	}
	return vendors[g.rand.Intn(len(vendors))]
}

func (g *fingerprintGenerator) getRandomWebGLRenderer() string {
	renderers := []string{
		"ANGLE (Intel(R) HD Graphics Direct3D11 vs_5_0 ps_5_0)",
		"ANGLE (NVIDIA GeForce GTX 1060 Direct3D11 vs_5_0 ps_5_0)",
		"ANGLE (Intel(R) UHD Graphics 620 Direct3D11 vs_5_0 ps_5_0)",
		"Intel Iris OpenGL Engine",
		"Mesa DRI Intel(R) HD Graphics",
	}
	return renderers[g.rand.Intn(len(renderers))]
}

func (g *fingerprintGenerator) getRandomFonts() []string {
	fontSets := [][]string{
		{"Arial", "Arial Black", "Calibri", "Cambria", "Cambria Math", "Comic Sans MS", "Consolas", "Courier", "Courier New", "Georgia", "Helvetica", "Impact", "Lucida Console", "Lucida Sans Unicode", "Microsoft Sans Serif", "MS Gothic", "MS PGothic", "MS Sans Serif", "MS Serif", "Palatino Linotype", "Segoe Print", "Segoe Script", "Segoe UI", "Segoe UI Light", "Segoe UI Symbol", "Tahoma", "Times", "Times New Roman", "Trebuchet MS", "Verdana", "Wingdings"},
		{"Arial", "Courier New", "Georgia", "Times New Roman", "Verdana"},
		{"Arial", "Arial Black", "Arial Narrow", "Book Antiqua", "Bookman Old Style", "Calibri", "Cambria", "Cambria Math", "Century", "Century Gothic", "Comic Sans MS", "Consolas", "Courier", "Courier New", "Garamond", "Georgia", "Helvetica", "Impact", "Lucida Console", "Lucida Sans Unicode", "Microsoft Sans Serif", "Monotype Corsiva", "MS Reference Sans Serif", "MS Reference Specialty", "MS Sans Serif", "MS Serif", "Palatino Linotype", "Segoe Print", "Segoe Script", "Segoe UI", "Segoe UI Light", "Segoe UI Symbol", "Tahoma", "Times", "Times New Roman", "Trebuchet MS", "Verdana"},
	}
	return fontSets[g.rand.Intn(len(fontSets))]
}

func (g *fingerprintGenerator) getRandomDNT() string {
	options := []string{"1", "null", "unspecified"}
	return options[g.rand.Intn(len(options))]
}

func (g *fingerprintGenerator) getRandomPlugins() []PluginData {
	return []PluginData{
		{
			Name:        "Chrome PDF Plugin",
			Description: "Portable Document Format",
			Filename:    "internal-pdf-viewer",
		},
		{
			Name:        "Chrome PDF Viewer",
			Description: "",
			Filename:    "mhjfbmdgcfjbbpaeojofohoefgiehjai",
		},
		{
			Name:        "Native Client",
			Description: "",
			Filename:    "internal-nacl-plugin",
		},
	}
}

func (g *fingerprintGenerator) formatLanguages(languages []string) string {
	result := "["
	for i, lang := range languages {
		result += fmt.Sprintf("'%s'", lang)
		if i < len(languages)-1 {
			result += ", "
		}
	}
	result += "]"
	return result
}

func (g *fingerprintGenerator) GenerateRandomProfile() RandomProfile {
	firstNamesMale := []string{"Александр", "Дмитрий", "Максим", "Сергей", "Андрей", "Алексей", "Артём", "Иван", "Кирилл", "Михаил", "Никита", "Матвей", "Роман", "Егор", "Арсений", "Илья", "Денис", "Евгений", "Даниил", "Тимофей"}
	firstNamesFemale := []string{"Анна", "Мария", "Елена", "Наталья", "Ольга", "Екатерина", "Анастасия", "Дарья", "Юлия", "Ирина", "Татьяна", "Светлана", "Ксения", "Полина", "Алиса", "Виктория", "Александра", "Вероника", "Арина", "Валерия"}
	lastNames := []string{"Иванов", "Смирнов", "Кузнецов", "Попов", "Васильев", "Петров", "Соколов", "Михайлов", "Новиков", "Федоров", "Морозов", "Волков", "Алексеев", "Лебедев", "Семенов", "Егоров", "Павлов", "Козлов", "Степанов", "Николаев"}

	isMale := g.rand.Intn(2) == 0
	var firstName string
	var gender string

	if isMale {
		firstName = firstNamesMale[g.rand.Intn(len(firstNamesMale))]
		gender = "male"
	} else {
		firstName = firstNamesFemale[g.rand.Intn(len(firstNamesFemale))]
		gender = "female"
	}

	lastName := lastNames[g.rand.Intn(len(lastNames))]
	if !isMale {
		lastName += "а"
	}

	// Generate birth date between 18-35 years ago
	minAge := 18
	maxAge := 35
	ageInDays := minAge*365 + g.rand.Intn((maxAge-minAge)*365)
	birthDate := time.Now().AddDate(0, 0, -ageInDays)

	return RandomProfile{
		FirstName: firstName,
		LastName:  lastName,
		BirthDate: birthDate,
		Gender:    gender,
	}
}
