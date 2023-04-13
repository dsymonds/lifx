package main

import (
	"context"
	"log"
	"time"

	"github.com/dsymonds/lifx"
)

func main() {
	ctx := context.Background()

	const wait = 2 * time.Second
	log.Printf("Discovering LIFX devices for %v...", wait)
	discCtx, cancel := context.WithTimeout(ctx, wait)
	devs, err := lifx.Discover(discCtx)
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
	}
}
