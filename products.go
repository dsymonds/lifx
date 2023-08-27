package lifx

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// products.json, from https://github.com/LIFX/products
//
//go:embed products.json
var rawProductsJSON []byte

// ProductsFile represents the data from a products.json file.
//
// The data is decoded from a version of https://github.com/LIFX/products
// embedded in this package.
var ProductsFile []VendorProducts

func init() {
	if err := json.Unmarshal(rawProductsJSON, &ProductsFile); err != nil {
		panic("internal error decoding products.json: " + err.Error())
	}
}

// VendorProducts represents a vendor and all their products.
type VendorProducts struct {
	VID  uint32 `json:"vid"`  // 1 == LIFX
	Name string `json:"name"` // e.g. "LIFX"

	Defaults ProductCapabilities `json:"defaults"`
	Products []Product           `json:"products"`
}

// ProductCapabilities represents the functional capabilities of a product.
//
// The fields in this structure are nullable because the data file has a
// default layering semantic. Any Product returned through DetermineProduct is
// guaranteed to set all fields, except where otherwise specified.
type ProductCapabilities struct {
	HEV    *bool `json:"hev,omitempty"`
	Color  *bool `json:"color,omitempty"`
	Matrix *bool `json:"matrix,omitempty"`

	Multizone         *bool    `json:"multizone,omitempty"`
	TemperatureRange  []uint16 `json:"temperature_range"` // should be two values (min and max); may be nil from DetermineProduct
	ExtendedMultizone *bool    `json:"extended_multizone,omitempty"`

	// TODO: much more
}

func (pc ProductCapabilities) String() string {
	var s []string
	checkBool := func(b *bool, name string) {
		if b != nil && *b {
			s = append(s, name)
		}
	}
	checkBool(pc.HEV, "hev")
	checkBool(pc.Color, "color")
	checkBool(pc.Matrix, "matrix")
	checkBool(pc.Multizone, "multizone")
	if tr := pc.TemperatureRange; len(tr) > 0 {
		s = append(s, fmt.Sprintf("temperature_range=[%d,%d]", tr[0], tr[1]))
	}
	checkBool(pc.ExtendedMultizone, "extended_multizone")
	return "{" + strings.Join(s, ",") + "}"
}

// merge applies values set in o.
func (pc *ProductCapabilities) merge(o ProductCapabilities) {
	copyBool := func(dst **bool, src *bool) {
		if src == nil {
			return
		}
		if *dst == nil {
			*dst = boolPtr(false) // will be immediately overwritten
		}
		**dst = *src
	}

	copyBool(&pc.HEV, o.HEV)
	copyBool(&pc.Color, o.Color)
	copyBool(&pc.Matrix, o.Matrix)

	copyBool(&pc.Multizone, o.Multizone)
	if tr := o.TemperatureRange; len(tr) > 0 {
		pc.TemperatureRange = []uint16{tr[0], tr[1]}
	}
	copyBool(&pc.ExtendedMultizone, o.ExtendedMultizone)
}

// Product represents information about a product.
type Product struct {
	PID      uint32              `json:"pid"`
	Name     string              `json:"name"`
	Features ProductCapabilities `json:"features"`
	Upgrades []struct {
		Major    uint16              `json:"major"`
		Minor    uint16              `json:"minor"`
		Features ProductCapabilities `json:"features"`
	} `json:"upgrades"`
}

// DetermineProduct determines the product and its derived capabilities.
// Use this rather than manually inspecting ProductsFile, which should be
// passed as the first argument.
//
// vendorID and productID arguments can be obtained with GetVersion,
// and firmwareVersion can be obtained with GetHostFirmware.
func DetermineProduct(file []VendorProducts, vendorID, productID uint32, firmwareVersion HostFirmware) (Product, error) {
	var vp *VendorProducts
	for i := range file {
		if file[i].VID == vendorID {
			vp = &file[i]
			break
		}
	}
	if vp == nil {
		return Product{}, fmt.Errorf("unknown vendor ID %d", vendorID)
	}

	var product Product
	var found bool
	for _, p := range vp.Products {
		if p.PID == productID {
			product, found = p, true
			break
		}
	}
	if !found {
		return Product{}, fmt.Errorf("unknown product ID %d for vendor %d (%s)", productID, vendorID, vp.Name)
	}

	// Start with the default capabilities, then copy over the product capabilities.
	// Finally, apply specific version upgrades.
	cap := ProductCapabilities{
		HEV:    boolPtr(false),
		Color:  boolPtr(false),
		Matrix: boolPtr(false),

		Multizone: boolPtr(false),
		// no TemperatureRange default
		ExtendedMultizone: boolPtr(false),
	}
	cap.merge(vp.Defaults)
	cap.merge(product.Features)
	for _, u := range product.Upgrades {
		// This logic seems wrong (majorX > majorY should ignore minorX and minorY),
		// but this is what is documented.
		if firmwareVersion.Major >= u.Major && firmwareVersion.Minor >= u.Minor {
			cap.merge(u.Features)
		}
	}
	product.Features = cap

	return product, nil
}

func boolPtr(b bool) *bool { return &b }
