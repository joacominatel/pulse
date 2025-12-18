package domain

import "time"

// MomentumInput represents the input data for momentum calculation.
// all data is provided upfront - no side effects or time acquisition inside.
type MomentumInput struct {
	// Events in the calculation window, already filtered by time.
	Events []MomentumEventData

	// WindowStart is the beginning of the sliding window.
	WindowStart time.Time

	// WindowEnd is the end of the sliding window (typically "now").
	WindowEnd time.Time

	// DecayFactor controls how quickly old events lose weight.
	// 1.0 means no decay, 0.5 means events at window edge count half.
	DecayFactor float64
}

// MomentumEventData is a minimal representation of an event for momentum calculation.
// decoupled from the full ActivityEvent to keep the algorithm pure.
type MomentumEventData struct {
	Weight    float64
	CreatedAt time.Time
	// IsNegative indicates if this event subtracts from momentum (e.g., leave).
	IsNegative bool
}

// MomentumResult contains the output of momentum calculation.
type MomentumResult struct {
	// Score is the final momentum value.
	Score Momentum

	// RawSum is the unscaled sum of weighted contributions.
	RawSum float64

	// EventCount is the number of events considered.
	EventCount int

	// EffectiveDecay is the average decay factor applied.
	EffectiveDecay float64
}

// CalculateMomentum computes community momentum from activity events.
// this is a pure function with no side effects - all inputs are explicit.
//
// algorithm:
// 1. for each event, compute its age within the window
// 2. apply time-based decay (newer events count more)
// 3. sum the weighted contributions (positive or negative)
// 4. clamp the result to non-negative
//
// the decay formula is: contribution = weight * (1 - age_ratio * (1 - decay_factor))
// where age_ratio = (window_end - event_time) / window_duration
//
// example with decay_factor=0.7:
// - event at window_end (age_ratio=0): contribution = weight * 1.0
// - event at window_start (age_ratio=1): contribution = weight * 0.7
func CalculateMomentum(input *MomentumInput) MomentumResult {
	if len(input.Events) == 0 {
		return MomentumResult{
			Score:          NewMomentum(0),
			RawSum:         0,
			EventCount:     0,
			EffectiveDecay: input.DecayFactor,
		}
	}

	windowDuration := input.WindowEnd.Sub(input.WindowStart)
	if windowDuration <= 0 {
		// invalid window, return zero
		return MomentumResult{
			Score:          NewMomentum(0),
			RawSum:         0,
			EventCount:     len(input.Events),
			EffectiveDecay: input.DecayFactor,
		}
	}

	var rawSum float64
	var totalDecay float64

	for _, event := range input.Events {
		// calculate age ratio (0 = newest, 1 = oldest in window)
		age := input.WindowEnd.Sub(event.CreatedAt)
		ageRatio := float64(age) / float64(windowDuration)

		// clamp age ratio to [0, 1]
		if ageRatio < 0 {
			ageRatio = 0
		}
		if ageRatio > 1 {
			ageRatio = 1
		}

		// apply decay: at age_ratio=0 we get 1.0, at age_ratio=1 we get decay_factor
		decayMultiplier := 1 - ageRatio*(1-input.DecayFactor)
		totalDecay += decayMultiplier

		// compute contribution
		contribution := event.Weight * decayMultiplier
		if event.IsNegative {
			contribution = -contribution
		}

		rawSum += contribution
	}

	effectiveDecay := totalDecay / float64(len(input.Events))

	return MomentumResult{
		Score:          NewMomentum(rawSum),
		RawSum:         rawSum,
		EventCount:     len(input.Events),
		EffectiveDecay: effectiveDecay,
	}
}

// SimpleMomentum calculates momentum using a simpler model without per-event decay.
// useful when events are pre-aggregated in the database.
//
// formula: momentum = weighted_sum * decay_factor
// where weighted_sum accounts for positive and negative event types.
func SimpleMomentum(weightedSum, decayFactor float64) Momentum {
	return NewMomentum(weightedSum * decayFactor)
}
