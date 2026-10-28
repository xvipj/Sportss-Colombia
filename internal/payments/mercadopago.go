package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.mercadopago.com"

// Client es un cliente mínimo de la API de Mercado Pago (Checkout Pro).
type Client struct {
	AccessToken   string
	PublicKey     string
	WebhookSecret string
	Sandbox       bool
	HTTP          *http.Client
}

type PreferenceItem struct {
	Title      string `json:"title"`
	Quantity   int    `json:"quantity"`
	CurrencyID string `json:"currency_id"`
	UnitPrice  int64  `json:"unit_price"`
}

type BackURLs struct {
	Success string `json:"success"`
	Failure string `json:"failure"`
	Pending string `json:"pending"`
}

type PreferenceRequest struct {
	Items               []PreferenceItem `json:"items"`
	ExternalReference   string           `json:"external_reference"`
	NotificationURL     string           `json:"notification_url"`
	BackURLs            BackURLs         `json:"back_urls"`
	AutoReturn          string           `json:"auto_return,omitempty"`
	StatementDescriptor string           `json:"statement_descriptor,omitempty"`
	Metadata            map[string]any   `json:"metadata,omitempty"`
}

type PreferenceResponse struct {
	ID               string `json:"id"`
	InitPoint        string `json:"init_point"`
	SandboxInitPoint string `json:"sandbox_init_point"`
}

type PaymentInfo struct {
	ID                int64   `json:"id"`
	Status            string  `json:"status"`
	StatusDetail      string  `json:"status_detail"`
	ExternalReference string  `json:"external_reference"`
	PaymentMethodID   string  `json:"payment_method_id"`
	TransactionAmount float64 `json:"transaction_amount"`
	DateApproved      string  `json:"date_approved"`
}

func NewClient(accessToken, publicKey, webhookSecret string, sandbox bool) *Client {
	return &Client{
		AccessToken: accessToken, PublicKey: publicKey, WebhookSecret: webhookSecret, Sandbox: sandbox,
		HTTP: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) CreatePreference(ctx context.Context, req PreferenceRequest) (*PreferenceResponse, error) {
	var out PreferenceResponse
	if err := c.doJSON(ctx, http.MethodPost, "/checkout/preferences", req, &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, fmt.Errorf("Mercado Pago no devolvió el ID de la preferencia")
	}
	return &out, nil
}

func (c *Client) GetPayment(ctx context.Context, id string) (*PaymentInfo, error) {
	var out PaymentInfo
	if err := c.doJSON(ctx, http.MethodGet, "/v1/payments/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ValidateWebhook comprueba la firma x-signature de Mercado Pago.
// Si no hay secreto configurado se acepta (solo recomendable en desarrollo).
func (c *Client) ValidateWebhook(dataID, requestID, signature string) bool {
	if strings.TrimSpace(c.WebhookSecret) == "" {
		return true
	}
	parts := strings.Split(signature, ",")
	ts, v1 := "", ""
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "ts":
			ts = kv[1]
		case "v1":
			v1 = kv[1]
		}
	}
	if ts == "" || v1 == "" {
		return false
	}
	manifest := fmt.Sprintf("id:%s;request-id:%s;ts:%s;", dataID, requestID, ts)
	mac := hmac.New(sha256.New, []byte(c.WebhookSecret))
	mac.Write([]byte(manifest))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(v1))
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if id := reqID(); id != "" {
		req.Header.Set("X-Idempotency-Key", id)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("conectar con Mercado Pago: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Mercado Pago respondió %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("respuesta inválida de Mercado Pago: %w", err)
		}
	}
	return nil
}

func reqID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
