package invoice

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/example/fintech-invoice-service/internal/infrai"
)

type PaymentEvent struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Amount   int64  `json:"amount_cents"`
	Occurred string `json:"occurred_at"`
}

type Order struct {
	ID            string         `json:"id"`
	CustomerEmail string         `json:"customer_email"`
	Currency      string         `json:"currency"`
	Total         int64          `json:"total_cents"`
	RiskScore     int            `json:"risk_score"`
	Payments      []PaymentEvent `json:"payment_events"`
}

type Notification struct {
	OrderID string `json:"order_id"`
	Kind    string `json:"kind"`
	Reason  string `json:"reason"`
}

type Result struct {
	State        string          `json:"state"`
	Notification Notification    `json:"notification"`
	PDF          json.RawMessage `json:"pdf,omitempty"`
}

type PDFGenerator interface {
	GeneratePDF(context.Context, infrai.GenerateRequest, string) (json.RawMessage, error)
}

type Workflow struct {
	PDF       PDFGenerator
	RiskLimit int
}

func (w Workflow) Run(ctx context.Context, order Order) (Result, error) {
	if err := validate(order); err != nil {
		return Result{}, err
	}
	if order.RiskScore >= w.RiskLimit {
		return decision(order.ID, "review_required", "risk threshold reached"), nil
	}
	if netPaid(order.Payments) < order.Total {
		return decision(order.ID, "payment_pending", "settled balance is below order total"), nil
	}

	html, err := renderInvoice(order)
	if err != nil {
		return Result{}, err
	}
	pdf, err := w.PDF.GeneratePDF(ctx, infrai.GenerateRequest{
		HTML:        html,
		PageSize:    "A4",
		Orientation: "portrait",
		Store:       true,
	}, "invoice-"+order.ID)
	if err != nil {
		return Result{}, err
	}
	result := decision(order.ID, "issued", "payment and risk checks passed")
	result.PDF = pdf
	return result, nil
}

func decision(orderID, state, reason string) Result {
	return Result{
		State: state,
		Notification: Notification{
			OrderID: orderID,
			Kind:    "invoice." + state,
			Reason:  reason,
		},
	}
}

func netPaid(events []PaymentEvent) int64 {
	var total int64
	for _, event := range events {
		switch event.Kind {
		case "captured":
			total += event.Amount
		case "refunded", "chargeback":
			total -= event.Amount
		}
	}
	return total
}

func validate(order Order) error {
	if strings.TrimSpace(order.ID) == "" || strings.TrimSpace(order.CustomerEmail) == "" {
		return fmt.Errorf("id and customer_email are required")
	}
	if order.Total <= 0 || order.Currency == "" {
		return fmt.Errorf("total_cents must be positive and currency is required")
	}
	for _, event := range order.Payments {
		if event.ID == "" || event.Amount <= 0 {
			return fmt.Errorf("payment event id and positive amount_cents are required")
		}
		if _, err := time.Parse(time.RFC3339, event.Occurred); err != nil {
			return fmt.Errorf("payment event occurred_at must be RFC3339: %w", err)
		}
	}
	return nil
}

var invoiceTemplate = template.Must(template.New("invoice").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><style>body{font-family:sans-serif;margin:40px}dt{font-weight:bold}dd{margin:0 0 12px}</style></head>
<body><h1>Invoice {{.ID}}</h1><dl><dt>Customer</dt><dd>{{.CustomerEmail}}</dd><dt>Total</dt><dd>{{.Total}} {{.Currency}}</dd></dl></body></html>`))

func renderInvoice(order Order) (string, error) {
	var output strings.Builder
	if err := invoiceTemplate.Execute(&output, order); err != nil {
		return "", fmt.Errorf("render invoice: %w", err)
	}
	return output.String(), nil
}
