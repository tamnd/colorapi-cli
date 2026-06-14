// Package colorapi is the library behind the colorapi command line:
// the HTTP client, request shaping, and the typed data models for
// The Color API (thecolorapi.com) — color identification and scheme generation.
package colorapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Host is the site this client talks to.
const Host = "www.thecolorapi.com"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://www.thecolorapi.com",
		UserAgent: "colorapi-cli/0.1 (tamnd87@gmail.com)",
		Rate:      200 * time.Millisecond,
		Timeout:   10 * time.Second,
		Retries:   3,
	}
}

// Client talks to thecolorapi.com over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// ColorInfo holds the identified properties of a color.
type ColorInfo struct {
	Name  string `kit:"id" json:"name"`
	Hex   string `json:"hex"`
	RGB   string `json:"rgb"`
	HSL   string `json:"hsl"`
	HSV   string `json:"hsv"`
	CMYK  string `json:"cmyk"`
	Exact bool   `json:"exact_match"`
}

// SchemeColor is one entry in a generated color scheme.
type SchemeColor struct {
	Hex  string `kit:"id" json:"hex"`
	Name string `json:"name"`
	RGB  string `json:"rgb"`
	HSL  string `json:"hsl"`
}

// --- raw API response shapes ---

type apiColor struct {
	Hex  struct{ Value string `json:"value"` } `json:"hex"`
	RGB  struct{ Value string `json:"value"` } `json:"rgb"`
	HSL  struct{ Value string `json:"value"` } `json:"hsl"`
	HSV  struct{ Value string `json:"value"` } `json:"hsv"`
	CMYK struct{ Value string `json:"value"` } `json:"cmyk"`
	Name struct {
		Value          string `json:"value"`
		ExactMatchName bool   `json:"exact_match_name"`
	} `json:"name"`
}

type schemeResponse struct {
	Colors []apiColor `json:"colors"`
}

// Identify resolves a color by one of hex, rgb, or hsl query parameters.
// Exactly one of hex/rgb/hsl must be non-empty.
func (c *Client) Identify(ctx context.Context, hex, rgb, hsl string) (*ColorInfo, error) {
	var param string
	switch {
	case hex != "":
		hex = strings.TrimPrefix(hex, "#")
		param = "hex=" + hex
	case rgb != "":
		param = "rgb=" + rgb
	case hsl != "":
		param = "hsl=" + hsl
	default:
		return nil, fmt.Errorf("one of --hex, --rgb, or --hsl is required")
	}
	u := c.cfg.BaseURL + "/id?" + param
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var ac apiColor
	if err := json.Unmarshal(body, &ac); err != nil {
		return nil, fmt.Errorf("decode color: %w", err)
	}
	return &ColorInfo{
		Name:  ac.Name.Value,
		Hex:   ac.Hex.Value,
		RGB:   ac.RGB.Value,
		HSL:   ac.HSL.Value,
		HSV:   ac.HSV.Value,
		CMYK:  ac.CMYK.Value,
		Exact: ac.Name.ExactMatchName,
	}, nil
}

// Scheme generates a color scheme from a base hex color.
// mode is one of: monochrome, monochrome-dark, monochrome-light,
// analogic, complement, analogic-complement, triad, quad.
// count is the number of colors to return.
func (c *Client) Scheme(ctx context.Context, hex, mode string, count int) ([]SchemeColor, error) {
	hex = strings.TrimPrefix(hex, "#")
	if mode == "" {
		mode = "complement"
	}
	if count <= 0 {
		count = 5
	}
	u := fmt.Sprintf("%s/scheme?hex=%s&mode=%s&count=%d", c.cfg.BaseURL, hex, mode, count)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var sr schemeResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("decode scheme: %w", err)
	}
	out := make([]SchemeColor, 0, len(sr.Colors))
	for _, ac := range sr.Colors {
		out = append(out, SchemeColor{
			Hex:  ac.Hex.Value,
			Name: ac.Name.Value,
			RGB:  ac.RGB.Value,
			HSL:  ac.HSL.Value,
		})
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	return b, err != nil, err
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}
