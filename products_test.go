package lifx

import (
	"encoding/json"
	"reflect"
	"testing"
)

func mustJSON(t *testing.T, x interface{}) string {
	b, err := json.Marshal(x)
	if err != nil {
		t.Fatalf("internal error: json.Marshal: %v", err)
	}
	return string(b)
}

func TestProducts(t *testing.T) {
	// Basic test. This will exercise the init-time products.json parsing
	// as well as DetermineProduct.
	const vid, pid = 1, 32 // LIFX Z

	// (2, 78) picks up extended multizone, but not the expanded temperature range.
	p, err := DetermineProduct(ProductsFile, vid, pid, HostFirmware{Major: 2, Minor: 78})
	if err != nil {
		t.Fatalf("DetermineProduct: %v", err)
	}
	p.Upgrades = nil // should have been applied to p.Features
	want := Product{
		PID:  pid,
		Name: "LIFX Z",
		Features: ProductCapabilities{
			// DetermineProduct should set omitted entries to explicit false values.
			HEV:    boolPtr(false),
			Color:  boolPtr(true),
			Matrix: boolPtr(false),

			Multizone:         boolPtr(true),
			TemperatureRange:  []uint16{2500, 9000},
			ExtendedMultizone: boolPtr(true),
		},
	}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("DetermineProduct did not yield the right result.\n got %s\nwant %s",
			mustJSON(t, p), mustJSON(t, want))
	}

	// Check that we get the different temperature range with a higher minor version.
	p, err = DetermineProduct(ProductsFile, vid, pid, HostFirmware{Major: 2, Minor: 80})
	if err != nil {
		t.Fatalf("DetermineProduct: %v", err)
	}
	if got, want := p.Features.TemperatureRange, []uint16{1500, 9000}; !reflect.DeepEqual(got, want) {
		t.Errorf("DetermineProduct on a higher firmware version gave wrong result for temperature_range.\n got %d, want %d", got, want)
	}
}
