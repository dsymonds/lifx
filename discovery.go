package lifx

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

const (
	stdPort = 56700
)

// Device represents a LIFX device on the local network.
//
// A device is bound to the Client that discovered it.
type Device struct {
	Addr   net.UDPAddr
	Serial [6]byte

	client *Client
	seq    uint8 // sequence number for this device
}

// Discover probes the network for LIFX devices.
// The provided context controls how long to wait for responses;
// its cancellation or deadline expiry will stop execution of Discover
// but will not return an error.
func (c *Client) Discover(ctx context.Context) ([]Device, error) {
	// Use a distinct UDP conn just for discovery so we control the timeout.
	conn, err := udpConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// https://lan.developer.lifx.com/docs/querying-the-device-for-data#discovery

	// Discovery: GetService(2) with tagged=1.
	var hdr header
	hdr.frameHeader.tagged = true
	hdr.frameHeader.source = c.source
	// hdr.frameAddress.target left as zero (all devices)
	hdr.frameAddress.resRequired = false // documented recommendation
	hdr.frameAddress.ackRequired = false // ditto
	hdr.protocolHeader.typ = uint16(pktGetService)
	msg := encodeMessage(hdr, nil)

	dst := &net.UDPAddr{
		IP:   net.IPv4(255, 255, 255, 255),
		Port: stdPort,
	}
	if _, err := conn.WriteToUDP(msg, dst); err != nil {
		return nil, fmt.Errorf("sending discovery request: %v", err)
	}

	// Wait for any responses.
	var devs []Device
	for {
		hdr, payload, raddr, err := readOnePacket(conn)
		if err != nil {
			var neterr net.Error
			if errors.As(err, &neterr) && neterr.Timeout() {
				// Not a failure.
				break
			}
			return nil, err
		}

		if hdr.frameHeader.source != c.source {
			return nil, fmt.Errorf("received message source 0x%x (want 0x%x)", hdr.frameHeader.source, c.source)
		}
		if rt := msgType(hdr.protocolHeader.typ); rt != pktStateService {
			// Some different message for someone else?
			return nil, fmt.Errorf("received message type %d (want %d)", rt, pktStateService)
		}
		if len(payload) != 5 {
			return nil, fmt.Errorf("StateService response had bad payload length %d", len(payload))
		}
		if payload[0] != 0x01 { // We only care about service=UDP
			continue
		}
		port := binary.LittleEndian.Uint32(payload[1:5])
		if port > 0xffff {
			return nil, fmt.Errorf("StateService response payload has illegal port field %x", payload[1:5])
		}

		devs = append(devs, Device{
			// Per docs, use the remote IP address, but the port from the payload.
			Addr: net.UDPAddr{
				IP:   raddr.IP,
				Port: int(port),
			},
			Serial: [6]byte(hdr.frameAddress.target[0:6]),

			client: c,
			seq:    1,
		})
	}
	return devs, nil
}
