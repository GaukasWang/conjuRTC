package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/Gaukas/seed2sdp"
	"github.com/Gaukas/transportc"
	"github.com/pion/ice/v2"
	"github.com/pion/webrtc/v3"
)

var (
	ErrArrLengthMismatch = errors.New("length of sdp slice and seeds do not match")
	connPool             = &sync.Map{}
)

func Conn2Mux(conn net.UDPConn) ice.UDPMux {
	return webrtc.NewICEUDPMux(nil, &conn)
}

// Mux2WebRTC creates a new WebRTC (as an answerer) DataChannel connection from a given mux
// For input, clientSDPs must all be of type Offer, seeds/clientSDPs must not be shared between multiple concurrent calls
// Mux2WebRTC will return a ready-to-go WebRTConn and the idx of which SDP is used.
func Mux2WebRTC(ctx context.Context, mux ice.UDPMux, seeds []string, clientSDPs []*seed2sdp.SDP, clientSetup string) (wrappedConn *transportc.WebRTConn, winnerId int, err error) {
	var abandoned = false
	var abandonedMutex = &sync.Mutex{}
	var chanConn chan *transportc.WebRTConn = make(chan *transportc.WebRTConn)
	winnerId = -1 // no one wins yet

	if len(clientSDPs) != len(seeds) {
		return nil, winnerId, ErrArrLengthMismatch
	}

	dcConfig := transportc.DataChannelConfig{
		Label:          "Conjure WebRTC Data Channel",
		SelfSDPType:    "answer",
		SendBufferSize: transportc.DataChannelBufferSizeDefault,
	}

	for i, clientSDP := range clientSDPs {
		seed := seeds[i]
		hkdfServer, hkdfClient := GetHKDFParamPair(seed)

		// First of all, complete the Client-generated SDP by inserting:
		// - Predicted Fingerprints
		// - Predicted ICE Parameters
		clientSDP.Fingerprint, err = seed2sdp.PredictDTLSFingerprint(hkdfClient)
		if err != nil {
			continue
		}
		clientSDP.IceParams, err = seed2sdp.PredictIceParameters(hkdfClient) // The deterministic
		if err != nil {
			continue
		}
		clientSDP.Malleables = seed2sdp.PredictSDPMalleables(hkdfClient) // It is temporarily hardcoded. Could be revisited in later versions.

		cert, err := seed2sdp.GetCertificate(hkdfServer)
		if err != nil {
			continue
		}
		iceParams, err := seed2sdp.PredictIceParameters(hkdfServer)
		if err != nil {
			continue
		}

		settingEngine := webrtc.SettingEngine{}
		iceParams.InjectSettingEngine(&settingEngine)

		webrtcConfiguration := webrtc.Configuration{
			Certificates: []webrtc.Certificate{
				cert,
			},
			// ICEServers: []webrtc.ICEServer{
			// 	{
			// 		URLs: []string{"stun:stun.l.google.com:19302"},
			// 	},
			// },
		}

		// m=application 9 UDP/DTLS/SCTP webrtc-datachannel
		clientSDP.AddMedia(seed2sdp.SDPMedia{
			MediaType:   "application",
			Description: "9 UDP/DTLS/SCTP webrtc-datachannel",
		})
		// a=group:BUNDLE 0
		clientSDP.AddAttrs(seed2sdp.SDPAttribute{
			Key:   "group",
			Value: "BUNDLE 0",
		})
		// a=mid:0
		clientSDP.AddAttrs(seed2sdp.SDPAttribute{
			Key:   "mid",
			Value: "0",
		})
		// a=sendrecv
		clientSDP.AddAttrs(seed2sdp.SDPAttribute{
			Value: "sendrecv",
		})
		// a=sctp-port:5000
		clientSDP.AddAttrs(seed2sdp.SDPAttribute{
			Key:   "sctp-port",
			Value: "5000",
		})
		// a=setup:actpass
		clientSDP.AddAttrs(seed2sdp.SDPAttribute{
			Key:   "setup",
			Value: clientSetup,
		})

		clientSDPstr := clientSDP.String()
		currentIdx := i

		go func() {
			conn, err := transportc.Dial("udp", "0.0.0.0")
			if err != nil {
				return
			}

			err = conn.Init(&dcConfig, settingEngine, webrtcConfiguration)
			if err != nil {
				return
			}
			conn.SetRemoteSDPJsonString(clientSDPstr)
			_, err = conn.LocalSDP() // force answerer to generate local SDP
			if err != nil {
				return
			}

			// Known issue:
			// If more than one conn is established, only first one will be honored by the server.
			// Currently we will close out all the others (asking them to retry)
			for (conn.Status() & transportc.WebRTConnReady) == 0 {
				time.Sleep(time.Millisecond * 10)
				abandonedMutex.Lock()
				defer abandonedMutex.Unlock()
				if abandoned { // Some critical error, or already a valid candidate
					conn.Close()
					return
				}
				if conn.Status()&transportc.WebRTConnClosed == 0 || conn.Status()&transportc.WebRTConnErrored == 0 {
					return
				}
			}

			// If it make it to here, it is ready. Signal all other goroutines to abandon
			abandonedMutex.Lock()
			if !abandoned {
				conn.Write([]byte("WINNER"))
				abandoned = true
				chanConn <- conn
				winnerId = currentIdx
			} else {
				conn.Write([]byte("RETRY"))
				conn.Close()
			}
			abandonedMutex.Unlock()
		}()

	}

	select {
	case wrappedConn = <-chanConn:
		return wrappedConn, winnerId, nil
	case <-ctx.Done():
		return nil, winnerId, ctx.Err()
	}
}
