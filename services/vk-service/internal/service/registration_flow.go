package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"conveer/pkg/crypto"
	"conveer/pkg/logger"
	"conveer/services/vk-service/internal/models"
	"conveer/services/vk-service/internal/repository"
	proxypb "conveer/services/proxy-service/proto"
	smspb "conveer/services/sms-service/proto"

	"github.com/playwright-community/playwright-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RegistrationFlow interface {
	RegisterAccount(ctx context.Context, accountID primitive.ObjectID, request *models.RegistrationRequest) (*models.RegistrationResult, error)
	RetryRegistration(ctx context.Context, accountID primitive.ObjectID) (*models.RegistrationResult, error)
}

type registrationFlow struct {
	accountRepo      repository.AccountRepository
	sessionRepo      repository.SessionRepository
	browserManager   BrowserManager
	stealthInjector  StealthInjector
	fingerprintGen   FingerprintGenerator
	proxyClient      proxypb.ProxyServiceClient
	smsClient        smspb.SMSServiceClient
	encryptor        crypto.Encryptor
	passwordGen      crypto.PasswordGenerator
	config           *models.RegistrationConfig
	messagingClient  interface{ PublishToQueue(string, interface{}) error }
	logger           logger.Logger
}

func NewRegistrationFlow(
	accountRepo repository.AccountRepository,
	sessionRepo repository.SessionRepository,
	browserManager BrowserManager,
	stealthInjector StealthInjector,
	fingerprintGen FingerprintGenerator,
	proxyClient proxypb.ProxyServiceClient,
	smsClient smspb.SMSServiceClient,
	encryptor crypto.Encryptor,
	passwordGen crypto.PasswordGenerator,
	config *models.RegistrationConfig,
	messagingClient interface{ PublishToQueue(string, interface{}) error },
	logger logger.Logger,
) RegistrationFlow {
	return &registrationFlow{
		accountRepo:      accountRepo,
		sessionRepo:      sessionRepo,
		browserManager:   browserManager,
		stealthInjector:  stealthInjector,
		fingerprintGen:   fingerprintGen,
		proxyClient:      proxyClient,
		smsClient:        smsClient,
		encryptor:        encryptor,
		passwordGen:      passwordGen,
		config:           config,
		messagingClient:  messagingClient,
		logger:           logger,
	}
}

