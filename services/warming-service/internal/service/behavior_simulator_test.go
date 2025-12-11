package service

import (
	"testing"
	"time"

	"github.com/grigta/conveer/services/warming-service/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// BehaviorSimulatorTestSuite is the test suite for BehaviorSimulator
type BehaviorSimulatorTestSuite struct {
	suite.Suite
	logger *MockLogger
	config *config.Config
}

func (s *BehaviorSimulatorTestSuite) SetupTest() {
	s.logger = new(MockLogger)
	s.config = &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				EnableRandomDelays:       true,
				DelayMinSeconds:          30,
				DelayMaxSeconds:          300,
				ActiveHoursStart:         8,
				ActiveHoursEnd:           22,
				NightPauseProbability:    0.9,
				WeekendActivityReduction: 0.7,
			},
		},
	}
}

func TestBehaviorSimulatorTestSuite(t *testing.T) {
	suite.Run(t, new(BehaviorSimulatorTestSuite))
}

// Test NewBehaviorSimulator
func (s *BehaviorSimulatorTestSuite) TestNewBehaviorSimulator() {
	sim := NewBehaviorSimulator(s.config, s.logger)
	s.NotNil(sim)
	s.NotNil(sim.rand)
}

// Test CalculateNextActionTime with random delays enabled
func (s *BehaviorSimulatorTestSuite) TestCalculateNextActionTime_RandomDelaysEnabled() {
	sim := NewBehaviorSimulator(s.config, s.logger)
	currentTime := time.Now()

	nextTime := sim.CalculateNextActionTime(currentTime, s.config.WarmingConfig.BehaviorSimulation)

	// Next time should be after current time
	s.True(nextTime.After(currentTime))
}

// Test CalculateNextActionTime with random delays disabled
func (s *BehaviorSimulatorTestSuite) TestCalculateNextActionTime_RandomDelaysDisabled() {
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				EnableRandomDelays: false,
			},
		},
	}

	sim := NewBehaviorSimulator(cfg, s.logger)
	currentTime := time.Now()

	nextTime := sim.CalculateNextActionTime(currentTime, cfg.WarmingConfig.BehaviorSimulation)

	// Should be exactly 5 minutes later when random delays are disabled
	expectedTime := currentTime.Add(5 * time.Minute)
	s.Equal(expectedTime, nextTime)
}

// Test generateRandomDelay
func (s *BehaviorSimulatorTestSuite) TestGenerateRandomDelay() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	// Test multiple times to ensure it's within range
	for i := 0; i < 100; i++ {
		delay := sim.generateRandomDelay(30, 300)
		s.True(delay >= 30*time.Second)
		s.True(delay <= 300*time.Second)
	}
}

// Test getTimeOfDayFactor - peak hours
func (s *BehaviorSimulatorTestSuite) TestGetTimeOfDayFactor_PeakHours() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	peakHours := []int{10, 11, 12, 14, 15, 16, 19, 20, 21}
	for _, hour := range peakHours {
		testTime := time.Date(2024, 1, 15, hour, 0, 0, 0, time.Local)
		factor := sim.getTimeOfDayFactor(testTime)
		s.Equal(0.7, factor, "Hour %d should have peak factor", hour)
	}
}

// Test getTimeOfDayFactor - slow hours
func (s *BehaviorSimulatorTestSuite) TestGetTimeOfDayFactor_SlowHours() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	slowHours := []int{2, 3, 4, 5, 23}
	for _, hour := range slowHours {
		testTime := time.Date(2024, 1, 15, hour, 0, 0, 0, time.Local)
		factor := sim.getTimeOfDayFactor(testTime)
		s.Equal(2.0, factor, "Hour %d should have slow factor", hour)
	}
}

// Test getTimeOfDayFactor - normal hours
func (s *BehaviorSimulatorTestSuite) TestGetTimeOfDayFactor_NormalHours() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	normalHours := []int{9, 13, 17, 18}
	for _, hour := range normalHours {
		testTime := time.Date(2024, 1, 15, hour, 0, 0, 0, time.Local)
		factor := sim.getTimeOfDayFactor(testTime)
		s.Equal(1.0, factor, "Hour %d should have normal factor", hour)
	}
}

// Test shouldApplyBurstPattern
func (s *BehaviorSimulatorTestSuite) TestShouldApplyBurstPattern() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	// Run multiple times to check probability
	burstCount := 0
	iterations := 1000
	for i := 0; i < iterations; i++ {
		if sim.shouldApplyBurstPattern() {
			burstCount++
		}
	}

	// With 15% probability, expect roughly 150 bursts (with some variance)
	burstPercentage := float64(burstCount) / float64(iterations)
	s.True(burstPercentage > 0.10 && burstPercentage < 0.20, 
		"Burst percentage %.2f should be around 15%%", burstPercentage)
}

