package invoice

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/example/fintech-invoice-service/internal/infrai"
)

type recordingGenerator struct {
	calls int
}

func (g *recordingGenerator) GeneratePDF(_ context.Context, request infrai.GenerateRequest, key string) (json.RawMessage, error) {
	g.calls++
	if request.HTML == "" || request.PageSize != "A4" || !request.Store {
		return nil, &infrai.APIError{Detail: infrai.ErrorDetail{Message: "unexpected request"}}
	}
	return json.RawMessage(`{"job_id":"job_example"}`), nil
}

func TestWorkflowDecision(t *testing.T) {
	base := Order{
		ID: "ord_1042", CustomerEmail: "ops@example.com", Currency: "USD", Total: 12500,
		Payments: []PaymentEvent{{ID: "pay_1", Kind: "captured", Amount: 12500, Occurred: "2026-08-25T09:00:00Z"}},
	}
	tests := []struct {
		name      string
		mutate    func(*Order)
		wantState string
		wantCalls int
	}{
		{name: "settled low-risk order is issued", mutate: func(*Order) {}, wantState: "issued", wantCalls: 1},
		{name: "high-risk order waits for review", mutate: func(o *Order) { o.RiskScore = 80 }, wantState: "review_required", wantCalls: 0},
		{name: "refund makes payment incomplete", mutate: func(o *Order) {
			o.Payments = append(o.Payments, PaymentEvent{ID: "refund_1", Kind: "refunded", Amount: 500, Occurred: "2026-08-25T09:05:00Z"})
		}, wantState: "payment_pending", wantCalls: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			order := base
			order.Payments = append([]PaymentEvent(nil), base.Payments...)
			test.mutate(&order)
			generator := &recordingGenerator{}
			result, err := (Workflow{PDF: generator, RiskLimit: 70}).Run(context.Background(), order)
			if err != nil {
				t.Fatal(err)
			}
			if result.State != test.wantState {
				t.Fatalf("state = %q, want %q", result.State, test.wantState)
			}
			if result.Notification.Kind != "invoice."+test.wantState {
				t.Fatalf("notification kind = %q", result.Notification.Kind)
			}
			if generator.calls != test.wantCalls {
				t.Fatalf("PDF calls = %d, want %d", generator.calls, test.wantCalls)
			}
		})
	}
}
