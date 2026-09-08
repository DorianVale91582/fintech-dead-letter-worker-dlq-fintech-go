package payments

import "testing"

func TestDecide(t *testing.T) {
	tests := []struct {
		name  string
		event Event
		want  Action
	}{
		{"transient failure can retry", Event{Attempt: 2, FailureClass: "processor"}, Retry},
		{"exhausted failure is dead lettered", Event{Attempt: 5, FailureClass: "processor"}, DeadLetter},
		{"risk failure skips further attempts", Event{Attempt: 1, FailureClass: "risk"}, ManualReview},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Decide(tt.event, 5); got.Action != tt.want {
				t.Fatalf("Decide() action = %q, want %q", got.Action, tt.want)
			}
		})
	}
}