func (f *registrationFlow) RegisterAccount(ctx context.Context, accountID primitive.ObjectID, request *models.RegistrationRequest) (*models.RegistrationResult, error) {
	startTime := time.Now()

	// Get or create session
	session, err := f.sessionRepo.GetSession(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if session == nil {
		session = &models.RegistrationSession{
			AccountID:   accountID,
			CurrentStep: models.StepProxyAllocation,
			StartedAt:   time.Now(),
			RetryCount:  0,
			StepCheckpoints: make(map[string]interface{}),
		}
		if err := f.sessionRepo.SaveSession(ctx, session); err != nil {
			return nil, fmt.Errorf("failed to save session: %w", err)
		}
	}

	// Execute registration steps
	result := &models.RegistrationResult{
		AccountID:  accountID.Hex(),
		RetryCount: session.RetryCount,
	}

	// Step 1: Allocate Proxy
	if session.CurrentStep == models.StepProxyAllocation {
		if err := f.allocateProxy(ctx, accountID, session); err != nil {
			f.handleStepError(ctx, accountID, session, models.StepProxyAllocation, err)
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("proxy allocation failed: %v", err)
			result.Step = string(models.StepProxyAllocation)
			return result, nil
		}
		session.CurrentStep = models.StepPhonePurchase
		f.sessionRepo.UpdateSession(ctx, accountID, bson.M{"current_step": session.CurrentStep})
	}

	// Step 2: Purchase Phone Number
	if session.CurrentStep == models.StepPhonePurchase {
		if err := f.purchasePhoneNumber(ctx, accountID, session, request); err != nil {
			f.handleStepError(ctx, accountID, session, models.StepPhonePurchase, err)
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("phone purchase failed: %v", err)
			result.Step = string(models.StepPhonePurchase)
			return result, nil
		}
		session.CurrentStep = models.StepFormFilling
		f.sessionRepo.UpdateSession(ctx, accountID, bson.M{"current_step": session.CurrentStep})
	}

	// Step 3-6: Browser automation
	browser, browserCtx, err := f.setupBrowser(ctx, session)
	if err != nil {
		f.handleStepError(ctx, accountID, session, session.CurrentStep, err)
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("browser setup failed: %v", err)
		result.Step = string(session.CurrentStep)
		return result, nil
	}
	defer f.cleanupBrowser(browser, browserCtx)

	page, err := browserCtx.NewPage()
	if err != nil {
		f.handleStepError(ctx, accountID, session, session.CurrentStep, err)
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("page creation failed: %v", err)
		return result, nil
	}

	// Inject stealth
	if err := f.stealthInjector.InjectStealth(page); err != nil {
		f.logger.Warn("Failed to inject stealth", "error", err)
	}

	// Step 3: Fill Registration Form
	if session.CurrentStep == models.StepFormFilling {
		if err := f.fillRegistrationForm(ctx, page, session, request); err != nil {
			f.handleStepError(ctx, accountID, session, models.StepFormFilling, err)
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("form filling failed: %v", err)
			result.Step = string(models.StepFormFilling)
			return result, nil
		}
		session.CurrentStep = models.StepSMSVerification
		f.sessionRepo.UpdateSession(ctx, accountID, bson.M{"current_step": session.CurrentStep})
	}

	// Step 4: SMS Verification
	if session.CurrentStep == models.StepSMSVerification {
		if err := f.verifySMSCode(ctx, page, session); err != nil {
			f.handleStepError(ctx, accountID, session, models.StepSMSVerification, err)
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("SMS verification failed: %v", err)
			result.Step = string(models.StepSMSVerification)
			return result, nil
		}
		session.CurrentStep = models.StepProfileSetup
		f.sessionRepo.UpdateSession(ctx, accountID, bson.M{"current_step": session.CurrentStep})
	}

	// Step 5: Profile Setup
	if session.CurrentStep == models.StepProfileSetup {
		password := f.passwordGen.GenerateSecure(16)
		if err := f.setupProfile(ctx, page, session, password); err != nil {
			f.handleStepError(ctx, accountID, session, models.StepProfileSetup, err)
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("profile setup failed: %v", err)
			result.Step = string(models.StepProfileSetup)
			return result, nil
		}

		// Save credentials
		cookies, err := f.extractCookies(browserCtx)
		if err != nil {
			f.logger.Warn("Failed to extract cookies", "error", err)
		}

		userID := f.extractUserID(page)

		// Update account with credentials
		if err := f.saveAccountCredentials(ctx, accountID, session.Phone, password, cookies, userID); err != nil {
			f.logger.Error("Failed to save account credentials", "error", err)
		}

		session.CurrentStep = models.StepComplete
		completedAt := time.Now()
		session.CompletedAt = &completedAt
		f.sessionRepo.UpdateSession(ctx, accountID, bson.M{
			"current_step": session.CurrentStep,
			"completed_at": completedAt,
		})
	}

	// Success
	result.Success = true
	result.UserID = f.extractUserID(page)
	result.Phone = session.Phone
	result.Duration = time.Since(startTime).Seconds()

	// Update account status
	f.accountRepo.UpdateAccountStatus(ctx, accountID, models.StatusCreated, "")

	f.logger.Info("Account registration completed successfully",
		"account_id", accountID,
		"user_id", result.UserID,
		"duration", result.Duration)

	return result, nil
}

func (f *registrationFlow) RetryRegistration(ctx context.Context, accountID primitive.ObjectID) (*models.RegistrationResult, error) {
	// Get account details
	account, err := f.accountRepo.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Increment retry count
	if err := f.accountRepo.IncrementRetryCount(ctx, accountID); err != nil {
		f.logger.Error("Failed to increment retry count", "error", err)
	}

	// Check retry limit
	if account.RetryCount >= f.config.MaxRetryAttempts {
		f.accountRepo.UpdateAccountStatus(ctx, accountID, models.StatusError, "max retries exceeded")
		return &models.RegistrationResult{
			Success:      false,
			AccountID:    accountID.Hex(),
			ErrorMessage: "maximum retry attempts exceeded",
			RetryCount:   account.RetryCount,
		}, nil
	}

	// Create registration request from account data
	request := &models.RegistrationRequest{
		FirstName: account.FirstName,
		LastName:  account.LastName,
	}

	// Set gender if available
	if account.Gender != "" {
		request.Gender = models.Gender(account.Gender)
	} else if account.Fingerprint != nil {
		// Fallback to fingerprint if gender not in account
		if genderVal, ok := account.Fingerprint["gender"].(string); ok {
			request.Gender = models.Gender(genderVal)
		}
	}

	// Set birth date if available
	if account.BirthDate != nil {
		request.BirthDate = *account.BirthDate
	}

	// Retry registration
	return f.RegisterAccount(ctx, accountID, request)
}

func (f *registrationFlow) allocateProxy(ctx context.Context, accountID primitive.ObjectID, session *models.RegistrationSession) error {
	// Call proxy service to allocate proxy
	resp, err := f.proxyClient.AllocateProxy(ctx, &proxypb.AllocateProxyRequest{
		AccountId: accountID.Hex(),
		Type:      "mobile",
		Country:   "RU",
	})
	if err != nil {
		return fmt.Errorf("failed to allocate proxy: %w", err)
	}

	// Save proxy details in session
	session.ProxyID, _ = primitive.ObjectIDFromHex(resp.Proxy.Id)
	session.ProxyURL = fmt.Sprintf("%s://%s:%d", resp.Proxy.Protocol, resp.Proxy.Host, resp.Proxy.Port)

	// Update account with proxy ID
	f.accountRepo.UpdateAccount(ctx, accountID, bson.M{"proxy_id": session.ProxyID})

	f.logger.Info("Proxy allocated", "account_id", accountID, "proxy_id", session.ProxyID)
	return nil
}

func (f *registrationFlow) purchasePhoneNumber(ctx context.Context, accountID primitive.ObjectID, session *models.RegistrationSession, request *models.RegistrationRequest) error {
	country := request.PreferredCountry
	if country == "" {
		country = "RU"
	}

	// Call SMS service to purchase number
	resp, err := f.smsClient.PurchaseNumber(ctx, &smspb.PurchaseNumberRequest{
		Service:   "vk",
		Country:   country,
		AccountId: accountID.Hex(),
	})
	if err != nil {
		return fmt.Errorf("failed to purchase phone number: %w", err)
	}

	// Save phone details in session
	session.Phone = resp.Phone
	session.ActivationID = resp.ActivationId

	// Update account with phone (encrypted)
	f.accountRepo.UpdateAccount(ctx, accountID, bson.M{
		"phone": session.Phone,
		"activation_id": session.ActivationID,
	})

	f.logger.Info("Phone number purchased", "account_id", accountID, "phone", session.Phone[:3]+"***")
	return nil
}

func (f *registrationFlow) setupBrowser(ctx context.Context, session *models.RegistrationSession) (playwright.Browser, playwright.BrowserContext, error) {
	proxyConfig := &ProxyConfig{
		Server: session.ProxyURL,
	}

	// Parse proxy URL for credentials if present
	if strings.Contains(session.ProxyURL, "@") {
		// Extract username:password from URL
		parts := strings.Split(session.ProxyURL, "://")
		if len(parts) == 2 {
			authAndHost := strings.Split(parts[1], "@")
			if len(authAndHost) == 2 {
				creds := strings.Split(authAndHost[0], ":")
				if len(creds) == 2 {
					proxyConfig.Username = creds[0]
					proxyConfig.Password = creds[1]
					proxyConfig.Server = parts[0] + "://" + authAndHost[1]
				}
			}
		}
	}

	browser, browserCtx, err := f.browserManager.AcquireBrowser(ctx, proxyConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to acquire browser: %w", err)
	}

	// Generate and apply fingerprint
	fingerprint := f.fingerprintGen.GenerateFingerprint()
	if err := f.fingerprintGen.ApplyFingerprint(browserCtx, fingerprint); err != nil {
		f.logger.Warn("Failed to apply fingerprint", "error", err)
	}

	// Save fingerprint to account
	fingerprintData := map[string]interface{}{
		"user_agent": fingerprint.UserAgent,
		"viewport":   fingerprint.Viewport,
		"timezone":   fingerprint.Timezone,
		"locale":     fingerprint.Locale,
		"platform":   fingerprint.Platform,
	}
	f.accountRepo.UpdateAccount(ctx, session.AccountID, bson.M{"fingerprint": fingerprintData})

	return browser, browserCtx, nil
}

func (f *registrationFlow) fillRegistrationForm(ctx context.Context, page playwright.Page, session *models.RegistrationSession, request *models.RegistrationRequest) error {
	// Navigate to VK registration page
	if _, err := page.Goto("https://vk.com/join", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(f.config.PageLoadTimeout.Seconds() * 1000),
	}); err != nil {
		return fmt.Errorf("failed to navigate to registration page: %w", err)
	}

	// Emulate human behavior
	f.stealthInjector.EmulateHumanBehavior(page)

	// Wait for form to load
	if err := page.WaitForSelector("#ij_form", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(30000),
	}); err != nil {
		return fmt.Errorf("registration form not found: %w", err)
	}

	// Fill first name
	firstNameInput := page.Locator("input[name='first_name']")
	if err := firstNameInput.Click(); err != nil {
		return fmt.Errorf("failed to click first name input: %w", err)
	}
	time.Sleep(f.stealthInjector.RandomDelay(f.config.FormFillDelayMin, f.config.FormFillDelayMax))
	firstNameHandle, err := firstNameInput.ElementHandle()
	if err != nil {
		return fmt.Errorf("failed to get first name element handle: %w", err)
	}
	if err := f.stealthInjector.TypeWithHumanSpeed(firstNameHandle, request.FirstName); err != nil {
		return fmt.Errorf("failed to type first name: %w", err)
	}

	// Fill last name
	lastNameInput := page.Locator("input[name='last_name']")
	if err := lastNameInput.Click(); err != nil {
		return fmt.Errorf("failed to click last name input: %w", err)
	}
	time.Sleep(f.stealthInjector.RandomDelay(f.config.FormFillDelayMin, f.config.FormFillDelayMax))
	lastNameHandle, err := lastNameInput.ElementHandle()
	if err != nil {
		return fmt.Errorf("failed to get last name element handle: %w", err)
	}
	if err := f.stealthInjector.TypeWithHumanSpeed(lastNameHandle, request.LastName); err != nil {
		return fmt.Errorf("failed to type last name: %w", err)
	}

	// Fill birth date
	if !request.BirthDate.IsZero() {
		daySelect := page.Locator("select[name='bday']")
		monthSelect := page.Locator("select[name='bmonth']")
		yearSelect := page.Locator("select[name='byear']")

		daySelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{fmt.Sprintf("%d", request.BirthDate.Day())},
		})
		time.Sleep(f.stealthInjector.RandomDelay(200, 500))

		monthSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{fmt.Sprintf("%d", request.BirthDate.Month())},
		})
		time.Sleep(f.stealthInjector.RandomDelay(200, 500))

		yearSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{fmt.Sprintf("%d", request.BirthDate.Year())},
		})
		time.Sleep(f.stealthInjector.RandomDelay(200, 500))
	}

	// Select gender
	if request.Gender != "" {
		genderValue := "2" // male
		if request.Gender == models.GenderFemale {
			genderValue = "1"
		}
		genderRadio := page.Locator(fmt.Sprintf("input[name='sex'][value='%s']", genderValue))
		if err := genderRadio.Click(); err != nil {
			f.logger.Warn("Failed to select gender", "error", err)
		}
		time.Sleep(f.stealthInjector.RandomDelay(200, 500))
	}

	// Fill phone number
	phoneInput := page.Locator("input[name='phone']")
	if err := phoneInput.Click(); err != nil {
		return fmt.Errorf("failed to click phone input: %w", err)
	}
	time.Sleep(f.stealthInjector.RandomDelay(f.config.FormFillDelayMin, f.config.FormFillDelayMax))
	phoneHandle, err := phoneInput.ElementHandle()
	if err != nil {
		return fmt.Errorf("failed to get phone element handle: %w", err)
	}
	if err := f.stealthInjector.TypeWithHumanSpeed(phoneHandle, session.Phone); err != nil {
		return fmt.Errorf("failed to type phone: %w", err)
	}

	// Click continue/get code button
	time.Sleep(f.stealthInjector.RandomDelay(1000, 2000))
	continueBtn := page.Locator("button[type='submit'], .FlatButton__content:has-text('Получить код')")
	if err := continueBtn.Click(); err != nil {
		return fmt.Errorf("failed to click continue button: %w", err)
	}

	f.logger.Info("Registration form filled", "account_id", session.AccountID)
	return nil
}

