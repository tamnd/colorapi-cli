package colorapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(srv *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	return NewClient(cfg)
}

// colorPayload returns a minimal Color API /id JSON response.
func colorPayload(hex, name string, exact bool) string {
	return `{
		"hex":  {"value": "` + hex + `", "clean": "` + strings.TrimPrefix(hex, "#") + `"},
		"rgb":  {"value": "rgb(0, 71, 171)", "r": 0, "g": 71, "b": 171},
		"hsl":  {"value": "hsl(215, 100%, 34%)", "h": 215, "s": 100, "l": 34},
		"hsv":  {"value": "hsv(215, 100%, 67%)", "h": 215, "s": 100, "v": 67},
		"cmyk": {"value": "cmyk(100, 58, 0, 33)", "c": 100, "m": 58, "y": 0, "k": 33},
		"name": {"value": "` + name + `", "closest_named_hex": "` + hex + `", "exact_match_name": ` + boolStr(exact) + `, "distance": 0}
	}`
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestIdentifyByHex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/id") {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("hex"); got != "0047AB" {
			t.Errorf("hex param = %q, want 0047AB", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(colorPayload("#0047AB", "Cobalt", true)))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	info, err := c.Identify(context.Background(), "0047AB", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Cobalt" {
		t.Errorf("Name = %q, want Cobalt", info.Name)
	}
	if info.Hex != "#0047AB" {
		t.Errorf("Hex = %q, want #0047AB", info.Hex)
	}
	if !info.Exact {
		t.Error("Exact = false, want true")
	}
}

func TestIdentifyStripLeadingHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// should have stripped the # so param is 0047AB not #0047AB
		if got := r.URL.Query().Get("hex"); got != "0047AB" {
			t.Errorf("hex param = %q, want 0047AB (# stripped)", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(colorPayload("#0047AB", "Cobalt", true)))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Identify(context.Background(), "#0047AB", "", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestIdentifyByRGB(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("rgb"); got != "255,0,128" {
			t.Errorf("rgb param = %q, want 255,0,128", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(colorPayload("#FF0080", "Rose", false)))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	info, err := c.Identify(context.Background(), "", "255,0,128", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Rose" {
		t.Errorf("Name = %q, want Rose", info.Name)
	}
}

func TestIdentifyByHSL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("hsl"); got != "215,100,34" {
			t.Errorf("hsl param = %q, want 215,100,34", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(colorPayload("#0047AB", "Cobalt", true)))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	info, err := c.Identify(context.Background(), "", "", "215,100,34")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Cobalt" {
		t.Errorf("Name = %q, want Cobalt", info.Name)
	}
}

func TestIdentifyNoInput(t *testing.T) {
	c := NewClient(DefaultConfig())
	c.cfg.Rate = 0
	_, err := c.Identify(context.Background(), "", "", "")
	if err == nil {
		t.Error("expected error when no color input given")
	}
}

func TestScheme(t *testing.T) {
	resp := map[string]any{
		"mode":  "triad",
		"count": 3,
		"colors": []map[string]any{
			{"hex": map[string]any{"value": "#FF6600"}, "name": map[string]any{"value": "Orange", "exact_match_name": true}, "rgb": map[string]any{"value": "rgb(255, 102, 0)"}, "hsl": map[string]any{"value": "hsl(24, 100%, 50%)"}, "hsv": map[string]any{"value": "hsv(24, 100%, 100%)"}, "cmyk": map[string]any{"value": "cmyk(0, 60, 100, 0)"}},
			{"hex": map[string]any{"value": "#0066FF"}, "name": map[string]any{"value": "Blue Ribbon", "exact_match_name": false}, "rgb": map[string]any{"value": "rgb(0, 102, 255)"}, "hsl": map[string]any{"value": "hsl(216, 100%, 50%)"}, "hsv": map[string]any{"value": "hsv(216, 100%, 100%)"}, "cmyk": map[string]any{"value": "cmyk(100, 60, 0, 0)"}},
			{"hex": map[string]any{"value": "#66FF00"}, "name": map[string]any{"value": "Bright Green", "exact_match_name": false}, "rgb": map[string]any{"value": "rgb(102, 255, 0)"}, "hsl": map[string]any{"value": "hsl(96, 100%, 50%)"}, "hsv": map[string]any{"value": "hsv(96, 100%, 100%)"}, "cmyk": map[string]any{"value": "cmyk(60, 0, 100, 0)"}},
		},
	}
	payload, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("hex") != "FF6600" {
			t.Errorf("scheme hex = %q, want FF6600", q.Get("hex"))
		}
		if q.Get("mode") != "triad" {
			t.Errorf("scheme mode = %q, want triad", q.Get("mode"))
		}
		if q.Get("count") != "3" {
			t.Errorf("scheme count = %q, want 3", q.Get("count"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	colors, err := c.Scheme(context.Background(), "FF6600", "triad", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(colors) != 3 {
		t.Fatalf("got %d colors, want 3", len(colors))
	}
	if colors[0].Hex != "#FF6600" {
		t.Errorf("colors[0].Hex = %q, want #FF6600", colors[0].Hex)
	}
	if colors[0].Name != "Orange" {
		t.Errorf("colors[0].Name = %q, want Orange", colors[0].Name)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(colorPayload("#0047AB", "Cobalt", true)))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := NewClient(cfg)

	_, err := c.Identify(context.Background(), "0047AB", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}
