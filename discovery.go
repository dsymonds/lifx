package lifx

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

const (
	stdPort = 56700
)

func udpConn(ctx context.Context) (*net.UDPConn, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, fmt.Errorf("net.ListenUDP: %v", err)
	}
	if d, ok := ctx.Deadline(); ok { // TODO: force a deadline if none provided?
		conn.SetReadDeadline(d)
	}
	return conn, nil
}

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

// Device represents a LIFX device on the local network.
type Device struct {
	Addr   net.UDPAddr
	Serial [6]byte
}

// Discover probes the network for LIFX devices.
// The provided context controls how long to wait for responses;
// its cancellation or deadline expiry will stop execution of Discover
// but will not return an error.
func Discover(ctx context.Context) ([]Device, error) {
	conn, err := udpConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// https://lan.developer.lifx.com/docs/querying-the-device-for-data#discovery

	// Discovery: GetService(2) with tagged=1.
	var hdr header
	hdr.frameHeader.tagged = true
	hdr.frameHeader.source = 0xdeadbeef // TODO: randomly generate
	// hdr.frameAddress.target left as zero (all devices)
	hdr.frameAddress.resRequired = false // documented recommendation
	hdr.frameAddress.ackRequired = false
	hdr.frameAddress.sequence = 1 // TODO: sequence on a per device basis
	hdr.protocolHeader.typ = 2
	msg := encodeMessage(hdr, nil)

	dst := &net.UDPAddr{
		IP:   net.IPv4(255, 255, 255, 255),
		Port: stdPort,
	}
	//log.Printf("sending %d byte message: %x", len(msg), msg)
	if _, err := conn.WriteToUDP(msg, dst); err != nil {
		return nil, fmt.Errorf("sending discovery request: %v", err)
	}

	// Wait for any responses.
	var devs []Device
	var scratch [4 << 10]byte
	for {
		nb, raddr, err := conn.ReadFrom(scratch[:])
		if err != nil {
			if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
				break
			}
			return nil, fmt.Errorf("reading response: %v", err)
		}
		b := scratch[:nb]
		//log.Printf("got back %d bytes from %s", nb, raddr)

		hdr, payload, err := decodeMessage(b)
		if err != nil {
			log.Printf("Decoding discovery response: %v", err)
			continue
		}
		// TODO: Check that hdr.frameHeader.source matches what we sent out.
		if hdr.protocolHeader.typ != 3 {
			// Some different message for someone else?
			continue
		}
		if len(payload) != 5 {
			log.Printf("StateService response had bad payload length %d", len(payload))
			continue
		}
		if payload[0] != 0x01 { // We only care about service=UDP
			continue
		}
		port := binary.LittleEndian.Uint32(payload[1:5])
		if port > 0xffff {
			log.Printf("StateService response payload has illegal port field %x", payload[1:5])
			continue
		}

		devs = append(devs, Device{
			// Per docs, use the remote IP address, but the port from the payload.
			Addr: net.UDPAddr{
				IP:   raddr.(*net.UDPAddr).IP,
				Port: int(port),
			},
			Serial: [6]byte(hdr.frameAddress.target[0:6]),
		})
	}
	return devs, nil
}
