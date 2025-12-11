package service

import (
	"context"
	"encoding/json"
	"fmt"

	vkpb "github.com/conveer/conveer/services/vk-service/proto"
	"github.com/playwright-community/playwright-go"
)

// VKIntegration handles VK account integration
type VKIntegration struct {
	vkClient vkpb.VKServiceClient
}

// NewVKIntegration creates a new VK integration
func NewVKIntegration(vkClient vkpb.VKServiceClient) *VKIntegration {
	return &VKIntegration{
		vkClient: vkClient,
	}
}

// CheckVKAccount verifies VK account exists and is ready
func (v *VKIntegration) CheckVKAccount(ctx context.Context, vkAccountID string) error {
	resp, err := v.vkClient.GetAccount(ctx, &vkpb.GetAccountRequest{
		AccountId: vkAccountID,
	})
	if err != nil {
		return fmt.Errorf("failed to get VK account: %w", err)
	}
	
	// Check account status
	if resp.Status != "created" && resp.Status != "warming" && resp.Status != "ready" {
		return fmt.Errorf("VK account not ready, status: %s", resp.Status)
	}
	
	return nil
}

// CreateVKAccount creates a new VK account
func (v *VKIntegration) CreateVKAccount(ctx context.Context, request *VKAccountRequest) (*VKAccountResult, error) {
	// Note: BirthDate needs to be converted to timestamp if it's a string
	// For now, passing nil for BirthDate
	resp, err := v.vkClient.CreateAccount(ctx, &vkpb.CreateAccountRequest{
		FirstName:        request.FirstName,
		LastName:         request.LastName,
		// BirthDate:        request.BirthDate, // TODO: Convert string to timestamp
		Gender:           request.Gender,
		PreferredCountry: request.PreferredCountry,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create VK account: %w", err)
	}

	return &VKAccountResult{
		AccountID: resp.Id,      // Use 'Id' field from Account message
		Status:    resp.Status,
	}, nil
}

// GetVKCredentials retrieves VK credentials
func (v *VKIntegration) GetVKCredentials(ctx context.Context, vkAccountID string) (*VKCredentials, error) {
	// First get basic account info
	accResp, err := v.vkClient.GetAccount(ctx, &vkpb.GetAccountRequest{
		AccountId: vkAccountID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get VK account: %w", err)
	}

	// Then get sensitive credentials
	credResp, err := v.vkClient.GetAccountCredentials(ctx, &vkpb.GetAccountRequest{
		AccountId: vkAccountID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get VK credentials: %w", err)
	}

	return &VKCredentials{
		UserID:   accResp.UserId,
		Phone:    accResp.Phone,
		Password: credResp.Password,
		Cookies:  credResp.Cookies,
	}, nil
}

// LoginToVK logs into VK using existing credentials
func (v *VKIntegration) LoginToVK(ctx context.Context, page playwright.Page, vkAccount *VKCredentials) error {
	// Load cookies if available - try session restore first
	if vkAccount.Cookies != "" {
		var cookies []playwright.Cookie
		if err := json.Unmarshal([]byte(vkAccount.Cookies), &cookies); err == nil {
			// Add cookies to browser context
			if err := page.Context().AddCookies(cookies...); err != nil {
				return fmt.Errorf("failed to add cookies: %w", err)
			}
		}
	}
	
	// Navigate to VK
	if _, err := page.Goto("https://vk.com", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return fmt.Errorf("failed to navigate to VK: %w", err)
	}
	
	// Check if already logged in
	if isLoggedIn, err := page.Locator("#l_pr").Count(); err == nil && isLoggedIn > 0 {
		// Already logged in via cookies
		return nil
	}

	// Need to login - check if password is available
	if vkAccount.Password == "" {
		return fmt.Errorf("VK password not available for login and cookies didn't restore session")
	}

	// Proceed with password login
	if err := page.Click("#index_login_button"); err != nil {
		// Try alternative login button
		if err := page.Click("button[type='submit']"); err != nil {
			return fmt.Errorf("failed to click login button: %w", err)
		}
	}
	
	// Enter phone
	if err := TypeWithHumanSpeed(page, "input[name='login']", vkAccount.Phone); err != nil {
		return fmt.Errorf("failed to enter phone: %w", err)
	}
	
	// Click continue/next
	if err := page.Click("button[type='submit']"); err != nil {
		return fmt.Errorf("failed to submit phone: %w", err)
	}
	
	// Wait for password field
	if err := page.WaitForSelector("input[name='password']", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	}); err != nil {
		return fmt.Errorf("password field not found: %w", err)
	}
	
	// Enter password
	if err := TypeWithHumanSpeed(page, "input[name='password']", vkAccount.Password); err != nil {
		return fmt.Errorf("failed to enter password: %w", err)
	}
	
	// Submit login
	if err := page.Click("button[type='submit']"); err != nil {
		return fmt.Errorf("failed to submit login: %w", err)
	}
	
	// Wait for navigation
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("failed to wait for login: %w", err)
	}
	
	// Verify login successful
	if isLoggedIn, err := page.Locator("#l_pr").Count(); err != nil || isLoggedIn == 0 {
		return fmt.Errorf("login failed")
	}
	
	return nil
}

// VKAccountRequest represents a request to create VK account
type VKAccountRequest struct {
	FirstName        string
	LastName         string
	BirthDate        string
	Gender           string
	PreferredCountry string
}

// VKAccountResult represents VK account creation result
type VKAccountResult struct {
	AccountID string
	Status    string
}

// VKCredentials represents VK account credentials
type VKCredentials struct {
	UserID   string
	Phone    string
	Password string
	Cookies  string
}
