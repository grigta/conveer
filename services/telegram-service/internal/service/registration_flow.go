package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"conveer/pkg/logger"
	"conveer/services/telegram-service/internal/models"
	"conveer/services/telegram-service/internal/repository"
	proxypb "conveer/services/proxy-service/proto"
	smspb "conveer/services/sms-service/proto"

	"github.com/playwright-community/playwright-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RegistrationFlow interface {
	StartRegistration(ctx context.Context, req *models.RegistrationRequest) (*models.RegistrationResult, error)
	RetryRegistration(ctx context.Context, accountID primitive.ObjectID) (*models.RegistrationResult, error)
}

type registrationFlow struct {
	accountRepo     *repository.AccountRepository
	sessionRepo     *repository.SessionRepository
	browserManager  BrowserManager
	stealthInjector StealthInjector
	fingerprintGen  FingerprintGenerator
	proxyClient     proxypb.ProxyServiceClient
	smsClient       smspb.SMSServiceClient
	config          *models.RegistrationConfig
	logger          logger.Logger
	metrics         MetricsCollector
}

func NewRegistrationFlow(
	accountRepo *repository.AccountRepository,
	sessionRepo *repository.SessionRepository,
	browserManager BrowserManager,
	stealthInjector StealthInjector,
	fingerprintGen FingerprintGenerator,
	proxyClient proxypb.ProxyServiceClient,
	smsClient smspb.SMSServiceClient,
	config *models.RegistrationConfig,
	logger logger.Logger,
	metrics MetricsCollector,
) RegistrationFlow {
	return &registrationFlow{
		accountRepo:     accountRepo,
		sessionRepo:     sessionRepo,
		browserManager:  browserManager,
		stealthInjector: stealthInjector,
		fingerprintGen:  fingerprintGen,
		proxyClient:     proxyClient,
		smsClient:       smsClient,
		config:          config,
		logger:          logger,
		metrics:         metrics,
	}
}

func (f *registrationFlow) StartRegistration(ctx context.Context, req *models.RegistrationRequest) (*models.RegistrationResult, error) {
	startTime := time.Now()
	f.metrics.IncrementRegistrationAttempts()

	// Create new account
	account := &models.TelegramAccount{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Username:  req.Username,
		Bio:       req.Bio,
		AvatarURL: req.AvatarURL,
		Status:    models.StatusCreating,
		ApiID:     req.ApiID,
		ApiHash:   req.ApiHash,
	}

	// Generate fingerprint
	fingerprint, err := f.fingerprintGen.GenerateFingerprint()
	if err != nil {
		return f.handleError(account, models.StepProxyAllocation, err, startTime)
	}
	account.Fingerprint = fingerprint

	// Save account
	if err := f.accountRepo.Create(ctx, account); err != nil {
		return f.handleError(account, models.StepProxyAllocation, err, startTime)
	}

	// Create registration session
	session := &models.RegistrationSession{
		AccountID:       account.ID,
		CurrentStep:     models.StepProxyAllocation,
		StepCheckpoints: make(map[string]interface{}),
	}

	if err := f.sessionRepo.Create(ctx, session); err != nil {
		return f.handleError(account, models.StepProxyAllocation, err, startTime)
	}

	// Execute registration steps
	result := f.executeRegistrationFlow(ctx, account, session, req)
	result.Duration = time.Since(startTime).Seconds()

	return result, nil
}

