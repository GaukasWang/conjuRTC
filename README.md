# conjuRTC

An extension build for Conjure in order to support WebRTC datachannel transport 

### Specs

- [x] Transport receives an external raw inbound socket as `net.UDPConn`
- [ ] Transport automatically match each inbound connection with their registration (or specifically, their SDP* offer)
- [ ] Transport then returns a `net.Conn`-compiant struct as the wrapped Data Channel to be used to exchange data between the Server and the Client.

\* SDP is the abbreviation of **Session Description Protocol**, the protocol WebRTC uses to exchange peer connection info before a connection could be established.