// Test applyHumanPatterns
func (s *BehaviorSimulatorTestSuite) TestApplyHumanPatterns() {
	sim := NewBehaviorSimulator(s.config, s.logger)
	testTime := time.Date(2024, 1, 15, 12, 37, 45, 0, time.Local)

	// Run multiple times and check that time is still reasonable
	for i := 0; i < 100; i++ {
		adjustedTime := sim.applyHumanPatterns(testTime)
		
		// Time should be within a reasonable range of original
		diff := adjustedTime.Sub(testTime)
		s.True(diff >= -5*time.Minute && diff <= 5*time.Minute)
	}
}

// Test ShouldSkipAction - night hours
func (s *BehaviorSimulatorTestSuite) TestShouldSkipAction_NightHours() {
	sim := NewBehaviorSimulator(s.config, s.logger)
	nightTime := time.Date(2024, 1, 15, 3, 0, 0, 0, time.Local) // 3 AM Monday

	// With 90% skip probability at night, should skip most of the time
	skippedCount := 0
	for i := 0; i < 100; i++ {
		if sim.ShouldSkipAction(nightTime, s.config.WarmingConfig.BehaviorSimulation) {
			skippedCount++
		}
	}

	s.True(skippedCount > 80, "Should skip most actions at night")
}

// Test ShouldSkipAction - weekend
func (s *BehaviorSimulatorTestSuite) TestShouldSkipAction_Weekend() {
	sim := NewBehaviorSimulator(s.config, s.logger)
	// Saturday at noon
	weekendTime := time.Date(2024, 1, 20, 12, 0, 0, 0, time.Local)
	s.Equal(time.Saturday, weekendTime.Weekday())

	// With 70% activity reduction, should skip some actions
	skippedCount := 0
	for i := 0; i < 100; i++ {
		if sim.ShouldSkipAction(weekendTime, s.config.WarmingConfig.BehaviorSimulation) {
			skippedCount++
		}
	}

	// Should skip around 30% of actions on weekends
	s.True(skippedCount > 15 && skippedCount < 50)
}

// Test ShouldSkipAction - weekday during active hours
func (s *BehaviorSimulatorTestSuite) TestShouldSkipAction_WeekdayActiveHours() {
	sim := NewBehaviorSimulator(s.config, s.logger)
	// Wednesday at noon
	weekdayTime := time.Date(2024, 1, 17, 12, 0, 0, 0, time.Local)
	s.Equal(time.Wednesday, weekdayTime.Weekday())

	// Should rarely skip during weekday active hours (only random 5% skip)
	skippedCount := 0
	for i := 0; i < 100; i++ {
		if sim.ShouldSkipAction(weekdayTime, s.config.WarmingConfig.BehaviorSimulation) {
			skippedCount++
		}
	}

	// Should skip around 5% of actions
	s.True(skippedCount < 15)
}

// Test GenerateActionSequence
func (s *BehaviorSimulatorTestSuite) TestGenerateActionSequence() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	sequence := sim.GenerateActionSequence(10, s.config.WarmingConfig.BehaviorSimulation)

	// Should generate some actions (might not be exactly 10 due to skips)
	s.True(len(sequence) > 0)
	s.True(len(sequence) <= 10)

	// All times should be within active hours
	for _, t := range sequence {
		hour := t.Hour()
		s.True(hour >= 8 && hour < 22, "Time %v should be within active hours", t)
	}
}

// Test AddHumanDelay
func (s *BehaviorSimulatorTestSuite) TestAddHumanDelay() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	// Test multiple times to ensure it's within range
	for i := 0; i < 100; i++ {
		delay := sim.AddHumanDelay(100, 500)
		// Delay should be between min and max + micro-variation (up to 500ms)
		s.True(delay >= 100*time.Millisecond)
		s.True(delay <= 1000*time.Millisecond)
	}
}

// Test GenerateBurstPattern
func (s *BehaviorSimulatorTestSuite) TestGenerateBurstPattern() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	// Test multiple times
	for i := 0; i < 100; i++ {
		burstSize := sim.GenerateBurstPattern(3, 7)
		s.True(burstSize >= 3)
		s.True(burstSize <= 7)
	}
}

// Test SimulateScrollDelay
func (s *BehaviorSimulatorTestSuite) TestSimulateScrollDelay() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	// Test distribution of scroll types
	fastCount := 0
	normalCount := 0
	slowCount := 0

	for i := 0; i < 1000; i++ {
		delay := sim.SimulateScrollDelay()
		if delay < 350*time.Millisecond {
			fastCount++
		} else if delay < 850*time.Millisecond {
			normalCount++
		} else {
			slowCount++
		}
	}

	// Should have roughly 20% fast, 50% normal, 30% slow
	s.True(fastCount > 100 && fastCount < 300, "Fast scroll count: %d", fastCount)
	s.True(normalCount > 400 && normalCount < 600, "Normal scroll count: %d", normalCount)
	s.True(slowCount > 200 && slowCount < 400, "Slow scroll count: %d", slowCount)
}