func (f *registrationFlow) verifySMSCode(ctx context.Context, page playwright.Page, session *models.RegistrationSession) error {
	// Wait for SMS code input to appear
	if err := page.WaitForSelector("input[name='code'], input[placeholder*='код']", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(60000),
	}); err != nil {
		return fmt.Errorf("SMS code input not found: %w", err)
	}

	// Poll for SMS code
	var smsCode string
	maxPolls := f.config.MaxSMSPolls
	if maxPolls == 0 {
		maxPolls = 30
	}

	for i := 0; i < maxPolls; i++ {
		resp, err := f.smsClient.GetSMSCode(ctx, &smspb.GetSMSCodeRequest{
			ActivationId: session.ActivationID,
		})
		if err != nil {
			f.logger.Warn("Failed to get SMS code", "attempt", i+1, "error", err)
			time.Sleep(f.config.SMSPollingInterval)
			continue
		}

		if resp.Code != "" {
			smsCode = resp.Code
			break
		}

		if resp.Status == "cancelled" {
			return fmt.Errorf("SMS activation cancelled")
		}

		time.Sleep(f.config.SMSPollingInterval)
	}

	if smsCode == "" {
		// Cancel activation
		f.smsClient.CancelActivation(ctx, &smspb.CancelActivationRequest{
			ActivationId: session.ActivationID,
		})
		return fmt.Errorf("SMS code not received within timeout")
	}

	// Enter SMS code
	codeInput := page.Locator("input[name='code'], input[placeholder*='код']").First()
	if err := codeInput.Click(); err != nil {
		return fmt.Errorf("failed to click code input: %w", err)
	}
	time.Sleep(f.stealthInjector.RandomDelay(500, 1000))

	// Type code with delays
	for _, digit := range smsCode {
		if err := codeInput.Type(string(digit)); err != nil {
			return fmt.Errorf("failed to type SMS code digit: %w", err)
		}
		time.Sleep(f.stealthInjector.RandomDelay(100, 300))
	}

	// Submit code
	time.Sleep(f.stealthInjector.RandomDelay(1000, 2000))
	submitBtn := page.Locator("button[type='submit'], .FlatButton__content:has-text('Продолжить')")
	if err := submitBtn.Click(); err != nil {
		// Try pressing Enter
		if err := codeInput.Press("Enter"); err != nil {
			return fmt.Errorf("failed to submit SMS code: %w", err)
		}
	}

	f.logger.Info("SMS verification completed", "account_id", session.AccountID)
	return nil
}

