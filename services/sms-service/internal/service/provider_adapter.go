package service

import (
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
)

type ProviderAdapter struct {
	logger    *logrus.Logger
	providers map[string]ProviderConfig
}

type ProviderConfig struct {
	Name     string
	Priority int
	Services []string
	Countries []string
	Enabled  bool
}

func NewProviderAdapter(logger *logrus.Logger) *ProviderAdapter {
	// Default configuration - should be loaded from config file
	providers := map[string]ProviderConfig{
		"smsactivate": {
			Name:     "smsactivate",
			Priority: 1,
			Services: []string{"all"},
			Countries: []string{"all"},
			Enabled:  true,
		},
	}

	return &ProviderAdapter{
		logger:    logger,
		providers: providers,
	}
}

func (pa *ProviderAdapter) SelectProvider(service, country string) string {
	// Simple selection logic - can be enhanced
	availableProviders := []string{}

	for name, config := range pa.providers {
		if !config.Enabled {
			continue
		}

		// Check if provider supports service and country
		if pa.supportsService(config, service) && pa.supportsCountry(config, country) {
			availableProviders = append(availableProviders, name)
		}
	}

	if len(availableProviders) == 0 {
		return "smsactivate" // Default fallback
	}

	// Random selection for load balancing
	rand.Seed(time.Now().UnixNano())
	return availableProviders[rand.Intn(len(availableProviders))]
}

func (pa *ProviderAdapter) supportsService(config ProviderConfig, service string) bool {
	for _, s := range config.Services {
		if s == "all" || s == service {
			return true
		}
	}
	return false
}

func (pa *ProviderAdapter) supportsCountry(config ProviderConfig, country string) bool {
	for _, c := range config.Countries {
		if c == "all" || c == country {
			return true
		}
	}
	return false
}
