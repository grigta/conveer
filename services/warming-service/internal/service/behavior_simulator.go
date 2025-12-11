package service

import (
	"math"
	"math/rand"
	"time"

	"conveer/pkg/logger"
	"conveer/services/warming-service/internal/config"
)

type BehaviorSimulator struct {
	config *config.Config
	logger logger.Logger
	rand   *rand.Rand
}

func NewBehaviorSimulator(cfg *config.Config, logger logger.Logger) *BehaviorSimulator {
	return &BehaviorSimulator{
		config: cfg,
		logger: logger,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *BehaviorSimulator) CalculateNextActionTime(currentTime time.Time, behaviorConfig config.BehaviorSimulationConfig) time.Time {
	if !behaviorConfig.EnableRandomDelays {
		// Fixed interval (5 minutes)
		return currentTime.Add(5 * time.Minute)
	}

	// Base delay with random variation
	baseDelay := b.generateRandomDelay(behaviorConfig.DelayMinSeconds, behaviorConfig.DelayMaxSeconds)

	// Apply time-of-day factor
	timeFactor := b.getTimeOfDayFactor(currentTime)
	adjustedDelay := time.Duration(float64(baseDelay) * timeFactor)

	// Apply burst pattern occasionally
	if b.shouldApplyBurstPattern() {
		// Quick action in a burst (30-90 seconds)
		adjustedDelay = time.Duration(b.rand.Intn(60)+30) * time.Second
	}

	nextTime := currentTime.Add(adjustedDelay)

	// Apply human-like patterns
	nextTime = b.applyHumanPatterns(nextTime)

	return nextTime
}

func (b *BehaviorSimulator) generateRandomDelay(minSeconds, maxSeconds int) time.Duration {
	// Use exponential distribution for more realistic delays
	// Most delays are short, with occasional long pauses
	lambda := 1.0 / float64(maxSeconds-minSeconds)
	exponentialDelay := -math.Log(1.0-b.rand.Float64()) / lambda

	// Clamp to min/max range
	delaySeconds := int(exponentialDelay) + minSeconds
	if delaySeconds > maxSeconds {
		delaySeconds = maxSeconds
	}

	return time.Duration(delaySeconds) * time.Second
}

func (b *BehaviorSimulator) getTimeOfDayFactor(t time.Time) float64 {
	hour := t.Hour()

	// Peak activity hours (10-12, 14-16, 19-21)
	peakHours := map[int]bool{
		10: true, 11: true, 12: true,
		14: true, 15: true, 16: true,
		19: true, 20: true, 21: true,
	}

	if peakHours[hour] {
		return 0.7 // Faster during peak hours
	}

	// Slow hours (early morning, late night)
	if hour < 8 || hour > 22 {
		return 2.0 // Slower during off-hours
	}

	return 1.0 // Normal speed
}

func (b *BehaviorSimulator) shouldApplyBurstPattern() bool {
	// 15% chance of being in a burst pattern
	return b.rand.Float64() < 0.15
}

func (b *BehaviorSimulator) applyHumanPatterns(t time.Time) time.Time {
	// Round to nearest minute sometimes (humans don't act on exact seconds)
	if b.rand.Float64() < 0.3 {
		t = t.Round(time.Minute)
	}

	// Prefer round numbers (5, 10, 15, 20, etc.)
	if b.rand.Float64() < 0.2 {
		minute := t.Minute()
		roundedMinute := ((minute + 4) / 5) * 5
		if roundedMinute == 60 {
			t = t.Add(time.Hour).Truncate(time.Hour)
		} else {
			diff := roundedMinute - minute
			t = t.Add(time.Duration(diff) * time.Minute)
		}
	}

	return t
}

func (b *BehaviorSimulator) ShouldSkipAction(currentTime time.Time, behaviorConfig config.BehaviorSimulationConfig) bool {
	hour := currentTime.Hour()
	dayOfWeek := currentTime.Weekday()

	// Check if outside active hours
	if hour < behaviorConfig.ActiveHoursStart || hour >= behaviorConfig.ActiveHoursEnd {
		// High probability of skipping during night hours
		if b.rand.Float64() < behaviorConfig.NightPauseProbability {
			b.logger.Debug("Skipping action due to night hours")
			return true
		}
	}

	// Weekend activity reduction
	if dayOfWeek == time.Saturday || dayOfWeek == time.Sunday {
		if b.rand.Float64() > behaviorConfig.WeekendActivityReduction {
			b.logger.Debug("Skipping action due to weekend reduction")
			return true
		}
	}

	// Random skip to make behavior more natural (5% chance)
	if b.rand.Float64() < 0.05 {
		b.logger.Debug("Random skip for natural behavior")
		return true
	}

	return false
}

func (b *BehaviorSimulator) GenerateActionSequence(actionsPerDay int, behaviorConfig config.BehaviorSimulationConfig) []time.Time {
	var sequence []time.Time

	// Start from beginning of active hours
	currentTime := time.Now().Truncate(24 * time.Hour).Add(
		time.Duration(behaviorConfig.ActiveHoursStart) * time.Hour,
	)

	// Add some initial randomness (0-30 minutes)
	currentTime = currentTime.Add(time.Duration(b.rand.Intn(30)) * time.Minute)

	actionsScheduled := 0
	attempts := 0
	maxAttempts := actionsPerDay * 3

	for actionsScheduled < actionsPerDay && attempts < maxAttempts {
		attempts++

		// Check if we should skip this time slot
		if b.ShouldSkipAction(currentTime, behaviorConfig) {
			currentTime = b.CalculateNextActionTime(currentTime, behaviorConfig)
			continue
		}

		// Add to sequence
		sequence = append(sequence, currentTime)
		actionsScheduled++

		// Calculate next action time
		currentTime = b.CalculateNextActionTime(currentTime, behaviorConfig)

		// If we've gone past active hours, stop
		if currentTime.Hour() >= behaviorConfig.ActiveHoursEnd {
			break
		}
	}

	return sequence
}

func (b *BehaviorSimulator) AddHumanDelay(minMs, maxMs int) time.Duration {
	// Human reaction time with natural variation
	baseDelay := b.rand.Intn(maxMs-minMs) + minMs

	// Add micro-variations (0-500ms)
	microVariation := b.rand.Intn(500)

	totalDelay := baseDelay + microVariation

	return time.Duration(totalDelay) * time.Millisecond
}

func (b *BehaviorSimulator) GenerateBurstPattern(minActions, maxActions int) int {
	// Generate number of actions in a burst
	burstSize := b.rand.Intn(maxActions-minActions+1) + minActions

	// Apply gaussian distribution for more realistic bursts
	// Most bursts are medium-sized, few are very small or very large
	mean := float64(minActions+maxActions) / 2
	stdDev := float64(maxActions-minActions) / 4

	gaussianBurst := int(b.rand.NormFloat64()*stdDev + mean)

	// Clamp to range
	if gaussianBurst < minActions {
		gaussianBurst = minActions
	}
	if gaussianBurst > maxActions {
		gaussianBurst = maxActions
	}

	return gaussianBurst
}

func (b *BehaviorSimulator) SimulateScrollDelay() time.Duration {
	// Simulate human scrolling behavior
	// Fast scroll: 100-300ms
	// Normal scroll: 300-800ms
	// Slow/reading scroll: 800-2000ms

	scrollType := b.rand.Float64()

	if scrollType < 0.2 {
		// Fast scroll (20%)
		return b.AddHumanDelay(100, 300)
	} else if scrollType < 0.7 {
		// Normal scroll (50%)
		return b.AddHumanDelay(300, 800)
	} else {
		// Slow/reading scroll (30%)
		return b.AddHumanDelay(800, 2000)
	}
}

func (b *BehaviorSimulator) SimulateTypingDelay(textLength int) time.Duration {
	// Average typing speed: 40 words per minute
	// Approximately 200 characters per minute
	// That's about 300ms per character

	baseDelayPerChar := 300 // milliseconds

	// Add variation (-100ms to +200ms per character)
	variation := b.rand.Intn(300) - 100

	delayPerChar := baseDelayPerChar + variation

	// Calculate total delay
	totalDelay := textLength * delayPerChar

	// Add thinking pauses (every 5-10 characters)
	pauseCount := textLength / (5 + b.rand.Intn(5))
	pauseDelay := pauseCount * (500 + b.rand.Intn(1500)) // 0.5-2 seconds per pause

	totalDelay += pauseDelay

	return time.Duration(totalDelay) * time.Millisecond
}

func (b *BehaviorSimulator) SimulateReadingDelay(textLength int) time.Duration {
	// Average reading speed: 250 words per minute
	// Approximately 1250 characters per minute
	// That's about 48ms per character

	baseDelayPerChar := 48 // milliseconds

	// Add variation based on complexity
	variation := b.rand.Intn(20) - 10
	delayPerChar := baseDelayPerChar + variation

	totalDelay := textLength * delayPerChar

	return time.Duration(totalDelay) * time.Millisecond
}

func (b *BehaviorSimulator) GetActivityLevel(t time.Time) float64 {
	hour := t.Hour()

	// Define activity levels throughout the day
	activityLevels := map[int]float64{
		0: 0.1, 1: 0.05, 2: 0.05, 3: 0.05, 4: 0.05, 5: 0.1,
		6: 0.2, 7: 0.4, 8: 0.6, 9: 0.8, 10: 0.9, 11: 0.95,
		12: 1.0, 13: 0.8, 14: 0.85, 15: 0.9, 16: 0.85, 17: 0.8,
		18: 0.7, 19: 0.85, 20: 0.9, 21: 0.8, 22: 0.6, 23: 0.3,
	}

	baseLevel := activityLevels[hour]

	// Apply day of week factor
	dayOfWeek := t.Weekday()
	if dayOfWeek == time.Saturday || dayOfWeek == time.Sunday {
		baseLevel *= b.config.WarmingConfig.BehaviorSimulation.WeekendActivityReduction
	}

	// Add some randomness (±10%)
	randomFactor := 0.9 + b.rand.Float64()*0.2

	return baseLevel * randomFactor
}