package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/conveer/conveer/services/max-service/internal/models"
	proxypb "github.com/conveer/conveer/services/proxy-service/proto"
	"github.com/playwright-community/playwright-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RegistrationFlow handles the Max messenger registration process
type RegistrationFlow struct {
	service *MaxService
	ctx     context.Context
	account *models.MaxAccount
	session *models.RegistrationSession
	browser playwright.Browser
	page    playwright.Page
}

// NewRegistrationFlow creates a new registration flow
func (s *MaxService) NewRegistrationFlow(ctx context.Context, accountID primitive.ObjectID) (*RegistrationFlow, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	
	session, err := s.sessionRepo.GetSession(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	
	return &RegistrationFlow{
		service: s,
		ctx:     ctx,
		account: account,
		session: session,
	}, nil
}

// Execute runs the registration flow
func (f *RegistrationFlow) Execute() error {
	start := time.Now()
	defer func() {
		f.service.metrics.RecordStepDuration("total", time.Since(start))
		// Release browser if it was allocated
		if f.browser != nil {
			f.service.browserManager.ReleaseBrowser(f.browser)
			f.browser = nil
		}
	}()
	
	// Execute steps based on current session state
	steps := []struct {
		step models.RegistrationStep
		fn   func() error
	}{
		{models.StepProxyAllocation, f.allocateProxy},
		{models.StepVKAccountCheck, f.checkVKAccount},
		{models.StepVKRegistration, f.registerVKAccount},
		{models.StepVKLogin, f.loginToVK},
		{models.StepMaxActivation, f.activateMax},
		{models.StepMaxProfileSetup, f.setupMaxProfile},
	}
	
	startIdx := 0
	for i, s := range steps {
		if s.step == f.session.CurrentStep {
			startIdx = i
			break
		}
	}
	
	for i := startIdx; i < len(steps); i++ {
		stepStart := time.Now()
		
		log.Printf("Executing step: %s", steps[i].step)
		f.session.CurrentStep = steps[i].step
		f.service.sessionRepo.UpdateStep(f.ctx, f.session.ID, steps[i].step, nil)
		
		if err := steps[i].fn(); err != nil {
			f.handleStepError(steps[i].step, err)
			return fmt.Errorf("step %s failed: %w", steps[i].step, err)
		}
		
		f.service.metrics.RecordStepDuration(string(steps[i].step), time.Since(stepStart))
		f.session.LastActivityAt = time.Now()
	}
	
	// Mark as complete
	f.session.CurrentStep = models.StepComplete
	now := time.Now()
	f.session.CompletedAt = &now
	f.service.sessionRepo.Complete(f.ctx, f.session.ID)
	
	// Update account status
	f.service.accountRepo.UpdateAccountStatus(f.ctx, f.account.ID, models.AccountStatusCreated, "")
	f.service.metrics.IncrementRegistrationSuccess()
	
	return nil
}

// Step 1: Allocate proxy
func (f *RegistrationFlow) allocateProxy() error {
	// Prefer Russian or Belarusian proxy for Max
	country := "RU"
	if rand.Float32() < 0.3 {
		country = "BY"
	}
	
	resp, err := f.service.proxyClient.AllocateProxy(f.ctx, &proxypb.AllocateProxyRequest{
		Type:    "residential",
		Country: country,
	})
	if err != nil {
		return fmt.Errorf("failed to allocate proxy: %w", err)
	}
	
	f.session.ProxyID = resp.ProxyId
	f.session.ProxyURL = resp.ProxyUrl
	f.account.ProxyID = resp.ProxyId
	f.account.RegistrationIP = resp.IpAddress
	
	// Save checkpoint
	f.session.StepCheckpoints["proxy"] = map[string]string{
		"proxy_id":  resp.ProxyId,
		"proxy_url": resp.ProxyUrl,
		"ip":        resp.IpAddress,
	}
	
	f.service.sessionRepo.UpdateSession(f.ctx, f.account.ID, map[string]interface{}{
		"proxy_id":  resp.ProxyId,
		"proxy_url": resp.ProxyUrl,
	})
	
	return nil
}

// Step 2: Check VK account
func (f *RegistrationFlow) checkVKAccount() error {
	// Check if VK account ID provided
	if f.session.VKAccountID != "" {
		// Verify VK account exists and is ready
		if err := f.service.vkIntegration.CheckVKAccount(f.ctx, f.session.VKAccountID); err != nil {
			return fmt.Errorf("VK account not ready: %w", err)
		}
		
		// Get VK credentials
		creds, err := f.service.vkIntegration.GetVKCredentials(f.ctx, f.session.VKAccountID)
		if err != nil {
			return fmt.Errorf("failed to get VK credentials: %w", err)
		}
		
		f.account.VKUserID = creds.UserID
		f.account.Phone = creds.Phone
		// Note: Password is not available from VK service proto
		// It should be stored separately or managed by VK service internally
		if creds.Password != "" {
			f.account.Password = creds.Password
		}
		f.account.IsVKLinked = true
		
		// Save checkpoint
		f.session.StepCheckpoints["vk_account"] = map[string]string{
			"vk_account_id": f.session.VKAccountID,
			"vk_user_id":    creds.UserID,
			"has_cookies":   fmt.Sprintf("%t", creds.Cookies != ""),
		}
		
		// Skip VK registration step
		f.session.CurrentStep = models.StepVKLogin
		
		return nil
	}
	
	// Check if should create new VK account
	if f.session.CreateNewVKAccount {
		// Will create in next step
		return nil
	}

	return fmt.Errorf("no VK account provided and create_new_vk_account not set")
}

// Step 3: Register new VK account if needed
func (f *RegistrationFlow) registerVKAccount() error {
	// Skip if already have VK account
	if f.session.VKAccountID != "" {
		return nil
	}
	
	// Create new VK account
	req := &VKAccountRequest{
		FirstName:        f.account.FirstName,
		LastName:         f.account.LastName,
		BirthDate:        "1990-01-01", // Default birth date
		Gender:           "male",
		PreferredCountry: "RU",
	}
	
	result, err := f.service.vkIntegration.CreateVKAccount(f.ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create VK account: %w", err)
	}
	
	f.session.VKAccountID = result.AccountID
	f.account.VKAccountID = result.AccountID
	f.account.IsVKLinked = true
	
	// Wait for VK account to be ready
	time.Sleep(10 * time.Second)
	
	// Get VK credentials
	creds, err := f.service.vkIntegration.GetVKCredentials(f.ctx, result.AccountID)
	if err != nil {
		return fmt.Errorf("failed to get VK credentials: %w", err)
	}
	
	f.account.VKUserID = creds.UserID
	f.account.Phone = creds.Phone
	// Note: Password is not available from VK service proto
	// It should be stored separately or managed by VK service internally
	if creds.Password != "" {
		f.account.Password = creds.Password
	}
	
	return nil
}

// Step 4: Login to VK
func (f *RegistrationFlow) loginToVK() error {
	// Setup browser with proxy
	fingerprint := GenerateFingerprint()
	f.account.Fingerprint = fingerprint
	f.account.UserAgent = fingerprint.UserAgent
	
	browser, err := f.service.browserManager.AcquireBrowser(f.ctx, &BrowserConfig{
		ProxyURL:    f.session.ProxyURL,
		Fingerprint: fingerprint,
	})
	if err != nil {
		return fmt.Errorf("failed to acquire browser: %w", err)
	}
	f.browser = browser
	
	// Create page
	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}
	f.page = page
	
	// Inject stealth
	if err := InjectStealth(page); err != nil {
		return fmt.Errorf("failed to inject stealth: %w", err)
	}
	
	// Get VK credentials
	creds := &VKCredentials{
		UserID:   f.account.VKUserID,
		Phone:    f.account.Phone,
		Password: f.account.Password,
	}
	
	// Load cookies if available
	if f.account.Cookies != "" {
		creds.Cookies = f.account.Cookies
	}
	
	// Login to VK
	if err := f.service.vkIntegration.LoginToVK(f.ctx, page, creds); err != nil {
		return fmt.Errorf("failed to login to VK: %w", err)
	}
	
	// Save VK session
	vkCookies, err := page.Context().Cookies()
	if err == nil {
		vkCookiesJSON, _ := json.Marshal(vkCookies)
		f.session.StepCheckpoints["vk_cookies"] = string(vkCookiesJSON)
	}
	
	return nil
}

