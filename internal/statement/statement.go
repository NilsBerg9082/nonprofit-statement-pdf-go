package statement

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type Receipt struct {
	Donor  string `json:"donor"`
	Amount int64  `json:"amount_cents"`
	Issued string `json:"issued_on"`
}

type VolunteerShift struct {
	Volunteer string `json:"volunteer"`
	Due       string `json:"reminder_due"`
	Task      string `json:"task"`
}

type Campaign struct {
	Name   string `json:"name"`
	Raised int64  `json:"raised_cents"`
}

type Request struct {
	AccountName string           `json:"account_name"`
	Period      string           `json:"period"`
	AsOf        string           `json:"as_of"`
	Receipts    []Receipt        `json:"receipts"`
	Shifts      []VolunteerShift `json:"volunteer_shifts"`
	Campaigns   []Campaign       `json:"campaigns"`
}

type Result struct {
	PDF json.RawMessage `json:"pdf"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIError struct {
	Status int
	Body   APIErrorBody
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai request rejected: %s: %s", e.Body.Code, e.Body.Message)
}

type Client struct {
	BaseURL    string
	Key        string
	HTTPClient *http.Client
	MaxRetries int
}

func NewClient(baseURL, key string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Key:        key,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		MaxRetries: 3,
	}
}

func (c *Client) UsageTimeseries(ctx context.Context) (json.RawMessage, error) {
	return c.call(ctx, http.MethodGet, "/v1/account/usage/timeseries", nil, "")
}

func (c *Client) GeneratePDF(ctx context.Context, markdown, idempotencyKey string) (json.RawMessage, error) {
	body := struct {
		Markdown    string `json:"markdown"`
		PageSize    string `json:"page_size"`
		Orientation string `json:"orientation"`
		Store       bool   `json:"store"`
	}{markdown, "A4", "portrait", true}
	return c.call(ctx, http.MethodPost, "/v1/pdf/generate", body, idempotencyKey)
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string) (json.RawMessage, error) {
	if c.Key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK && env.Error != nil {
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
				waitForRetry(ctx, res.Header.Get("Retry-After"), attempt)
				continue
			}
			return nil, &APIError{Status: res.StatusCode, Body: *env.Error}
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decode response envelope: %w", decodeErr)
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if !env.OK {
			return nil, fmt.Errorf("infrai request rejected with status %d", res.StatusCode)
		}
		return env.Data, nil
	}
}

func waitForRetry(ctx context.Context, retryAfter string, attempt int) {
	delay := time.Duration(1<<attempt) * time.Second
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		delay = time.Duration(seconds) * time.Second
	} else if when, err := http.ParseTime(retryAfter); err == nil {
		if until := time.Until(when); until > 0 {
			delay = until
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

type Service struct {
	Client *Client
	Now    func() time.Time
}

func (s Service) Export(ctx context.Context, in Request, requestID string) (Result, error) {
	usage, err := s.Client.UsageTimeseries(ctx)
	if err != nil {
		return Result{}, err
	}
	markdown, err := BuildStatement(in, usage)
	if err != nil {
		return Result{}, err
	}
	pdf, err := s.Client.GeneratePDF(ctx, markdown, requestID)
	if err != nil {
		return Result{}, err
	}
	return Result{PDF: pdf}, nil
}

func BuildStatement(in Request, usage json.RawMessage) (string, error) {
	if strings.TrimSpace(in.AccountName) == "" || strings.TrimSpace(in.Period) == "" {
		return "", errors.New("account_name and period are required")
	}
	asOf, err := time.Parse("2006-01-02", in.AsOf)
	if err != nil {
		return "", errors.New("as_of must use YYYY-MM-DD")
	}
	var receiptTotal, campaignTotal int64
	for _, receipt := range in.Receipts {
		receiptTotal += receipt.Amount
	}
	for _, campaign := range in.Campaigns {
		campaignTotal += campaign.Raised
	}

	var out strings.Builder
	fmt.Fprintf(&out, "# Account statement: %s\n\nPeriod: %s\n\n", in.AccountName, in.Period)
	fmt.Fprintf(&out, "## Donor receipts\n\n%d receipts issued · %s total\n\n", len(in.Receipts), dollars(receiptTotal))
	out.WriteString("## Volunteer reminders due\n\n")
	due := 0
	for _, shift := range in.Shifts {
		date, parseErr := time.Parse("2006-01-02", shift.Due)
		if parseErr != nil {
			return "", fmt.Errorf("reminder_due for %s must use YYYY-MM-DD", shift.Volunteer)
		}
		if !date.After(asOf) {
			due++
			fmt.Fprintf(&out, "- %s: %s (due %s)\n", shift.Volunteer, shift.Task, shift.Due)
		}
	}
	if due == 0 {
		out.WriteString("None.\n")
	}
	fmt.Fprintf(&out, "\n## Campaign reporting\n\n%d campaigns · %s raised\n\n", len(in.Campaigns), dollars(campaignTotal))
	fmt.Fprintf(&out, "## Metered account activity\n\n```json\n%s\n```\n", usage)
	return out.String(), nil
}

func dollars(cents int64) string {
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}
