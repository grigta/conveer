package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/grigta/conveer/services/sms-service/internal/models"

	"github.com/sirupsen/logrus"
)

type SMSActivateClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
	logger  *logrus.Logger
}

func NewSMSActivateClient(apiKey string, logger *logrus.Logger) *SMSActivateClient {
	return &SMSActivateClient{
		apiKey:  apiKey,
		baseURL: "https://api.sms-activate.org/stubs/handler_api.php",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (c *SMSActivateClient) PurchaseNumber(ctx context.Context, service, country, operator string, maxPrice float64) (*models.Phone, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("action", "getNumber")
	params.Set("service", c.mapService(service))
	params.Set("country", c.mapCountry(country))

	if operator != "" {
		params.Set("operator", operator)
	}
	if maxPrice > 0 {
		params.Set("maxPrice", fmt.Sprintf("%.2f", maxPrice))
	}

	resp, err := c.makeRequest(ctx, params)
	if err != nil {
		return nil, err
	}

	// Parse response: ACCESS_NUMBER:123456789:1234567890
	parts := strings.Split(resp, ":")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid response format: %s", resp)
	}

	if parts[0] != "ACCESS_NUMBER" {
		return nil, fmt.Errorf("failed to get number: %s", resp)
	}

	activationID := parts[1]
	phoneNumber := parts[2]

	// Get price info
	price := c.getPrice(service, country)

	phone := &models.Phone{
		Number:       phoneNumber,
		CountryCode:  c.getCountryCode(country),
		Country:      country,
		Operator:     operator,
		Provider:     "smsactivate",
		Service:      service,
		Price:        price,
		ActivationID: activationID,
	}

	return phone, nil
}

func (c *SMSActivateClient) GetSMSCode(ctx context.Context, activationID string) (string, string, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("action", "getStatus")
	params.Set("id", activationID)

	resp, err := c.makeRequest(ctx, params)
	if err != nil {
		return "", "", err
	}

	// Parse response
	if resp == "STATUS_WAIT_CODE" {
		return "", "", fmt.Errorf("waiting for code")
	}

	if strings.HasPrefix(resp, "STATUS_OK:") {
		code := strings.TrimPrefix(resp, "STATUS_OK:")
		return c.extractCode(code), code, nil
	}

	if strings.HasPrefix(resp, "STATUS_CANCEL") {
		return "", "", fmt.Errorf("activation cancelled")
	}

	return "", "", fmt.Errorf("unexpected status: %s", resp)
}

func (c *SMSActivateClient) CancelActivation(ctx context.Context, activationID string) (bool, float64, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("action", "setStatus")
	params.Set("id", activationID)
	params.Set("status", "8") // Cancel status

	resp, err := c.makeRequest(ctx, params)
	if err != nil {
		return false, 0, err
	}

	if resp == "ACCESS_CANCEL" {
		// Get refund amount (simplified - should query actual refund)
		return true, c.getPrice("", ""), nil
	}

	return false, 0, fmt.Errorf("failed to cancel: %s", resp)
}

func (c *SMSActivateClient) GetBalance(ctx context.Context) (float64, string, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("action", "getBalance")

	resp, err := c.makeRequest(ctx, params)
	if err != nil {
		return 0, "", err
	}

	// Parse response: ACCESS_BALANCE:100.50
	if strings.HasPrefix(resp, "ACCESS_BALANCE:") {
		balanceStr := strings.TrimPrefix(resp, "ACCESS_BALANCE:")
		balance, err := strconv.ParseFloat(balanceStr, 64)
		if err != nil {
			return 0, "", err
		}
		return balance, "RUB", nil
	}

	return 0, "", fmt.Errorf("failed to get balance: %s", resp)
}

func (c *SMSActivateClient) makeRequest(ctx context.Context, params url.Values) (string, error) {
	url := c.baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (c *SMSActivateClient) mapService(service string) string {
	// Map internal service names to SMSActivate service codes
	serviceMap := map[string]string{
		"whatsapp":  "wa",
		"telegram":  "tg",
		"google":    "go",
		"facebook":  "fb",
		"instagram": "ig",
		"twitter":   "tw",
		"vk":        "vk",
		"other":     "ot",
	}

	if code, ok := serviceMap[strings.ToLower(service)]; ok {
		return code
	}
	return "ot" // Default to "other"
}

func (c *SMSActivateClient) mapCountry(country string) string {
	// Map country codes (simplified)
	countryMap := map[string]string{
		"US": "0",   // USA
		"RU": "1",   // Russia
		"KZ": "2",   // Kazakhstan
		"CN": "3",   // China
		"PH": "4",   // Philippines
		"MM": "5",   // Myanmar
		"ID": "6",   // Indonesia
		"MY": "7",   // Malaysia
		"KE": "8",   // Kenya
		"TZ": "9",   // Tanzania
		"VN": "10",  // Vietnam
		"GB": "16",  // United Kingdom
		"LV": "49",  // Latvia
		"PL": "15",  // Poland
		"UA": "46",  // Ukraine
	}

	if code, ok := countryMap[strings.ToUpper(country)]; ok {
		return code
	}
	return "0" // Default to USA
}

func (c *SMSActivateClient) getCountryCode(country string) string {
	// Get country dial code
	codeMap := map[string]string{
		"US": "+1",
		"RU": "+7",
		"KZ": "+7",
		"CN": "+86",
		"PH": "+63",
		"MM": "+95",
		"ID": "+62",
		"MY": "+60",
		"KE": "+254",
		"TZ": "+255",
		"VN": "+84",
		"GB": "+44",
		"LV": "+371",
		"PL": "+48",
		"UA": "+380",
	}

	if code, ok := codeMap[strings.ToUpper(country)]; ok {
		return code
	}
	return "+1"
}

func (c *SMSActivateClient) getPrice(service, country string) float64 {
	// Simplified pricing - should fetch actual prices from API
	basePrice := 10.0 // RUB

	// Adjust by service
	serviceMultiplier := map[string]float64{
		"whatsapp":  1.5,
		"telegram":  1.2,
		"google":    2.0,
		"facebook":  1.8,
		"instagram": 1.6,
	}

	if mult, ok := serviceMultiplier[strings.ToLower(service)]; ok {
		basePrice *= mult
	}

	return basePrice
}

func (c *SMSActivateClient) extractCode(fullSMS string) string {
	// Extract numeric code from SMS
	// Simple implementation - can be enhanced with regex
	parts := strings.Fields(fullSMS)
	for _, part := range parts {
		if len(part) >= 4 && len(part) <= 8 {
			if _, err := strconv.Atoi(part); err == nil {
				return part
			}
		}
	}
	return fullSMS // Return full SMS if no code found
}
