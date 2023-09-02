package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/dsymonds/lifx"
)

var (
	playLabel = flag.String("play", "TV", "`label` of a device to exercise")
)

func main() {
	flag.Parse()
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
		hf, err := dev.GetHostFirmware(ctx)
		if err == nil {
			log.Printf("  firmware (%d,%d) built %v", hf.Major, hf.Minor, hf.Build)
		} else {
			log.Printf("  [%v]", err)
		}
		prod, err := lifx.DetermineProduct(lifx.ProductsFile, vendor, product, hf)
		if err == nil {
			log.Printf("  product is %q", prod.Name)
			log.Printf("  features: %s", prod.Features)
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
		col, err := dev.GetColor(ctx)
		if err == nil {
			log.Printf("  color: %+v", col)
		} else {
			log.Printf("  [%v]", err)
		}
		label, err := dev.GetLabel(ctx)
		if err == nil {
			log.Printf("  label: %q", label)
		} else {
			log.Printf("  [%v]", err)
		}

		if label == *playLabel {
			playDev = dev
		}
	}

	if playDev == nil {
		log.Printf("No device with label %q; I'm done.", *playLabel)
		return
	}
	playDev.Tracef = func(ctx context.Context, format string, args ...interface{}) {
		log.Printf("--> "+format, args...)
	}

	// Capture current state.
	state, err := playDev.CaptureState(ctx)
	if err != nil {
		log.Fatalf("CaptureState: %v", err)
	}

	// Set a solid green over a short period.
	const greenTime = 3 * time.Second
	log.Printf("Going green...")
	if err := playDev.QuietOn(ctx); err != nil { // put in an on-but-no-light state
		log.Printf("QuietOn: %v", err)
	}
	if err := playDev.SetColor(ctx, lifx.Color{Hue: 0x5555, Saturation: 0xFFFF, Brightness: 0xBBBB}, greenTime); err != nil {
		log.Printf("SetColor: %v", err)
	}
	time.Sleep(greenTime)

	// Do something interesting.
	const playTime = 10 * time.Second
	log.Printf("Setting red & blue...")
	zones := make([]lifx.Color, state.NumZones())
	for i := range zones {
		if i&1 == 0 {
			// Red
			zones[i] = lifx.Color{Hue: 0, Saturation: 0xFFFF, Brightness: 0xBBBB}
		} else {
			// Blue
			zones[i] = lifx.Color{Hue: 0xAAAA, Saturation: 0xFFFF, Brightness: 0xFFFF}
		}
	}
	err = playDev.SetExtendedColorZones(ctx, playTime/2, zones)
	if err != nil {
		log.Printf("SetExtendedColorZones: %v", err)
	}
	log.Printf("Transition runnings for %v...", playTime/2)
	time.Sleep(playTime / 2)

	// Gently flash.
	log.Printf("Waving...")
	const cycles = 5
	err = playDev.SetWaveform(ctx, lifx.WaveformConfig{
		Waveform:  lifx.SineWaveform,
		Transient: true, // the default for Sine anyway

		Color: lifx.Color{
			Hue:        0xD709,
			Saturation: 0xFFFF,
			Brightness: 0xFFFF,
		},

		Period: (playTime / 2) / cycles,
		Cycles: cycles,
	})
	if err != nil {
		log.Fatalf("SetWaveform: %v", err)
	}
	time.Sleep(playTime / 2)

	log.Printf("Restoring state...")
	if err := playDev.RestoreState(ctx, state); err != nil {
		log.Fatalf("RestoreState: %v", err)
	}
}
