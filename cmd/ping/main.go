package main

import (
	"context"
	"log"
	"time"

	"github.com/dsymonds/lifx"
)

func main() {
	ctx := context.Background()

	client, err := lifx.NewClient()
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	const wait = 2 * time.Second
	log.Printf("Discovering LIFX devices for %v...", wait)
	discCtx, cancel := context.WithTimeout(ctx, wait)
	devs, err := client.Discover(discCtx)
	if err != nil {
		log.Fatalf("Discover: %v", err)
	}
	cancel()

	for _, dev := range devs {
		log.Printf("* %v (serial %x)", dev.Addr.String(), dev.Serial)
		vendor, product, err := dev.GetVersion(ctx)
		if err == nil {
			log.Printf("  vendor=%d product=%d", vendor, product)
		} else {
			log.Printf("  [%v]", err)
		}
		power, err := dev.GetPower(ctx)
		if err == nil {
			log.Printf("  power: %.1f%%", float64(power)/65535*100)
		} else {
			log.Printf("  [%v]", err)
		}
		lpower, err := dev.GetLightPower(ctx)
		if err == nil {
			log.Printf("  light power: %.1f%%", float64(lpower)/65535*100)
		} else {
			log.Printf("  [%v]", err)
		}
		label, err := dev.GetLabel(ctx)
		if err == nil {
			log.Printf("  label: %q", label)
		} else {
			log.Printf("  [%v]", err)
		}

		zones, err := dev.GetExtendedColorZones(ctx)
		if err == nil {
			z0 := zones[0]
			log.Printf("  %d zones of color; first is hsb(%d,%d,%d) k=%d",
				len(zones), z0.Hue, z0.Saturation, z0.Brightness, z0.Kelvin)
		} else {
			log.Printf("  [%v]", err)
		}

		if label == "TV" && len(zones) > 0 {
			for i := range zones {
				if i&1 == 0 {
					// Red
					zones[i] = lifx.Color{Hue: 0, Saturation: 0xFFFF, Brightness: 0xFFFF}
				} else {
					// Blue
					zones[i] = lifx.Color{Hue: 0xAAAA, Saturation: 0xFFFF, Brightness: 0xFFFF}
				}
			}
			err := dev.SetExtendedColorZones(ctx, 2*time.Second, zones)
			if err != nil {
				log.Printf("  SetExtendedColorZones: %v", err)
			}
		}
	}
}
