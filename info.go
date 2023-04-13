package lifx

import (
	"context"
	"encoding/binary"
	"fmt"
)

func (d *Device) GetPower(ctx context.Context) (uint16, error) {
	payload, err := d.query(ctx, 20, 22, nil)
	if err != nil {
		return 0, err
	}
	if len(payload) != 2 {
		return 0, fmt.Errorf("StatePower malformed: length=%d", len(payload))
	}
	return binary.LittleEndian.Uint16(payload), nil
}

func (d *Device) GetLabel(ctx context.Context) (string, error) {
	payload, err := d.query(ctx, 23, 25, nil)
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
