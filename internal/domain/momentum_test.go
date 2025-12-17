package domain

import (
	"testing"
	"time"
)

func TestCalculateMomentum_EmptyEvents(t *testing.T) {
	input := MomentumInput{
		Events:      nil,
		WindowStart: time.Now().Add(-1 * time.Hour),
		WindowEnd:   time.Now(),
		DecayFactor: 0.7,
	}

	result := CalculateMomentum(input)

	if result.Score.Value() != 0 {
		t.Errorf("expected score 0, got %f", result.Score.Value())
	}
	if result.EventCount != 0 {
		t.Errorf("expected event count 0, got %d", result.EventCount)
	}
}

func TestCalculateMomentum_SingleEventAtWindowEnd(t *testing.T) {
	now := time.Now()
	input := MomentumInput{
		Events: []MomentumEventData{
			{Weight: 1.0, CreatedAt: now, IsNegative: false},
		},
		WindowStart: now.Add(-1 * time.Hour),
		WindowEnd:   now,
		DecayFactor: 0.7,
	}

	result := CalculateMomentum(input)

	// event at window end should have no decay (multiplier = 1.0)
	if result.Score.Value() != 1.0 {
		t.Errorf("expected score 1.0, got %f", result.Score.Value())
	}
}

func TestCalculateMomentum_SingleEventAtWindowStart(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-1 * time.Hour)
	input := MomentumInput{
		Events: []MomentumEventData{
			{Weight: 1.0, CreatedAt: windowStart, IsNegative: false},
		},
		WindowStart: windowStart,
		WindowEnd:   now,
		DecayFactor: 0.7,
	}

	result := CalculateMomentum(input)

	// event at window start should have full decay (multiplier = 0.7)
	expected := 0.7
	if result.Score.Value() != expected {
		t.Errorf("expected score %f, got %f", expected, result.Score.Value())
	}
}

func TestCalculateMomentum_EventAtWindowMidpoint(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-1 * time.Hour)
	midpoint := now.Add(-30 * time.Minute)
	input := MomentumInput{
		Events: []MomentumEventData{
			{Weight: 1.0, CreatedAt: midpoint, IsNegative: false},
		},
		WindowStart: windowStart,
		WindowEnd:   now,
		DecayFactor: 0.7,
	}

	result := CalculateMomentum(input)

	// at midpoint, age_ratio = 0.5, so multiplier = 1 - 0.5*(1-0.7) = 0.85
	expected := 0.85
	tolerance := 0.001
	if result.Score.Value() < expected-tolerance || result.Score.Value() > expected+tolerance {
		t.Errorf("expected score ~%f, got %f", expected, result.Score.Value())
	}
}

func TestCalculateMomentum_NegativeEventSubtracts(t *testing.T) {
	now := time.Now()
	input := MomentumInput{
		Events: []MomentumEventData{
			{Weight: 2.0, CreatedAt: now, IsNegative: false},
			{Weight: 1.0, CreatedAt: now, IsNegative: true},
		},
		WindowStart: now.Add(-1 * time.Hour),
		WindowEnd:   now,
		DecayFactor: 1.0, // no decay for simplicity
	}

	result := CalculateMomentum(input)

	// 2.0 - 1.0 = 1.0
	if result.Score.Value() != 1.0 {
		t.Errorf("expected score 1.0, got %f", result.Score.Value())
	}
}

func TestCalculateMomentum_ClampedToZero(t *testing.T) {
	now := time.Now()
	input := MomentumInput{
		Events: []MomentumEventData{
			{Weight: 1.0, CreatedAt: now, IsNegative: false},
			{Weight: 5.0, CreatedAt: now, IsNegative: true},
		},
		WindowStart: now.Add(-1 * time.Hour),
		WindowEnd:   now,
		DecayFactor: 1.0,
	}

	result := CalculateMomentum(input)

	// 1.0 - 5.0 = -4.0, clamped to 0
	if result.Score.Value() != 0 {
		t.Errorf("expected score 0 (clamped), got %f", result.Score.Value())
	}
	// raw sum should still be negative
	if result.RawSum >= 0 {
		t.Errorf("expected negative raw sum, got %f", result.RawSum)
	}
}

func TestCalculateMomentum_InvalidWindowReturnsZero(t *testing.T) {
	now := time.Now()
	input := MomentumInput{
		Events: []MomentumEventData{
			{Weight: 1.0, CreatedAt: now, IsNegative: false},
		},
		WindowStart: now,
		WindowEnd:   now.Add(-1 * time.Hour), // end before start
		DecayFactor: 0.7,
	}

	result := CalculateMomentum(input)

	if result.Score.Value() != 0 {
		t.Errorf("expected score 0 for invalid window, got %f", result.Score.Value())
	}
}

func TestSimpleMomentum(t *testing.T) {
	tests := []struct {
		name        string
		weightedSum float64
		decayFactor float64
		expected    float64
	}{
		{"positive_sum", 10.0, 0.7, 7.0},
		{"negative_sum_clamped", -5.0, 0.7, 0.0},
		{"zero_sum", 0.0, 0.7, 0.0},
		{"no_decay", 10.0, 1.0, 10.0},
		{"full_decay", 10.0, 0.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SimpleMomentum(tt.weightedSum, tt.decayFactor)
			if result.Value() != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result.Value())
			}
		})
	}
}