func (f *registrationFlow) setupProfile(ctx context.Context, page playwright.Page, session *models.RegistrationSession, password string) error {
	// Wait for password field
	if err := page.WaitForSelector("input[type='password'], input[name='password']", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(30000),
	}); err != nil {
		return fmt.Errorf("password field not found: %w", err)
	}

	// Set password
	passwordInput := page.Locator("input[type='password'], input[name='password']").First()
	if err := passwordInput.Click(); err != nil {
		return fmt.Errorf("failed to click password input: %w", err)
	}
	time.Sleep(f.stealthInjector.RandomDelay(500, 1000))
	passwordHandle, err := passwordInput.ElementHandle()
	if err != nil {
		return fmt.Errorf("failed to get password element handle: %w", err)
	}
	if err := f.stealthInjector.TypeWithHumanSpeed(passwordHandle, password); err != nil {
		return fmt.Errorf("failed to type password: %w", err)
	}

	// Confirm password if needed
	confirmInput := page.Locator("input[name='password_confirm'], input[placeholder*='Повторите']")
	if count, _ := confirmInput.Count(); count > 0 {
		if err := confirmInput.Click(); err != nil {
			f.logger.Warn("Failed to click confirm password", "error", err)
		}
		time.Sleep(f.stealthInjector.RandomDelay(500, 1000))
		confirmHandle, err := confirmInput.ElementHandle()
		if err != nil {
			f.logger.Warn("Failed to get confirm password element handle", "error", err)
		} else if err := f.stealthInjector.TypeWithHumanSpeed(confirmHandle, password); err != nil {
			f.logger.Warn("Failed to type confirm password", "error", err)
		}
	}

	// Submit password
	time.Sleep(f.stealthInjector.RandomDelay(1000, 2000))
	submitBtn := page.Locator("button[type='submit'], .FlatButton__content:has-text('Готово'), .FlatButton__content:has-text('Продолжить')")
	if err := submitBtn.Click(); err != nil {
		f.logger.Warn("Failed to click submit button", "error", err)
		// Try pressing Enter
		passwordInput.Press("Enter")
	}

	// Wait for redirect to profile or feed
	time.Sleep(5 * time.Second)

	// Skip optional steps (photo upload, friend suggestions)
	skipBtn := page.Locator(".FlatButton__content:has-text('Пропустить'), a:has-text('Пропустить')")
	for i := 0; i < 3; i++ {
		if count, _ := skipBtn.Count(); count > 0 {
			skipBtn.First().Click()
			time.Sleep(2 * time.Second)
		}
	}

	// Navigate to main page to ensure we're logged in
	page.Goto("https://vk.com/feed", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	})

	f.logger.Info("Profile setup completed", "account_id", session.AccountID)
	return nil
}

