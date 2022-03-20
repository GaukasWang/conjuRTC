package transport

import "net"

type Transport struct{}

func (Transport) Name() string      { return "webrtc" }
func (Transport) LogPrefix() string { return "WEBRTC" }

func (Transport) MatchConnectionWithReg(conn *net.UDPConn)
