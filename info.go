package lifx

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

func (d *Device) GetLightPower(ctx context.Context) (uint16, error) {
	payload, err := d.query(ctx, pktGetLightPower, pkgStateLightPower, nil)
	if err != nil {
		return 0, err
	}
	if len(payload) != 2 {
		return 0, fmt.Errorf("StateLightPower malformed: length=%d", len(payload))
	}
	return binary.LittleEndian.Uint16(payload), nil
}

func (d *Device) SetLightPower(ctx context.Context, level uint16, duration time.Duration) error {
	dur := duration.Milliseconds()
	if dur < 0 || dur > math.MaxUint32 {
		return fmt.Errorf("duration %v out of range", duration)
	}

	var payload []byte
	payload = binary.LittleEndian.AppendUint16(payload, level)
	binary.LittleEndian.AppendUint32(payload, uint32(dur))

	return d.set(ctx, pktSetLightPower, payload)
}

func (d *Device) GetPower(ctx context.Context) (uint16, error) {
	payload, err := d.query(ctx, pktGetPower, pktStatePower, nil)
	if err != nil {
		return 0, err
	}
	if len(payload) != 2 {
		return 0, fmt.Errorf("StatePower malformed: length=%d", len(payload))
	}
	return binary.LittleEndian.Uint16(payload), nil
}

func (d *Device) GetLabel(ctx context.Context) (string, error) {
	payload, err := d.query(ctx, pktGetLabel, pktStateLabel, nil)
	if err != nil {
		return "", err
	}

	// Ignore trailing NULs in payload.
	for i := len(payload) - 1; i >= 0; i-- {
		if payload[i] != 0x00 {
			break
		}
		payload = payload[:i]
	}

	return string(payload), nil
}

func (d *Device) GetVersion(ctx context.Context) (vendor, product uint32, err error) {
	payload, err := d.query(ctx, pktGetVersion, pktStateVersion, nil)
	if err != nil {
		return 0, 0, err
	}
	if len(payload) != 12 {
		return 0, 0, fmt.Errorf("StateVersion malformed: length=%d", len(payload))
	}
	vendor = binary.LittleEndian.Uint32(payload[0:4])
	product = binary.LittleEndian.Uint32(payload[4:8])
	return
}

type State struct {
	power uint16
	zones []Color

	// TODO: will need to capture effects too.
}

func (s State) NumZones() int { return len(s.zones) }

// CaptureState queries the device and returns its current configuration.
func (d *Device) CaptureState(ctx context.Context) (state State, err error) {
	state.power, err = d.GetLightPower(ctx)
	if err != nil {
		err = fmt.Errorf("GetLightPower: %w", err)
		return
	}
	state.zones, err = d.GetExtendedColorZones(ctx)
	if err != nil {
		err = fmt.Errorf("GetExtendedColorZones: %w", err)
		return
	}
	return
}

// RestoreState restores a device to its configuration at the time CaptureState was invoked.
func (d *Device) RestoreState(ctx context.Context, state State) error {
	err := d.SetExtendedColorZones(ctx, 0, state.zones)
	if err != nil {
		return fmt.Errorf("SetExtendedColorZones: %w", err)
	}
	err = d.SetLightPower(ctx, state.power, 0)
	if err != nil {
		return fmt.Errorf("SetLightPower: %w", err)
	}
	return nil
}
