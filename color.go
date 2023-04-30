package lifx

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

const (
	encodedColorLength = 2 + 2 + 2 + 2 // four uint16s
)

// Color represents a single HSBK value.
//
// https://lan.developer.lifx.com/docs/field-types#color
type Color struct {
	Hue, Saturation, Brightness uint16
	Kelvin                      uint16
}

// encode writes the color into the given destination slice.
// The caller must ensure len(dst) is at least encodedColorLength.
func (c *Color) encode(dst []byte) {
	binary.LittleEndian.PutUint16(dst[0:2], c.Hue)
	binary.LittleEndian.PutUint16(dst[2:4], c.Saturation)
	binary.LittleEndian.PutUint16(dst[4:6], c.Brightness)
	binary.LittleEndian.PutUint16(dst[6:8], c.Kelvin)
}

func (c *Color) decode(b []byte) {
	c.Hue = binary.LittleEndian.Uint16(b[0:2])
	c.Saturation = binary.LittleEndian.Uint16(b[2:4])
	c.Brightness = binary.LittleEndian.Uint16(b[4:6])
	c.Kelvin = binary.LittleEndian.Uint16(b[6:8])
}

func (d *Device) GetExtendedColorZones(ctx context.Context) (zones []Color, err error) {
	payload, err := d.query(ctx, pktGetExtendedColorZones, pktStateExtendedColorZones, nil)
	if err != nil {
		return nil, err
	}
	if len(payload) < 5 {
		return nil, fmt.Errorf("StateExtendedColorZones too short: length=%d", len(payload))
	}
	zonesCount := int(binary.LittleEndian.Uint16(payload[0:2])) // "The number of zones on your strip"
	zoneIndex := int(binary.LittleEndian.Uint16(payload[2:4]))  // "The first zone represented in the packet"
	colorsCount := int(payload[4])                              // "The number of HSBK values in the colors array that map to zones."

	colors := payload[5:]
	if want := colorsCount * encodedColorLength; want > len(colors) {
		return nil, fmt.Errorf("StateExtendedColorZones too short: colorsCount=%d length=%d", colorsCount, len(payload))
	} else if want < len(colors) {
		colors = colors[:want]
	}

	// TODO: We don't handle the case where the entire strip's color state is returned
	// in a single message. What happens? Will we get multiple StateExtendedColorZones messages?
	// The documentation is unclear on this point. Let's proceed under the assumption that
	// the zones are all given.
	if zonesCount != colorsCount || zoneIndex != 0 {
		return nil, fmt.Errorf("can't handle partial/complex StateExtendedColorZones message")
	}

	zones = make([]Color, colorsCount)
	for i := 0; i < colorsCount; i++ {
		off := i * encodedColorLength
		zones[i].decode(colors[off : off+encodedColorLength])
	}

	return
}

func (d *Device) SetExtendedColorZones(ctx context.Context, duration time.Duration, zones []Color) error {
	if len(zones) > 82 {
		return fmt.Errorf("too many zones to set; %d > 82", len(zones))
	}
	dur, err := uint32Millis(duration)
	if err != nil {
		return err
	}

	payload := make([]byte, 4+1+2+1+len(zones)*encodedColorLength)
	binary.LittleEndian.PutUint32(payload[0:4], dur) // duration
	payload[4] = 1                                   // apply; MultiZoneExtendedApplicationRequest(APPLY)
	binary.LittleEndian.PutUint16(payload[5:7], 0)   // zone_index
	payload[7] = uint8(len(zones))
	for i, off := 0, 8; i < len(zones); i++ {
		// The next line doesn't strictly need the second slice arg, but it is a useful sanity check.
		zones[i].encode(payload[off : off+encodedColorLength])
		off += encodedColorLength
	}

	return d.set(ctx, pktSetExtendedColorZones, payload)
}

type Waveform int

const (
	SawWaveform      = Waveform(0)
	SineWaveform     = Waveform(1)
	HalfSineWaveform = Waveform(2)
	TriangleWaveform = Waveform(3)
	PulseWaveform    = Waveform(4)
)

type WaveformConfig struct {
	Waveform  Waveform
	Transient bool

	Color Color

	Period time.Duration
	Cycles float32

	// TODO: skew_ratio, if needed. Also, optionality (and use SetWaveformOptional).
}

func (d *Device) SetWaveform(ctx context.Context, cfg WaveformConfig) error {
	period, err := uint32Millis(cfg.Period)
	if err != nil {
		return err
	}

	payload := make([]byte, 21)
	payload[1] = boolInt(cfg.Transient)                                         // transient
	cfg.Color.encode(payload[2:10])                                             // hue, saturation, brightness, kelvin
	binary.LittleEndian.PutUint32(payload[10:14], period)                       // period
	binary.LittleEndian.PutUint32(payload[14:18], math.Float32bits(cfg.Cycles)) // cycles; this encoding is a guess
	// skew_ratio left at 0 (only used for Pulse), which encodes 0.5.
	payload[20] = byte(cfg.Waveform)

	return d.set(ctx, pktSetWaveform, payload)
}
