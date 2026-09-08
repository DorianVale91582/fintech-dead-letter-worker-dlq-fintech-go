package payments

import "time"

type Event struct {
	PaymentID    string `json:"payment_id"`
	AmountCents  int64  `json:"amount_cents"`
	Currency     string `json:"currency"`
	Attempt      int    `json:"attempt"`
	FailureClass string `json:"failure_class"`
}

type Action string

const (
	Retry        Action = "retry"
	DeadLetter   Action = "dead_letter"
	ManualReview Action = "manual_review"
)

type Decision struct {
	Action Action `json:"action"`
	Reason string `json:"reason"`
}

type Notification struct {
	PaymentID   string    `json:"payment_id"`
	MessageID   string    `json:"message_id"`
	Action      Action    `json:"action"`
	Reason      string    `json:"reason"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	RecordedAt  time.Time `json:"recorded_at"`
}

func Decide(event Event, maxAttempts int) Decision {
	if event.FailureClass == "risk" {
		return Decision{Action: ManualReview, Reason: "risk failure requires human approval"}
	}
	if event.Attempt >= maxAttempts {
		return Decision{Action: DeadLetter, Reason: "retry allowance exhausted"}
	}
	return Decision{Action: Retry, Reason: "retry remains within allowance"}
}