// Step 5: Activate Max messenger
func (f *RegistrationFlow) activateMax() error {
	// Navigate to Max messenger page
	maxURLs := []string{
		"https://vk.com/messenger",
		"https://vk.me",
		"https://vk.com/max",
	}
	
	var activated bool
	for _, url := range maxURLs {
		if _, err := f.page.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(30000),
		}); err != nil {
			continue
		}
		
		// Wait for page to load
		time.Sleep(3 * time.Second)
		
		// Look for activation button
		activationButtons := []string{
			"button:has-text('Начать использовать')",
			"button:has-text('Start using')",
			"button:has-text('Активировать')",
			"button:has-text('Activate')",
			"a:has-text('Попробовать Max')",
		}
		
		for _, selector := range activationButtons {
			if count, _ := f.page.Locator(selector).Count(); count > 0 {
				if err := f.page.Click(selector); err == nil {
					activated = true
					break
				}
			}
		}
		
		if activated {
			break
		}
	}
	
	if !activated {
		// Max might already be activated
		log.Printf("Max activation button not found, might be already activated")
	}
	
	// Wait for activation
	time.Sleep(5 * time.Second)
	
	// Check for permissions dialog
	if count, _ := f.page.Locator("button:has-text('Разрешить')").Count(); count > 0 {
		f.page.Click("button:has-text('Разрешить')")
	}
	
	// Extract Max session token from cookies or localStorage
	maxToken, err := f.page.Evaluate(`
		(() => {
			// Try to get from localStorage
			const token = localStorage.getItem('max_session_token') || 
						  localStorage.getItem('vk_max_token') ||
						  sessionStorage.getItem('max_token');
			return token;
		})()
	`)
	if err == nil && maxToken != nil {
		if tokenStr, ok := maxToken.(string); ok && tokenStr != "" {
			f.account.MaxSessionToken = tokenStr
			f.session.MaxSessionToken = tokenStr
		}
	}
	
	return nil
}