func (f *registrationFlow) extractCookies(ctx playwright.BrowserContext) ([]byte, error) {
	cookies, err := ctx.Cookies()
	if err != nil {
		return nil, fmt.Errorf("failed to get cookies: %w", err)
	}

	// Convert playwright cookies to our Cookie model
	var modelCookies []models.Cookie
	for _, c := range cookies {
		modelCookie := models.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HTTPOnly: c.HttpOnly,
			Secure:   c.Secure,
		}
		if c.Expires > 0 {
			modelCookie.Expires = time.Unix(int64(c.Expires), 0)
		}
		if c.SameSite != nil {
			modelCookie.SameSite = string(*c.SameSite)
		}
		modelCookies = append(modelCookies, modelCookie)
	}

	return json.Marshal(modelCookies)
}

func (f *registrationFlow) extractUserID(page playwright.Page) string {
	// Try to get user ID from page URL or content
	url := page.URL()
	if strings.Contains(url, "id") {
		parts := strings.Split(url, "id")
		if len(parts) > 1 {
			// Extract numeric ID
			var userID string
			for _, char := range parts[1] {
				if char >= '0' && char <= '9' {
					userID += string(char)
				} else {
					break
				}
			}
			if userID != "" {
				return userID
			}
		}
	}

	// Try to get from page content
	userIDElement := page.Locator("a[href*='/id']").First()
	if href, err := userIDElement.GetAttribute("href"); err == nil && href != "" {
		if strings.Contains(href, "id") {
			parts := strings.Split(href, "id")
			if len(parts) > 1 {
				var userID string
				for _, char := range parts[1] {
					if char >= '0' && char <= '9' {
						userID += string(char)
					} else {
						break
					}
				}
				if userID != "" {
					return userID
				}
			}
		}
	}

	return ""
}

