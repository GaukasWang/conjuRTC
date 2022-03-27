package transport

import (
	"context"
	"net"

	"github.com/Gaukas/transportc"
)

type ClientDialerFunc func(dialContext context.Context, network string, address string) (net.Conn, error)

type Client struct {
	WebrtcSeed string // Need to be included in the registration Client-to-Station
	webRTConn  *transportc.WebRTConn
	Dialer     ClientDialerFunc
}

func NewClient(seed string) Client {
	// Initialize the WebRTConn

	return Client{
		WebrtcSeed: seed,
		Dialer: func(dialContext context.Context, network string, address string) (net.Conn, error) {
			return nil, nil
		},
	}
}

func (c *Client) Connect() {
	// When building the UDP mux, add all phantoms' ICE candidate into one single SDP
}
