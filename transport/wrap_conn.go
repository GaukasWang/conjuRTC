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

type ClientSetup string

const (
	CLIENT_SETUP_ACTPASS ClientSetup = "actpass"
	CLIENT_SETUP_ACTIVE  ClientSetup = "active"
	CLIENT_SETUP_PASSIVE ClientSetup = "passive"
)

var (
	ErrArrLengthMismatch = errors.New("length of sdp slice and seeds do not match")
	ErrNoRegistration    = errors.New("no registration found")
)

func Conn2Mux(conn *net.UDPConn) ice.UDPMux {
	return webrtc.NewICEUDPMux(nil, conn)
}

// Mux2WebRTC creates a new WebRTC (as an answerer) DataChannel connection from a given mux
// For input, clientSDPs must all be of type Offer, seeds/clientSDPs must not be shared between multiple concurrent calls
// Mux2WebRTC will return a ready-to-go WebRTConn and the idx of which SDP is used.
func Mux2WebRTC(ctx context.Context, mux ice.UDPMux, seeds []string, clientSDPs []*seed2sdp.SDP, clientSetup ClientSetup) (wrappedConn *transportc.WebRTConn, matchedID int, err error) {
	var abandoned = false
	var abandonedMutex = &sync.Mutex{}
	var chanConn chan *transportc.WebRTConn = make(chan *transportc.WebRTConn)
	var winnerId int = -1 // no one wins yet

	if len(clientSDPs) == 0 {
		return nil, winnerId, ErrNoRegistration
	}

	if len(clientSDPs) != len(seeds) {
		return nil, winnerId, ErrArrLengthMismatch
	}

	dcConfig := transportc.DataChannelConfig{
		Label:          "Conjure WebRTC Data Channel",
		SelfSDPType:    "answer",
		SendBufferSize: 0,
	}

	for i, clientSDP := range clientSDPs {
		seed := seeds[i]
		hkdfServer, hkdfClient := GetHKDFParamPair(seed)

		// Based on the seed and the deflated SDP, predict the complete SDP generated by the client(peer)
		err = completeClientSDP(clientSDP, hkdfClient, clientSetup)
		if err != nil {
			continue // ignore invalid SDPs
		}

		clientSDPstr := clientSDP.String()
		currentIdx := i

		// Based on the seed, lock the complete SDP generated by server(self)
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
				time.Sleep(time.Millisecond * 5)

				abandonedMutex.Lock()
				defer abandonedMutex.Unlock()

				// abandoned due to another candidate stands out - return
				if abandoned {
					conn.Close()
					return
				}

				// Closed or Errored - return
				if conn.Status()&transportc.WebRTConnClosed == 1 || conn.Status()&transportc.WebRTConnErrored == 1 {
					return
				}
			}

			// If it make it to here, it is ready. Signal all other goroutines to abandon
			abandonedMutex.Lock()
			if !abandoned {
				// Ready to go
				conn.Write([]byte("WINNER"))
				abandoned = true
				chanConn <- conn
				winnerId = currentIdx
			} else {
				// tell other clients to retry connecting.
				// maybe the client should reuse the old registration to prevent repeated registration?
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

func completeClientSDP(clientSDP *seed2sdp.SDP, hkdfClient *seed2sdp.HKDFParams, clientSetup ClientSetup) error {
	var err error
	clientSDP.Fingerprint, err = seed2sdp.PredictDTLSFingerprint(hkdfClient)
	if err != nil {
		return err
	}
	clientSDP.IceParams, err = seed2sdp.PredictIceParameters(hkdfClient) // The deterministic
	if err != nil {
		return err
	}
	clientSDP.Malleables = seed2sdp.PredictSDPMalleables(hkdfClient) // currently hardcoded

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
		Value: string(clientSetup),
	})

	return nil
}
