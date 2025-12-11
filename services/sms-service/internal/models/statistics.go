package models

// GetStatisticsResponse is returned by HTTP endpoint /api/v1/statistics
// and used internally by the repository/service layer.
//
// Note: gRPC uses proto-level GetStatisticsResponse; this type is for internal/HTTP usage.
type GetStatisticsResponse struct {
	TotalActivations      int32             `json:"total_activations"`
	SuccessfulActivations int32             `json:"successful_activations"`
	FailedActivations     int32             `json:"failed_activations"`
	CancelledActivations  int32             `json:"cancelled_activations"`
	TotalSpent            float32           `json:"total_spent"`
	AveragePrice          float32           `json:"average_price"`
	ByService             map[string]int32  `json:"by_service"`
	ByCountry             map[string]int32  `json:"by_country"`
	ByProvider            map[string]float32 `json:"by_provider"`
}


