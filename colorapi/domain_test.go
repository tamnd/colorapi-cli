package colorapi

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring, which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "colorapi" {
		t.Errorf("Scheme = %q, want colorapi", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "colorapi" {
		t.Errorf("Identity.Binary = %q, want colorapi", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"0047AB", "colorinfo", "0047AB"},
		{"#FF6600", "colorinfo", "FF6600"},
		{"ffffff", "colorinfo", "ffffff"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") should return an error")
	}
}

func TestLocateColorInfo(t *testing.T) {
	got, err := Domain{}.Locate("colorinfo", "0047AB")
	want := "https://" + Host + "/id?hex=0047AB"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateSchemeColor(t *testing.T) {
	got, err := Domain{}.Locate("schemecolor", "FF6600")
	want := "https://" + Host + "/scheme?hex=FF6600"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "0047AB")
	if err == nil {
		t.Error("Locate with unknown type should return an error")
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip.
// Kit derives URI type from struct name: ColorInfo → colorinfo.
// Kit uses the kit:"id" field value as the URI id (Name field for ColorInfo).
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	color := &ColorInfo{Name: "Cobalt", Hex: "#0047AB", RGB: "rgb(0, 71, 171)"}
	u, err := h.Mint(color)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	// kit derives type from struct name (colorinfo) and id from kit:"id" field (Name = "Cobalt")
	if want := "colorapi://colorinfo/Cobalt"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("colorapi", "FF6600")
	if err != nil || got.String() != "colorapi://colorinfo/FF6600" {
		t.Errorf("ResolveOn = (%q, %v), want colorapi://colorinfo/FF6600", got.String(), err)
	}
}
