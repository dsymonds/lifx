package main

import (
	"context"
	"log"
	"time"

	"github.com/dsymonds/lifx"
)

const (
	// Label of a device to exercise.
	playLabel = "TV"
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

	var playDev *lifx.Device
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

		if label == playLabel {
			playDev = dev
		}
	}

	if playDev == nil {
		log.Printf("No device with label %q; I'm done.", playLabel)
		return
	}

	// Capture current state.
	state, err := playDev.CaptureState(ctx)
	if err != nil {
		log.Fatalf("CaptureState: %v", err)
	}

	// Do something interesting.
	const playTime = 10 * time.Second
	zones := make([]lifx.Color, state.NumZones())
	for i := range zones {
		if i&1 == 0 {
			// Red
			zones[i] = lifx.Color{Hue: 0, Saturation: 0xFFFF, Brightness: 0xBBBB}
		} else {
			// Green
			zones[i] = lifx.Color{Hue: 0xAAAA, Saturation: 0xFFFF, Brightness: 0xFFFF}
		}
	}
	err = playDev.SetExtendedColorZones(ctx, playTime/2, zones)
	if err != nil {
		log.Printf("SetExtendedColorZones: %v", err)
	}
	log.Printf("Transition runnings for %v...", playTime/2)
	time.Sleep(playTime / 2)
	// TODO: exercise waveforms too?

	// Wait, then restore the original state.
	log.Printf("Sleeping for %v...", playTime/2)
	time.Sleep(playTime / 2)

	log.Printf("Restoring state...")
	if err := playDev.RestoreState(ctx, state); err != nil {
		log.Fatalf("RestoreState: %v", err)
	}
}
