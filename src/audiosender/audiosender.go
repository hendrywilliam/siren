package audiosender

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

// Data interpolation
var SILENCE_FRAMES = []byte{0xF8, 0xFF, 0xFE}

type AudioSender struct {
	sequence  uint16
	timestamp uint32
	ssrc      uint32
}

func (as *AudioSender) Send(ctx context.Context, udpConn *net.UDPConn, secretKeys [32]byte, data <-chan []byte, done chan bool) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case frame := <-data:
			packet, err := as.encrypt(secretKeys, frame)
			if err != nil {
				return err
			}

			_, err = udpConn.Write(packet)
			if err != nil {
				return err
			}

		case <-ticker.C:
		}
	}
}

func (as *AudioSender) encrypt(secretKeys [32]byte, rawData []byte) ([]byte, error) {

	rtpHeader := make([]byte, 12)
	rtpHeader[0] = 0x80
	rtpHeader[1] = 0x78
	binary.BigEndian.PutUint16(rtpHeader[2:], as.sequence)
	binary.BigEndian.PutUint32(rtpHeader[4:], as.timestamp)
	binary.BigEndian.PutUint32(rtpHeader[8:], as.ssrc)

	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	copy(nonce, rtpHeader)

	aead, err := chacha20poly1305.NewX(secretKeys[:])
	if err != nil {
		return nil, err
	}

	encrypted := aead.Seal(nil, nonce, rawData, nil)

	packet := append(rtpHeader, encrypted...)

	atomic.AddUint32(&as.timestamp, 960)
	as.sequence++

	return packet, nil
}