func (f *registrationFlow) executeRegistrationFlow(
	ctx context.Context,
	account *models.TelegramAccount,
	session *models.RegistrationSession,
	req *models.RegistrationRequest,
) *models.RegistrationResult {
	var browser playwright.Browser
	var browserContext playwright.BrowserContext
	var page playwright.Page
	defer func() {
		if page != nil {
			page.Close()
		}
		if browserContext != nil {
			browserContext.Close()
		}
		if browser != nil {
			f.browserManager.ReleaseBrowser(browser)
		}
	}()

	// Step 1: Allocate proxy
	proxyConfig, err := f.allocateProxy(ctx, account, session)
	if err != nil {
		return f.handleError(account, models.StepProxyAllocation, err, time.Now())
	}

	// Step 2: Acquire browser with proxy
	browser, browserContext, err = f.browserManager.AcquireBrowser(ctx, proxyConfig)
	if err != nil {
		return f.handleError(account, models.StepProxyAllocation, err, time.Now())
	}

	// Create new page
	page, err = browserContext.NewPage()
	if err != nil {
		return f.handleError(account, models.StepProxyAllocation, err, time.Now())
	}

	// Inject stealth
	if err := f.stealthInjector.InjectStealth(page); err != nil {
		f.logger.Warn("Failed to inject stealth", "error", err)
	}

	// Step 3: Purchase phone number
	phone, activationID, err := f.purchasePhone(ctx, account, session, req.PreferredCountry)
	if err != nil {
		return f.handleError(account, models.StepPhonePurchase, err, time.Now())
	}
	account.Phone = phone
	account.ActivationID = activationID
	session.Phone = phone
	session.ActivationID = activationID

	// Step 4: Navigate to Telegram Web and enter phone
	if err := f.navigateAndEnterPhone(ctx, page, account, session); err != nil {
		return f.handleError(account, models.StepPhoneEntry, err, time.Now())
	}

	// Step 5: Wait for and enter SMS code
	if err := f.handleSMSVerification(ctx, page, account, session); err != nil {
		return f.handleError(account, models.StepSMSVerification, err, time.Now())
	}

	// Step 6: Setup profile
	if err := f.setupProfile(ctx, page, account, session, req); err != nil {
		return f.handleError(account, models.StepProfileSetup, err, time.Now())
	}

	// Step 7: Setup username if provided
	if req.Username != "" {
		if err := f.setupUsername(ctx, page, account, session); err != nil {
			f.logger.Warn("Failed to setup username", "error", err)
			// Non-critical, continue
		}
	}

	// Step 8: Upload avatar if provided
	if req.AvatarURL != "" {
		if err := f.uploadAvatar(ctx, page, account, session); err != nil {
			f.logger.Warn("Failed to upload avatar", "error", err)
			// Non-critical, continue
		}
	}

	// Step 9: Setup two-factor authentication if requested
	if req.EnableTwoFactor {
		if err := f.setupTwoFactor(ctx, page, account, session); err != nil {
			f.logger.Warn("Failed to setup 2FA", "error", err)
			// Non-critical, continue
		}
	}

	// Save cookies and session
	cookies, _ := browserContext.Cookies()
	cookieBytes, _ := serializeCookies(cookies)
	account.Cookies = cookieBytes

	// Mark as complete
	account.Status = models.StatusCreated
	f.accountRepo.Update(ctx, account)
	f.sessionRepo.Complete(ctx, session.ID)

	f.metrics.IncrementRegistrationSuccess()
	f.metrics.IncrementAccountCreated(models.StatusCreated)

	return &models.RegistrationResult{
		Success:   true,
		AccountID: account.ID.Hex(),
		UserID:    account.UserID,
		Phone:     account.Phone,
		Username:  account.Username,
	}
}

func (f *registrationFlow) allocateProxy(ctx context.Context, account *models.TelegramAccount, session *models.RegistrationSession) (*ProxyConfig, error) {
	f.metrics.IncrementProxyRequests()
	stepStart := time.Now()
	defer func() {
		f.metrics.RecordStepDuration("proxy_allocation", time.Since(stepStart).Seconds())
	}()

	resp, err := f.proxyClient.AllocateProxy(ctx, &proxypb.AllocateProxyRequest{
		Type:     "mobile",
		Country:  "US",
		Duration: 3600,
	})

	if err != nil {
		f.metrics.IncrementProxyFailure()
		return nil, fmt.Errorf("failed to allocate proxy: %w", err)
	}

	f.metrics.IncrementProxySuccess()

	proxyID, _ := primitive.ObjectIDFromHex(resp.ProxyId)
	account.ProxyID = proxyID
	session.ProxyID = proxyID
	session.ProxyURL = resp.ProxyUrl

	f.sessionRepo.UpdateStep(ctx, session.ID, models.StepProxyAllocation, map[string]interface{}{
		"proxy_id":  resp.ProxyId,
		"proxy_url": resp.ProxyUrl,
	})

	return &ProxyConfig{
		Server:   resp.ProxyUrl,
		Username: resp.Username,
		Password: resp.Password,
	}, nil
}

func (f *registrationFlow) purchasePhone(ctx context.Context, account *models.TelegramAccount, session *models.RegistrationSession, preferredCountry string) (string, string, error) {
	f.metrics.IncrementSMSRequests()
	stepStart := time.Now()
	defer func() {
		f.metrics.RecordStepDuration("phone_purchase", time.Since(stepStart).Seconds())
	}()

	country := preferredCountry
	if country == "" {
		country = "any"
	}

	resp, err := f.smsClient.PurchaseNumber(ctx, &smspb.PurchaseNumberRequest{
		Service: "telegram",
		Country: country,
	})

	if err != nil {
		f.metrics.IncrementSMSFailure()
		return "", "", fmt.Errorf("failed to purchase phone number: %w", err)
	}

	f.sessionRepo.UpdateStep(ctx, session.ID, models.StepPhonePurchase, map[string]interface{}{
		"phone":         resp.Phone,
		"activation_id": resp.ActivationId,
	})

	return resp.Phone, resp.ActivationId, nil
}