// Step 6: Setup Max profile
func (f *RegistrationFlow) setupMaxProfile() error {
	// Set username if provided
	if f.account.Username != "" {
		// Look for username field
		usernameSelectors := []string{
			"input[name='username']",
			"input[placeholder*='username']",
			"input[placeholder*='имя пользователя']",
		}
		
		for _, selector := range usernameSelectors {
			if count, _ := f.page.Locator(selector).Count(); count > 0 {
				TypeWithHumanSpeed(f.page, selector, f.account.Username)
				break
			}
		}
	}
	
	// Set avatar if provided
	if f.account.AvatarURL != "" {
		// Implementation for avatar upload would go here
		log.Printf("Avatar upload not implemented yet")
	}
	
	// Extract all cookies including Max-specific ones
	cookies, err := f.page.Context().Cookies()
	if err != nil {
		return fmt.Errorf("failed to get cookies: %w", err)
	}
	
	// Convert cookies to JSON
	cookiesJSON, err := json.Marshal(cookies)
	if err != nil {
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}
	
	f.account.Cookies = string(cookiesJSON)
	
	// Extract VK access token if available
	accessToken, err := f.page.Evaluate(`
		(() => {
			// Try to get VK access token
			const token = window.vk?.access_token || 
						  localStorage.getItem('vk_access_token');
			return token;
		})()
	`)
	if err == nil && accessToken != nil {
		if tokenStr, ok := accessToken.(string); ok && tokenStr != "" {
			f.account.VKAccessToken = tokenStr
			f.session.VKAccessToken = tokenStr
		}
	}
	
	// Save account with all credentials
	if err := f.service.accountRepo.UpdateAccountFullCredentials(
		f.ctx,
		f.account.ID,
		f.account.Phone,
		f.account.Password,
		f.account.Cookies,
		f.account.VKUserID,
		f.account.VKAccessToken,
		f.account.MaxSessionToken,
		models.AccountStatusCreated,
	); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}
	
	return nil
}

// Helper methods

func (f *RegistrationFlow) handleStepError(step models.RegistrationStep, err error) {
	f.service.metrics.IncrementRegistrationFailure(string(step))
	
	// Check for specific errors
	errorMsg := err.Error()
	
	if strings.Contains(errorMsg, "CAPTCHA") {
		f.service.publishManualIntervention(f.account.ID.Hex(), "CAPTCHA detected")
		f.service.accountRepo.UpdateAccountStatus(f.ctx, f.account.ID, models.AccountStatusSuspended, errorMsg)
	} else if strings.Contains(errorMsg, "VK account banned") || strings.Contains(errorMsg, "VK account not ready") {
		f.service.accountRepo.UpdateAccountStatus(f.ctx, f.account.ID, models.AccountStatusError, "VK account issue")
	} else if strings.Contains(errorMsg, "rate limit") {
		f.service.accountRepo.UpdateAccountStatus(f.ctx, f.account.ID, models.AccountStatusError, "Rate limited")
	} else if strings.Contains(errorMsg, "banned") || strings.Contains(errorMsg, "blocked") {
		f.service.accountRepo.UpdateAccountStatus(f.ctx, f.account.ID, models.AccountStatusBanned, errorMsg)
	} else {
		f.service.accountRepo.UpdateAccountStatus(f.ctx, f.account.ID, models.AccountStatusError, errorMsg)
	}
	
	// Release resources
	if f.browser != nil {
		f.service.browserManager.ReleaseBrowser(f.browser)
		f.browser = nil // Prevent double release in defer
	}
	
	if f.session.ProxyID != "" {
		f.service.proxyClient.ReleaseProxy(f.ctx, &proxypb.ReleaseProxyRequest{
			ProxyId: f.session.ProxyID,
		})
	}
}
