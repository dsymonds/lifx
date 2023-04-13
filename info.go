package lifx

import (
	"context"
	"fmt"
)

func (d *Device) GetLabel(ctx context.Context) (string, error) {
	conn, err := udpConn(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	var hdr header
	hdr.frameHeader.source = 0xdeadbeef // TODO: randomly generate
	copy(hdr.frameAddress.target[0:6], d.Serial[:])
	hdr.frameAddress.resRequired = true
	hdr.frameAddress.sequence = 1 // TODO: sequence on a per device basis
	hdr.protocolHeader.typ = 23
	msg := encodeMessage(hdr, nil)

	if _, err := conn.WriteToUDP(msg, &d.Addr); err != nil {
		return "", fmt.Errorf("sending GetLabel: %v", err)
	}

	hdr, payload, _, err := readOnePacket(conn)
	if err != nil {
		return "", err
	}

	// TODO: Check that hdr.frameHeader.source matches what we sent out.
	if hdr.protocolHeader.typ != 25 {
		// Some different message for someone else?
		return "", fmt.Errorf("GetLabel triggered message type %d", hdr.protocolHeader.typ)
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