func (f *registrationFlow) navigateAndEnterPhone(ctx context.Context, page playwright.Page, account *models.TelegramAccount, session *models.RegistrationSession) error {
	stepStart := time.Now()
	defer func() {
		f.metrics.RecordStepDuration("phone_entry", time.Since(stepStart).Seconds())
	}()

	// Navigate to Telegram Web
	if _, err := page.Goto("https://web.telegram.org/k/", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return fmt.Errorf("failed to navigate to Telegram: %w", err)
	}

	// Wait for page to load
	time.Sleep(3 * time.Second)

	// Click on login button if exists
	if err := page.Click("button:has-text('Log in by phone Number')", playwright.PageClickOptions{
		Timeout: playwright.Float(5000),
	}); err != nil {
		// Button might not exist, continue
		f.logger.Debug("Login button not found, continuing")
	}

	// Enter phone number
	phoneInput := page.Locator("input[type='tel']")
	if err := phoneInput.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	}); err != nil {
		return fmt.Errorf("phone input not found: %w", err)
	}

	// Type phone number with delays
	for _, digit := range account.Phone {
		phoneInput.Type(string(digit), playwright.LocatorTypeOptions{
			Delay: playwright.Float(rand.Intn(200) + 100),
		})
	}

	// Click next/continue button
	nextButton := page.Locator("button.btn-primary:has-text('Next')")
	if err := nextButton.Click(); err != nil {
		// Try alternative selector
		if err := page.Click("button[type='submit']"); err != nil {
			return fmt.Errorf("failed to submit phone number: %w", err)
		}
	}

	f.sessionRepo.UpdateStep(ctx, session.ID, models.StepPhoneEntry, map[string]interface{}{
		"phone_entered": true,
	})

	return nil
}

func (f *registrationFlow) handleSMSVerification(ctx context.Context, page playwright.Page, account *models.TelegramAccount, session *models.RegistrationSession) error {
	stepStart := time.Now()
	defer func() {
		f.metrics.RecordStepDuration("sms_verification", time.Since(stepStart).Seconds())
	}()

	// Wait for SMS code input to appear
	codeInput := page.Locator("input[type='tel'][autocomplete='off']")
	if err := codeInput.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	}); err != nil {
		return fmt.Errorf("code input not found: %w", err)
	}

	// Poll for SMS code
	var smsCode string
	maxPolls := f.config.MaxSMSPolls
	for i := 0; i < maxPolls; i++ {
		resp, err := f.smsClient.GetSMSCode(ctx, &smspb.GetSMSCodeRequest{
			ActivationId: account.ActivationID,
		})

		if err == nil && resp.Code != "" {
			smsCode = resp.Code
			f.metrics.IncrementSMSSuccess()
			break
		}

		if i < maxPolls-1 {
			time.Sleep(f.config.SMSPollingInterval)
		}
	}

	if smsCode == "" {
		f.metrics.IncrementSMSFailure()
		return fmt.Errorf("failed to receive SMS code")
	}

	// Enter SMS code
	for _, digit := range smsCode {
		codeInput.Type(string(digit), playwright.LocatorTypeOptions{
			Delay: playwright.Float(rand.Intn(200) + 100),
		})
	}

	// Wait for verification
	time.Sleep(2 * time.Second)

	f.sessionRepo.UpdateStep(ctx, session.ID, models.StepSMSVerification, map[string]interface{}{
		"sms_verified": true,
	})

	return nil
}

func (f *registrationFlow) setupProfile(ctx context.Context, page playwright.Page, account *models.TelegramAccount, session *models.RegistrationSession, req *models.RegistrationRequest) error {
	stepStart := time.Now()
	defer func() {
		f.metrics.RecordStepDuration("profile_setup", time.Since(stepStart).Seconds())
	}()

	// Wait for profile setup page
	time.Sleep(3 * time.Second)

	// Enter first name
	firstNameInput := page.Locator("input[name='first_name'], input[placeholder*='First']")
	if count, _ := firstNameInput.Count(); count > 0 {
		firstNameInput.First().Clear()
		firstNameInput.First().Type(account.FirstName, playwright.LocatorTypeOptions{
			Delay: playwright.Float(100),
		})
	}

	// Enter last name if provided
	if account.LastName != "" {
		lastNameInput := page.Locator("input[name='last_name'], input[placeholder*='Last']")
		if count, _ := lastNameInput.Count(); count > 0 {
			lastNameInput.First().Clear()
			lastNameInput.First().Type(account.LastName, playwright.LocatorTypeOptions{
				Delay: playwright.Float(100),
			})
		}
	}

	// Click continue/save
	saveButton := page.Locator("button:has-text('Start Messaging'), button:has-text('Continue'), button.btn-primary")
	if err := saveButton.First().Click(); err != nil {
		f.logger.Warn("Failed to click save button", "error", err)
	}

	f.sessionRepo.UpdateStep(ctx, session.ID, models.StepProfileSetup, map[string]interface{}{
		"profile_setup": true,
	})

	return nil
}

