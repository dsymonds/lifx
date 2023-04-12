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