func (f *registrationFlow) saveAccountCredentials(ctx context.Context, accountID primitive.ObjectID, phone, password string, cookies []byte, userID string) error {
	// Use the new method that properly encrypts all sensitive fields
	return f.accountRepo.UpdateAccountFullCredentials(ctx, accountID, phone, password, cookies, userID, models.StatusCreated)
}

func (f *registrationFlow) handleStepError(ctx context.Context, accountID primitive.ObjectID, session *models.RegistrationSession, step models.RegistrationStep, err error) {
	f.logger.Error("Registration step failed",
		"account_id", accountID,
		"step", step,
		"error", err)

	// Update session with error
	f.sessionRepo.UpdateSession(ctx, accountID, bson.M{
		"last_error": err.Error(),
		"current_step": step,
	})

	// Check if error requires manual intervention
	errStr := strings.ToLower(err.Error())
	requiresManualIntervention := false
	interventionReason := ""

	// Common patterns that require manual intervention
	if strings.Contains(errStr, "captcha") || strings.Contains(errStr, "капча") {
		requiresManualIntervention = true
		interventionReason = "Captcha detected"
	} else if strings.Contains(errStr, "blocked") || strings.Contains(errStr, "banned") {
		requiresManualIntervention = true
		interventionReason = "Account blocked or banned"
	} else if strings.Contains(errStr, "phone already used") || strings.Contains(errStr, "phone in use") {
		requiresManualIntervention = true
		interventionReason = "Phone number already in use"
	} else if strings.Contains(errStr, "suspicious activity") || strings.Contains(errStr, "подозрительная") {
		requiresManualIntervention = true
		interventionReason = "Suspicious activity detected"
	} else if session.RetryCount >= 3 && step == models.StepSMSVerification {
		requiresManualIntervention = true
		interventionReason = "SMS verification failed after multiple attempts"
	}

	// Publish to manual intervention queue if needed
	if requiresManualIntervention && f.messagingClient != nil {
		message := map[string]interface{}{
			"account_id":   accountID.Hex(),
			"reason":       interventionReason,
			"step":         string(step),
			"error":        err.Error(),
			"session_id":   session.ID.Hex(),
			"retry_count":  session.RetryCount,
			"timestamp":    time.Now(),
		}

		if pubErr := f.messagingClient.PublishToQueue("vk.manual_intervention", message); pubErr != nil {
			f.logger.Error("Failed to publish manual intervention request",
				"account_id", accountID,
				"reason", interventionReason,
				"error", pubErr)
		} else {
			f.logger.Info("Manual intervention requested",
				"account_id", accountID,
				"reason", interventionReason)

			// Update account status to suspended
			f.accountRepo.UpdateAccountStatus(ctx, accountID, models.StatusSuspended,
				fmt.Sprintf("Manual intervention: %s", interventionReason))
		}
	}

	// Release resources if needed
	if step == models.StepPhonePurchase && session.ActivationID != "" {
		// Cancel SMS activation
		if _, cancelErr := f.smsClient.CancelActivation(ctx, &smspb.CancelActivationRequest{
			ActivationId: session.ActivationID,
		}); cancelErr != nil {
			f.logger.Error("Failed to cancel activation", "error", cancelErr)
		}
	}

	if step != models.StepProxyAllocation && session.ProxyID != primitive.NilObjectID {
		// Release proxy
		if _, releaseErr := f.proxyClient.ReleaseProxy(ctx, &proxypb.ReleaseProxyRequest{
			AccountId: accountID.Hex(),
		}); releaseErr != nil {
			f.logger.Error("Failed to release proxy", "error", releaseErr)
		}
	}
}

func (f *registrationFlow) cleanupBrowser(browser playwright.Browser, ctx playwright.BrowserContext) {
	if ctx != nil {
		if err := ctx.Close(); err != nil {
			f.logger.Warn("Failed to close browser context", "error", err)
		}
	}
	if browser != nil {
		if err := f.browserManager.ReleaseBrowser(browser); err != nil {
			f.logger.Warn("Failed to release browser", "error", err)
		}
	}
}