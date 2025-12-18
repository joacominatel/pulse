package domain

import (
	"testing"
)

func TestNewWeight_ValidRange(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"minimum", 0.1, false},
		{"maximum", 10.0, false},
		{"mid_range", 5.0, false},
		{"default", 1.0, false},
		{"below_minimum", 0.05, true},
		{"above_maximum", 10.1, true},
		{"zero", 0.0, true},
		{"negative", -1.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight, err := NewWeight(tt.value)

			if tt.wantErr {
				if err != ErrWeightOutOfRange {
					t.Errorf("expected ErrWeightOutOfRange, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if weight.Value() != tt.value {
					t.Errorf("expected %f, got %f", tt.value, weight.Value())
				}
			}
		})
	}
}

func TestEventType_Validation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"join", "join", true},
		{"leave", "leave", true},
		{"post", "post", true},
		{"comment", "comment", true},
		{"reaction", "reaction", true},
		{"share", "share", true},
		{"view", "view", true},
		{"invalid", "invalid_type", false},
		{"empty", "", false},
		{"uppercase", "JOIN", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventType, err := ParseEventType(tt.input)

			if tt.valid {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !eventType.IsValid() {
					t.Error("expected event type to be valid")
				}
			} else if err == nil {
				t.Error("expected error for invalid event type")
			}
		})
	}
}

func TestEventType_DefaultWeight(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  float64
	}{
		{EventTypeJoin, 3.0},
		{EventTypeLeave, 2.0},
		{EventTypePost, 5.0},
		{EventTypeComment, 3.0},
		{EventTypeReaction, 1.0},
		{EventTypeShare, 4.0},
		{EventTypeView, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.eventType.String(), func(t *testing.T) {
			weight := tt.eventType.DefaultWeight()

			if weight.Value() != tt.expected {
				t.Errorf("expected default weight %f, got %f", tt.expected, weight.Value())
			}
		})
	}
}

func TestEventType_IsPositiveSignal(t *testing.T) {
	positiveTypes := []EventType{
		EventTypeJoin, EventTypePost, EventTypeComment,
		EventTypeReaction, EventTypeShare, EventTypeView,
	}
	negativeTypes := []EventType{EventTypeLeave}

	for _, et := range positiveTypes {
		if !et.IsPositiveSignal() {
			t.Errorf("expected %s to be positive signal", et)
		}
	}

	for _, et := range negativeTypes {
		if et.IsPositiveSignal() {
			t.Errorf("expected %s to be negative signal", et)
		}
	}
}

func TestMomentum_ClampedToZero(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{5.0, 5.0},
		{0.0, 0.0},
		{-1.0, 0.0},
		{-100.0, 0.0},
	}

	for _, tt := range tests {
		momentum := NewMomentum(tt.input)
		if momentum.Value() != tt.expected {
			t.Errorf("NewMomentum(%f): expected %f, got %f", tt.input, tt.expected, momentum.Value())
		}
	}
}

func TestMomentum_Add(t *testing.T) {
	m := NewMomentum(5.0)

	added := m.Add(3.0)
	if added.Value() != 8.0 {
		t.Errorf("expected 8.0, got %f", added.Value())
	}

	// subtracting more than current should clamp to zero
	subtracted := m.Add(-10.0)
	if subtracted.Value() != 0.0 {
		t.Errorf("expected 0.0 (clamped), got %f", subtracted.Value())
	}
}

func TestSlug_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid_simple", "my-community", nil},
		{"valid_numbers", "community-123", nil},
		{"valid_minimum", "abc", nil},
		{"empty", "", ErrSlugEmpty},
		{"too_short", "ab", ErrSlugTooShort},
		{"uppercase", "My-Community", ErrSlugInvalid},
		{"spaces", "my community", ErrSlugInvalid},
		{"underscores", "my_community", ErrSlugInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSlug(tt.input)

			if tt.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestUsername_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid_simple", "johndoe", nil},
		{"valid_with_underscore", "john_doe", nil},
		{"valid_with_numbers", "john123", nil},
		{"valid_minimum", "abc", nil},
		{"valid_starts_with_number", "123john", nil}, // allowed per current impl
		{"empty", "", ErrUsernameEmpty},
		{"too_short", "ab", ErrUsernameTooShort},
		{"has_spaces", "john doe", ErrUsernameInvalid},
		{"has_hyphen", "john-doe", ErrUsernameInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewUsername(tt.input)

			if tt.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
