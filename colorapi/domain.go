package colorapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes colorapi as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/colorapi-cli/colorapi"
//
// The same Domain also builds the standalone colorapi binary (see cli.NewApp).
func init() { kit.Register(Domain{}) }

// Domain is the colorapi driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "colorapi",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "colorapi",
			Short:  "Identify colors and generate color schemes from The Color API",
			Long: `colorapi fetches color data from thecolorapi.com — identify a color in any
color space (hex, RGB, HSL) and get its name, hex, RGB, HSL, HSV, and CMYK
representations, or generate harmonious color schemes. No API key required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/colorapi-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// id: identify a color by hex, rgb, or hsl
	kit.Handle(app, kit.OpMeta{
		Name:    "id",
		Group:   "read",
		Single:  true,
		Summary: "Identify a color and get its properties",
	}, identifyOp)

	// scheme: generate a color scheme from a base hex color
	kit.Handle(app, kit.OpMeta{
		Name:    "scheme",
		Group:   "read",
		List:    true,
		Summary: "Generate a color scheme from a base hex color",
		Args:    []kit.Arg{{Name: "hex", Help: "base color hex code (e.g. 0047AB or #0047AB)"}},
	}, schemeOp)
}

// newClient builds the client from host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type identifyInput struct {
	Hex    string  `kit:"flag" help:"hex color code (e.g. 0047AB or #0047AB)"`
	Rgb    string  `kit:"flag" help:"RGB color as r,g,b (e.g. 0,71,171)"`
	Hsl    string  `kit:"flag" help:"HSL color as h,s,l (e.g. 215,100,34)"`
	Client *Client `kit:"inject"`
}

type schemeInput struct {
	Hex    string  `kit:"arg" help:"base color hex code (e.g. 0047AB or #0047AB)"`
	Mode   string  `kit:"flag" default:"complement" help:"scheme mode: monochrome, monochrome-dark, monochrome-light, analogic, complement, analogic-complement, triad, quad"`
	Count  int     `kit:"flag" default:"5" help:"number of colors to return"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func identifyOp(ctx context.Context, in identifyInput, emit func(*ColorInfo) error) error {
	if in.Hex == "" && in.Rgb == "" && in.Hsl == "" {
		return errs.Usage("one of --hex, --rgb, or --hsl is required")
	}
	info, err := in.Client.Identify(ctx, in.Hex, in.Rgb, in.Hsl)
	if err != nil {
		return mapErr(err)
	}
	return emit(info)
}

func schemeOp(ctx context.Context, in schemeInput, emit func(*SchemeColor) error) error {
	if in.Hex == "" {
		return errs.Usage("hex argument is required")
	}
	mode := in.Mode
	if mode == "" {
		mode = "complement"
	}
	count := in.Count
	if count <= 0 {
		count = 5
	}
	colors, err := in.Client.Scheme(ctx, in.Hex, mode, count)
	if err != nil {
		return mapErr(err)
	}
	for i := range colors {
		if err := emit(&colors[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns an input into the canonical (type, id).
// Kit derives URI types from struct names: ColorInfo → colorinfo, SchemeColor → schemecolor.
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty colorapi reference")
	}
	// strip leading # for hex colors
	hex := strings.TrimPrefix(input, "#")
	return "colorinfo", hex, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "colorinfo":
		return fmt.Sprintf("https://%s/id?hex=%s", Host, id), nil
	case "schemecolor":
		return fmt.Sprintf("https://%s/scheme?hex=%s", Host, id), nil
	default:
		return "", errs.Usage("colorapi has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	return err
}