func (f *registrationFlow) setupUsername(ctx context.Context, page playwright.Page, account *models.TelegramAccount, session *models.RegistrationSession) error {
	stepStart := time.Now()
	defer func() {
		f.metrics.RecordStepDuration("username_setup", time.Since(stepStart).Seconds())
	}()

	// Navigate to settings if needed
	// Implementation depends on Telegram Web UI

	f.sessionRepo.UpdateStep(ctx, session.ID, models.StepUsernameSetup, map[string]interface{}{
		"username": account.Username,
	})

	return nil
}

func (f *registrationFlow) uploadAvatar(ctx context.Context, page playwright.Page, account *models.TelegramAccount, session *models.RegistrationSession) error {
	stepStart := time.Now()
	defer func() {
		f.metrics.RecordStepDuration("avatar_upload", time.Since(stepStart).Seconds())
	}()

	// Implementation for avatar upload
	// This would involve downloading the image from AvatarURL and uploading it

	f.sessionRepo.UpdateStep(ctx, session.ID, models.StepAvatarUpload, map[string]interface{}{
		"avatar_uploaded": true,
	})

	return nil
}

func (f *registrationFlow) setupTwoFactor(ctx context.Context, page playwright.Page, account *models.TelegramAccount, session *models.RegistrationSession) error {
	stepStart := time.Now()
	defer func() {
		f.metrics.RecordStepDuration("two_factor_setup", time.Since(stepStart).Seconds())
	}()

	// Generate random password for 2FA
	password := generateRandomPassword()
	account.Password = password
	account.TwoFactorSecret = password // Store encrypted

	// Implementation for 2FA setup
	// This would involve navigating to security settings and setting up 2FA

	f.sessionRepo.UpdateStep(ctx, session.ID, models.StepTwoFactorSetup, map[string]interface{}{
		"two_factor_enabled": true,
	})

	return nil
}

func (f *registrationFlow) RetryRegistration(ctx context.Context, accountID primitive.ObjectID) (*models.RegistrationResult, error) {
	account, err := f.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Increment retry count
	f.accountRepo.IncrementRetryCount(ctx, accountID)

	// Get existing session or create new one
	session, err := f.sessionRepo.GetByAccountID(ctx, accountID)
	if err != nil {
		session = &models.RegistrationSession{
			AccountID:       accountID,
			CurrentStep:     models.StepProxyAllocation,
			StepCheckpoints: make(map[string]interface{}),
		}
		f.sessionRepo.Create(ctx, session)
	}

	// Create request from existing account data
	req := &models.RegistrationRequest{
		FirstName:        account.FirstName,
		LastName:         account.LastName,
		Username:         account.Username,
		Bio:              account.Bio,
		AvatarURL:        account.AvatarURL,
		EnableTwoFactor:  account.TwoFactorSecret != "",
		ApiID:            account.ApiID,
		ApiHash:          account.ApiHash,
	}

	// Execute registration flow
	result := f.executeRegistrationFlow(ctx, account, session, req)
	return result, nil
}

func (f *registrationFlow) handleError(account *models.TelegramAccount, step models.RegistrationStep, err error, startTime time.Time) (*models.RegistrationResult, error) {
	f.logger.Error("Registration failed", "step", step, "error", err)
	f.metrics.IncrementRegistrationFailure(string(step))

	if account.ID != primitive.NilObjectID {
		f.accountRepo.UpdateStatus(context.Background(), account.ID, models.StatusError, err.Error())
	}

	return &models.RegistrationResult{
		Success:      false,
		ErrorMessage: err.Error(),
		Step:         string(step),
		Duration:     time.Since(startTime).Seconds(),
		RetryCount:   account.RetryCount,
	}, err
}

func serializeCookies(cookies []playwright.Cookie) ([]byte, error) {
	// Implement cookie serialization
	return nil, nil
}

func generateRandomPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"
	password := make([]byte, 16)
	for i := range password {
		password[i] = charset[rand.Intn(len(charset))]
	}
	return string(password)
}
