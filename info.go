package lifx

import (
	"context"
	"encoding/binary"
	"fmt"
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