// Test SimulateTypingDelay
func (s *BehaviorSimulatorTestSuite) TestSimulateTypingDelay() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	tests := []struct {
		name       string
		textLength int
	}{
		{"short text", 10},
		{"medium text", 50},
		{"long text", 200},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			delay := sim.SimulateTypingDelay(tt.textLength)
			
			// Delay should be proportional to text length
			minExpected := time.Duration(tt.textLength*200) * time.Millisecond
			maxExpected := time.Duration(tt.textLength*500) * time.Millisecond
			
			s.True(delay >= minExpected, "Delay %v should be >= %v for length %d", delay, minExpected, tt.textLength)
		})
	}
}

// Test SimulateReadingDelay
func (s *BehaviorSimulatorTestSuite) TestSimulateReadingDelay() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	tests := []struct {
		name       string
		textLength int
	}{
		{"short text", 100},
		{"medium text", 500},
		{"long text", 2000},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			delay := sim.SimulateReadingDelay(tt.textLength)
			
			// Reading should be faster than typing
			// ~48ms per character
			minExpected := time.Duration(tt.textLength*38) * time.Millisecond
			maxExpected := time.Duration(tt.textLength*58) * time.Millisecond
			
			s.True(delay >= minExpected && delay <= maxExpected,
				"Delay %v should be between %v and %v", delay, minExpected, maxExpected)
		})
	}
}

// Test GetActivityLevel
func (s *BehaviorSimulatorTestSuite) TestGetActivityLevel() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	tests := []struct {
		name     string
		hour     int
		minLevel float64
		maxLevel float64
	}{
		{"night", 2, 0.0, 0.1},
		{"morning", 9, 0.6, 1.0},
		{"noon", 12, 0.8, 1.1},
		{"afternoon", 15, 0.7, 1.0},
		{"evening", 20, 0.7, 1.0},
		{"late night", 23, 0.2, 0.5},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			testTime := time.Date(2024, 1, 15, tt.hour, 30, 0, 0, time.Local) // Monday
			level := sim.GetActivityLevel(testTime)
			
			s.True(level >= tt.minLevel && level <= tt.maxLevel,
				"Activity level %.2f at hour %d should be between %.2f and %.2f",
				level, tt.hour, tt.minLevel, tt.maxLevel)
		})
	}
}

// Test GetActivityLevel - weekend reduction
func (s *BehaviorSimulatorTestSuite) TestGetActivityLevel_Weekend() {
	sim := NewBehaviorSimulator(s.config, s.logger)

	// Compare same hour on weekday vs weekend
	weekdayTime := time.Date(2024, 1, 17, 12, 0, 0, 0, time.Local) // Wednesday
	weekendTime := time.Date(2024, 1, 20, 12, 0, 0, 0, time.Local) // Saturday

	weekdayLevel := sim.GetActivityLevel(weekdayTime)
	weekendLevel := sim.GetActivityLevel(weekendTime)

	// Weekend level should be lower (multiplied by 0.7)
	s.True(weekendLevel < weekdayLevel, 
		"Weekend level %.2f should be less than weekday level %.2f",
		weekendLevel, weekdayLevel)
}

// Benchmark tests
func BenchmarkCalculateNextActionTime(b *testing.B) {
	logger := new(MockLogger)
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				EnableRandomDelays: true,
				DelayMinSeconds:    30,
				DelayMaxSeconds:    300,
			},
		},
	}

	sim := NewBehaviorSimulator(cfg, logger)
	currentTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sim.CalculateNextActionTime(currentTime, cfg.WarmingConfig.BehaviorSimulation)
	}
}

func BenchmarkGenerateActionSequence(b *testing.B) {
	logger := new(MockLogger)
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				EnableRandomDelays:    true,
				DelayMinSeconds:       30,
				DelayMaxSeconds:       300,
				ActiveHoursStart:      8,
				ActiveHoursEnd:        22,
				NightPauseProbability: 0.9,
				WeekendActivityReduction: 0.7,
			},
		},
	}

	sim := NewBehaviorSimulator(cfg, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sim.GenerateActionSequence(10, cfg.WarmingConfig.BehaviorSimulation)
	}
}

func BenchmarkShouldSkipAction(b *testing.B) {
	logger := new(MockLogger)
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				ActiveHoursStart:         8,
				ActiveHoursEnd:           22,
				NightPauseProbability:    0.9,
				WeekendActivityReduction: 0.7,
			},
		},
	}

	sim := NewBehaviorSimulator(cfg, logger)
	testTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sim.ShouldSkipAction(testTime, cfg.WarmingConfig.BehaviorSimulation)
	}
}

func BenchmarkGetActivityLevel(b *testing.B) {
	logger := new(MockLogger)
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				WeekendActivityReduction: 0.7,
			},
		},
	}

	sim := NewBehaviorSimulator(cfg, logger)
	testTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sim.GetActivityLevel(testTime)
	}
}

