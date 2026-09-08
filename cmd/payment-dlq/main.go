package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/example/fintech-dead-letter-worker/infrai"
	"github.com/example/fintech-dead-letter-worker/payments"
)

type service struct {
	queue *infrai.Client
	now   func() time.Time
}

func main() {
	client, err := infrai.New(os.Getenv("INFRAI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	svc := &service{queue: client, now: time.Now}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /payments", svc.submitPayment)
	mux.HandleFunc("POST /worker/run", svc.runWorker)

	addr := ":8080"
	if value := os.Getenv("ADDR"); value != "" {
		addr = value
	}
	log.Printf("payment dead-letter service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *service) submitPayment(w http.ResponseWriter, r *http.Request) {
	var event payments.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil || event.PaymentID == "" || event.AmountCents <= 0 || event.Currency == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_id, positive amount_cents, and currency are required"})
		return
	}
	if event.Attempt == 0 {
		event.Attempt = 1
	}
	if err := s.queue.Publish(r.Context(), event, "payment-"+event.PaymentID+"-"+strconv.Itoa(event.Attempt)); err != nil {
		writeQueueError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"payment_id": event.PaymentID, "status": "queued"})
}

func (s *service) runWorker(w http.ResponseWriter, r *http.Request) {
	messages, err := s.queue.Consume(r.Context(), 10, 30)
	if err != nil {
		writeQueueError(w, err)
		return
	}

	results := make([]payments.Notification, 0, len(messages))
	for _, message := range messages {
		var event payments.Event
		if err := json.Unmarshal(message.Payload, &event); err != nil {
			log.Printf("audit message_id=%s action=dead_letter reason=invalid_payment_payload", message.MessageID)
			if err := s.queue.Ack(r.Context(), message.MessageID); err != nil {
				writeQueueError(w, err)
				return
			}
			continue
		}

		decision := payments.Decide(event, 5)
		note := payments.Notification{
			PaymentID: event.PaymentID, MessageID: message.MessageID,
			Action: decision.Action, Reason: decision.Reason,
			AmountCents: event.AmountCents, Currency: event.Currency,
			RecordedAt: s.now().UTC(),
		}
		encoded, _ := json.Marshal(note)
		log.Printf("audit %s", encoded)
		results = append(results, note)

		if decision.Action != payments.Retry {
			if err := s.queue.Ack(r.Context(), message.MessageID); err != nil {
				writeQueueError(w, err)
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"processed": len(messages), "notifications": results})
}

func writeQueueError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		writeJSON(w, apiErr.Status, map[string]any{"error": apiErr.Detail})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "queue request could not be completed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Printf("encode response: %v\n", err)
	}
}
