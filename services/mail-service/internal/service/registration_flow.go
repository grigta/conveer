package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/conveer/conveer/services/mail-service/internal/models"
	proxypb "github.com/conveer/conveer/services/proxy-service/proto"
	smspb "github.com/conveer/conveer/services/sms-service/proto"
	"github.com/playwright-community/playwright-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RegistrationFlow handles the mail.ru registration process
type RegistrationFlow struct {
	service *MailService
	ctx     context.Context
	account *models.MailAccount
	session *models.RegistrationSession
	browser playwright.Browser
	page    playwright.Page
}

// NewRegistrationFlow creates a new registration flow
func (s *MailService) NewRegistrationFlow(ctx context.Context, accountID primitive.ObjectID) (*RegistrationFlow, error) {
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
		{models.StepEmailGeneration, f.generateEmail},
		{models.StepFormFilling, f.fillRegistrationForm},
		{models.StepPhoneVerification, f.verifyPhone},
		{models.StepCaptchaHandling, f.handleCaptcha},
		{models.StepEmailConfirmation, f.confirmEmail},
		{models.StepProfileSetup, f.setupProfile},
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
	resp, err := f.service.proxyClient.AllocateProxy(f.ctx, &proxypb.AllocateProxyRequest{
		Type:    "mobile",
		Country: "RU",
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

// Step 2: Generate email
func (f *RegistrationFlow) generateEmail() error {
	// Generate random email or use custom prefix
	var prefixStr string

	// Try to get prefix from checkpoints
	if v, ok := f.session.StepCheckpoints["email_prefix"].(string); ok && v != "" {
		prefixStr = v
	} else {
		// Generate new prefix if not exists
		prefixStr = f.generateRandomString(10)
		// Store the prefix for future reference
		f.session.StepCheckpoints["email_prefix"] = prefixStr
	}

	f.session.Email = fmt.Sprintf("%s@mail.ru", prefixStr)
	f.account.Email = f.session.Email

	// Generate password
	f.session.Password = f.generateSecurePassword()
	f.account.Password = f.session.Password

	// Save checkpoint
	f.session.StepCheckpoints["email"] = map[string]string{
		"email":    f.session.Email,
		"password": f.session.Password,
		"prefix":   prefixStr,
	}

	f.service.sessionRepo.UpdateSession(f.ctx, f.account.ID, map[string]interface{}{
		"email":    f.session.Email,
		"password": f.session.Password,
	})

	return nil
}

// Step 3: Fill registration form
func (f *RegistrationFlow) fillRegistrationForm() error {
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
	
	// Navigate to signup page
	if _, err := page.Goto("https://account.mail.ru/signup", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return fmt.Errorf("failed to navigate to signup page: %w", err)
	}
	
	// Wait for form
	if err := page.WaitForSelector("form", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	}); err != nil {
		return fmt.Errorf("registration form not found: %w", err)
	}
	
	// Fill email
	if err := f.typeWithHumanSpeed(page, "input[name='email']", strings.Split(f.session.Email, "@")[0]); err != nil {
		return fmt.Errorf("failed to fill email: %w", err)
	}
	
	// Fill password
	if err := f.typeWithHumanSpeed(page, "input[name='password']", f.session.Password); err != nil {
		return fmt.Errorf("failed to fill password: %w", err)
	}
	
	// Fill first name
	if err := f.typeWithHumanSpeed(page, "input[name='firstname']", f.account.FirstName); err != nil {
		return fmt.Errorf("failed to fill first name: %w", err)
	}
	
	// Fill last name
	if err := f.typeWithHumanSpeed(page, "input[name='lastname']", f.account.LastName); err != nil {
		return fmt.Errorf("failed to fill last name: %w", err)
	}
	
	// Set birth date
	if err := f.setBirthDate(page, f.account.BirthDate); err != nil {
		return fmt.Errorf("failed to set birth date: %w", err)
	}
	
	// Select gender
	if err := f.selectGender(page, f.account.Gender); err != nil {
		return fmt.Errorf("failed to select gender: %w", err)
	}
	
	// Random delay before submit
	time.Sleep(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)
	
	// Click submit
	if err := page.Click("button[type='submit']"); err != nil {
		return fmt.Errorf("failed to submit form: %w", err)
	}
	
	// Wait for next step
	time.Sleep(3 * time.Second)
	
	return nil
}

// Step 4: Verify phone
func (f *RegistrationFlow) verifyPhone() error {
	// Check if phone verification is enabled for this session
	if !f.session.UsePhoneVerification {
		return nil
	}

	// Check if phone field exists
	phoneExists, err := f.page.Locator("input[name='phone']").Count()
	if err != nil || phoneExists == 0 {
		// Phone verification not required by the form
		return nil
	}
	
	// Purchase phone number
	resp, err := f.service.smsClient.PurchaseNumber(f.ctx, &smspb.PurchaseNumberRequest{
		Service: "mail.ru",
		Country: "RU",
	})
	if err != nil {
		return fmt.Errorf("failed to purchase phone: %w", err)
	}
	
	f.session.Phone = resp.PhoneNumber
	f.session.ActivationID = resp.ActivationId
	f.account.Phone = resp.PhoneNumber
	f.account.ActivationID = resp.ActivationId
	
	// Enter phone number
	if err := f.typeWithHumanSpeed(f.page, "input[name='phone']", resp.PhoneNumber); err != nil {
		return fmt.Errorf("failed to enter phone: %w", err)
	}
	
	// Click send SMS
	if err := f.page.Click("button[data-test-id='send-code-button']"); err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}
	
	// Wait for SMS code
	var smsCode string
	for i := 0; i < f.service.config.MaxSMSPolls; i++ {
		time.Sleep(f.service.config.SMSPollingInterval)
		
		codeResp, err := f.service.smsClient.GetSMSCode(f.ctx, &smspb.GetSMSCodeRequest{
			ActivationId: resp.ActivationId,
		})
		if err == nil && codeResp.Code != "" {
			smsCode = codeResp.Code
			break
		}
	}
	
	if smsCode == "" {
		return fmt.Errorf("SMS code not received")
	}
	
	// Enter SMS code
	if err := f.typeWithHumanSpeed(f.page, "input[name='code']", smsCode); err != nil {
		return fmt.Errorf("failed to enter SMS code: %w", err)
	}
	
	// Submit code
	if err := f.page.Click("button[type='submit']"); err != nil {
		return fmt.Errorf("failed to submit SMS code: %w", err)
	}
	
	// Wait for verification
	time.Sleep(3 * time.Second)
	
	return nil
}

// Step 5: Handle CAPTCHA
func (f *RegistrationFlow) handleCaptcha() error {
	// Check for CAPTCHA
	captchaSelectors := []string{
		".captcha-image",
		".g-recaptcha",
		"iframe[src*='captcha']",
		"div[class*='captcha']",
	}
	
	for _, selector := range captchaSelectors {
		count, _ := f.page.Locator(selector).Count()
		if count > 0 {
			f.session.CaptchaDetected = true
			f.service.metrics.IncrementCaptchaDetected()
			
			// Publish to manual intervention queue
			if err := f.service.publishManualIntervention(f.account.ID.Hex(), "CAPTCHA detected"); err != nil {
				log.Printf("Failed to publish manual intervention: %v", err)
			}
			
			// Update account status
			f.service.accountRepo.UpdateAccountStatus(f.ctx, f.account.ID, models.AccountStatusSuspended, "CAPTCHA detected")
			
			return fmt.Errorf("CAPTCHA detected, manual intervention required")
		}
	}
	
	return nil
}

// Step 6: Confirm email
func (f *RegistrationFlow) confirmEmail() error {
	// Check if email confirmation is required
	if _, err := f.page.Goto("https://e.mail.ru/inbox", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		// Email confirmation might not be required
		return nil
	}
	
	// Wait for confirmation email
	time.Sleep(5 * time.Second)
	
	// Look for confirmation email
	confirmationLink, err := f.page.Locator("a[href*='confirm']").First().GetAttribute("href")
	if err != nil || confirmationLink == "" {
		// No confirmation required
		return nil
	}
	
	// Navigate to confirmation link
	if _, err := f.page.Goto(confirmationLink, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("failed to confirm email: %w", err)
	}
	
	return nil
}

// Step 7: Setup profile
func (f *RegistrationFlow) setupProfile() error {
	// Extract cookies
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
	
	// Save account with all credentials
	if err := f.service.accountRepo.UpdateAccountFullCredentials(
		f.ctx,
		f.account.ID,
		f.account.Phone,
		f.account.Password,
		f.account.Cookies,
		f.account.Email,
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
	} else if strings.Contains(errorMsg, "rate limit") || strings.Contains(errorMsg, "too many requests") {
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
	
	if f.session.ActivationID != "" {
		f.service.smsClient.CancelActivation(f.ctx, &smspb.CancelActivationRequest{
			ActivationId: f.session.ActivationID,
		})
	}
}

func (f *RegistrationFlow) typeWithHumanSpeed(page playwright.Page, selector string, text string) error {
	return TypeWithHumanSpeed(page, selector, text)
}

func (f *RegistrationFlow) setBirthDate(page playwright.Page, birthDate string) error {
	// Parse birth date (format: YYYY-MM-DD)
	parts := strings.Split(birthDate, "-")
	if len(parts) != 3 {
		return fmt.Errorf("invalid birth date format")
	}
	
	// Fill day
	if err := page.SelectOption("select[name='birth_day']", playwright.SelectOptionValues{
		Values: &[]string{parts[2]},
	}); err != nil {
		return err
	}
	
	// Fill month
	if err := page.SelectOption("select[name='birth_month']", playwright.SelectOptionValues{
		Values: &[]string{parts[1]},
	}); err != nil {
		return err
	}
	
	// Fill year
	if err := page.SelectOption("select[name='birth_year']", playwright.SelectOptionValues{
		Values: &[]string{parts[0]},
	}); err != nil {
		return err
	}
	
	return nil
}

func (f *RegistrationFlow) selectGender(page playwright.Page, gender string) error {
	var selector string
	if gender == "male" {
		selector = "input[value='male']"
	} else {
		selector = "input[value='female']"
	}
	
	return page.Click(selector)
}

func (f *RegistrationFlow) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func (f *RegistrationFlow) generateSecurePassword() string {
	const (
		lowercase = "abcdefghijklmnopqrstuvwxyz"
		uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digits    = "0123456789"
		special   = "!@#$%^&*"
	)
	
	var password strings.Builder
	password.WriteByte(uppercase[rand.Intn(len(uppercase))])
	password.WriteByte(lowercase[rand.Intn(len(lowercase))])
	password.WriteByte(digits[rand.Intn(len(digits))])
	password.WriteByte(special[rand.Intn(len(special))])
	
	allChars := lowercase + uppercase + digits + special
	for i := 0; i < 8; i++ {
		password.WriteByte(allChars[rand.Intn(len(allChars))])
	}
	
	// Shuffle the password
	runes := []rune(password.String())
	rand.Shuffle(len(runes), func(i, j int) {
		runes[i], runes[j] = runes[j], runes[i]
	})
	
	return string(runes)
}
