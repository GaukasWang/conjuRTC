package transport

import (
	"context"
	"crypto/rand"
	"errors"
	"net"

	"github.com/Gaukas/seed2sdp"
	"github.com/Gaukas/transportc"
	"github.com/pion/webrtc/v3"
)

type Client struct {
	// Transport Implementation
	webRTConn *transportc.WebRTConn

	// Pseudorandom Seed
	webrtcSeed string // Need to be included in the registration Client-to-Station, Readonly after set

	// Reusable HKDF parameters based on the seed
	hkdfServer *seed2sdp.HKDFParams
	hkdfClient *seed2sdp.HKDFParams

	// ICE Configs
	iceServers []webrtc.ICEServer
}

func NewClient(seed string, iceServers []webrtc.ICEServer) *Client {
	// Initialize the WebRTConn
	hkdfServer, hkdfClient := GetHKDFParamPair(seed)
	conn, _ := transportc.Dial("udp", "0.0.0.0")
	return &Client{
		webrtcSeed: seed,
		webRTConn:  conn,
		hkdfServer: hkdfServer,
		hkdfClient: hkdfClient,
		iceServers: iceServers,
	}
}

// DefaultClient() generates a crypto/rand seed for a new client
func DefaultClient() (*Client, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return NewClient(
		string(b),
		[]webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	), nil
}

func (c *Client) Seed() string {
	return c.webrtcSeed
}

// Prepare() initializes the WebRTConn and generates the local SDP offer
// This function should be called before c.LocalSDP() or c.DeflatedLocalSDP()
func (c *Client) Prepare() error {
	cert, err := seed2sdp.GetCertificate(c.hkdfClient)
	if err != nil {
		return err
	}
	iceParams, err := seed2sdp.PredictIceParameters(c.hkdfClient)
	if err != nil {
		return err
	}

	newDCConfig := transportc.DataChannelConfig{
		Label:          "Conjure WebRTC Data Channel",
		SelfSDPType:    "offer",
		SendBufferSize: 0, // unlimited buffer size
	}
	newSettingEngine := webrtc.SettingEngine{}
	iceParams.InjectSettingEngine(&newSettingEngine)

	newConfiguration := webrtc.Configuration{
		Certificates: []webrtc.Certificate{cert},
		ICEServers:   c.iceServers,
	}

	err = c.webRTConn.Init(&newDCConfig, newSettingEngine, newConfiguration)

	return err
}

func (c *Client) LocalSDP() (*seed2sdp.SDP, error) {
	locapSdp, err := c.webRTConn.LocalSDPJsonString()
	if err != nil {
		return nil, err
	}

	sdp := seed2sdp.ParseSDP(locapSdp)
	return &sdp, nil
}

// When building the UDP mux, add all phantoms' ICE candidate into one single SDP
func (c *Client) Connect(ctx context.Context, phantoms []*net.IP) (net.Conn, error) {
	var err error

	// Guessing server SDP answer
	serverSDP := seed2sdp.SDP{
		SDPType:    "answer",
		Malleables: seed2sdp.PredictSDPMalleables(c.hkdfServer),
	}

	serverSDP.Fingerprint, err = seed2sdp.PredictDTLSFingerprint(c.hkdfServer)
	if err != nil {
		return nil, err
	}
	serverSDP.IceParams, err = seed2sdp.PredictIceParameters(c.hkdfServer) // The deterministic
	if err != nil {
		return nil, err
	}

	// m=application 9 UDP/DTLS/SCTP webrtc-datachannel
	serverSDP.AddMedia(seed2sdp.SDPMedia{
		MediaType:   "application",
		Description: "9 UDP/DTLS/SCTP webrtc-datachannel",
	})
	// a=group:BUNDLE 0
	serverSDP.AddAttrs(seed2sdp.SDPAttribute{
		Key:   "group",
		Value: "BUNDLE 0",
	})
	// a=mid:0
	serverSDP.AddAttrs(seed2sdp.SDPAttribute{
		Key:   "mid",
		Value: "0",
	})
	// a=sendrecv
	serverSDP.AddAttrs(seed2sdp.SDPAttribute{
		Value: "sendrecv",
	})
	// a=sctp-port:5000
	serverSDP.AddAttrs(seed2sdp.SDPAttribute{
		Key:   "sctp-port",
		Value: "5000",
	})
	// a=setup:active
	serverSDP.AddAttrs(seed2sdp.SDPAttribute{
		Key:   "setup",
		Value: "active",
	})

	for _, phantom := range phantoms {
		rtpCandidate := seed2sdp.ICECandidate{}
		rtpCandidate.SetComponent(seed2sdp.ICEComponentRTP)
		rtpCandidate.SetProtocol(seed2sdp.UDP)
		rtpCandidate.SetIpAddr(*phantom)
		rtpCandidate.SetPort(RandPort(c.Seed()))
		rtpCandidate.SetCandidateType(seed2sdp.Host) // Srflx?

		rtcpCandidate := seed2sdp.ICECandidate{}
		rtcpCandidate.SetComponent(seed2sdp.ICEComponentRTCP)
		rtcpCandidate.SetProtocol(seed2sdp.UDP)
		rtcpCandidate.SetIpAddr(*phantom)
		rtcpCandidate.SetPort(RandPort(c.Seed()))
		rtcpCandidate.SetCandidateType(seed2sdp.Host)

		serverSDP.IceCandidates = append(serverSDP.IceCandidates, rtpCandidate, rtcpCandidate)
	}

	// Set the remote SDP
	err = c.webRTConn.SetRemoteSDPJsonString(serverSDP.String())
	if err != nil {
		return nil, err
	}

	// Block until conn is good to go
	for (c.webRTConn.Status() & transportc.WebRTConnReady) == 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// Read the first packet to make sure this connection is accepted by the station
	buf := make([]byte, 8)
	n := 0
	for n == 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		n, _ = c.webRTConn.Read(buf)
	}
	if string(buf) == "WINNER" { // Accepted
		return c.webRTConn, nil
	} else if string(buf) == "RETRY" { // Rejected
		return nil, errors.New("connection closed by station due to a socket conflict")
	} else {
		return nil, errors.New("unexpected packet, maybe pipe is broken")
	}
}
