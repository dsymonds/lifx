package lifx

import (
	"encoding/binary"
	"fmt"
)

// header represents a LIFX message header.
//
// https://lan.developer.lifx.com/docs/packet-contents#header
type header struct {
	// This structure only contains fields that are settable.
	// The rest are fixed or computed.

	// https://lan.developer.lifx.com/docs/packet-contents#frame-header
	frameHeader struct {
		//size        uint16
		//protocol    uint16
		//addressable bool
		tagged bool
		//origin uint8
		source uint32
	}

	// https://lan.developer.lifx.com/docs/packet-contents#frame-address
	frameAddress struct {
		target      [8]uint8
		resRequired bool
		ackRequired bool
		sequence    uint8
	}

	// https://lan.developer.lifx.com/docs/packet-contents#protocol-header
	protocolHeader struct {
		typ uint16
	}
}

func encodeMessage(hdr header, payload []byte) []byte {
	bit := func(b bool) uint {
		if b {
			return 1
		}
		return 0
	}

	finalSize := 8 + 16 + 12 + len(payload)
	out := make([]byte, 0, finalSize)

	// Frame header (8 bytes).
	out = binary.LittleEndian.AppendUint16(out, uint16(finalSize))
	out = append(out, 0)                                                   // low byte of protocol (1024)
	out = append(out, byte(0x04|1<<4|bit(hdr.frameHeader.tagged)<<5|0<<6)) // remainder of protol, addressable, tagged, origin
	out = binary.LittleEndian.AppendUint32(out, hdr.frameHeader.source)

	// Frame address (16 bytes).
	out = append(out, hdr.frameAddress.target[:]...)
	out = append(out, 0, 0, 0, 0, 0, 0)                                                             // reserved
	out = append(out, byte(bit(hdr.frameAddress.resRequired)|bit(hdr.frameAddress.ackRequired)<<1)) // and 6 reserved bits
	out = append(out, hdr.frameAddress.sequence)

	// Protocol header (12 bytes).
	out = append(out, 0, 0, 0, 0, 0, 0, 0, 0) // reserved
	out = binary.LittleEndian.AppendUint16(out, hdr.protocolHeader.typ)
	out = append(out, 0, 0) // reserved

	// Payload itself.
	out = append(out, payload...)

	if len(out) != finalSize {
		panic(fmt.Sprintf("internal error: encoded message to %d bytes but it should have been %d bytes", len(out), finalSize))
	}
	return out
}

func decodeMessage(b []byte) (hdr header, payload []byte, err error) {
	if len(b) < 36 {
		err = fmt.Errorf("message too short: %d bytes < minimum 36 bytes", len(b))
		return
	}
	finalSize := int(binary.LittleEndian.Uint16(b[0:2]))
	if finalSize != len(b) {
		err = fmt.Errorf("message has invalid size %d; got %d bytes", finalSize, len(b))
		return
	}
	b, payload = b[:36], b[36:]

	// TODO: target?
	hdr.frameHeader.source = binary.LittleEndian.Uint32(b[4:8])

	copy(hdr.frameAddress.target[:], b[8:16])
	// TODO: resRequired? ackRequired?
	hdr.frameAddress.sequence = b[23]

	hdr.protocolHeader.typ = binary.LittleEndian.Uint16(b[32:34])

	return
}
