package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	transport "github.com/GaukasWang/conjuRTC/lib"
	"golang.org/x/net/context"
)

func main() {
	basePort := uint16(9000)
	portRange := int64(20)
	transport.SetBasePort(basePort)
	transport.SetPortRange(portRange)

	// Generate new Client instance
	c, err := transport.DefaultClient()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Will connect to port: %d\n", transport.RandPort(c.Seed()))

	// Prepare for connection
	fmt.Println("Preparing Client...")
	err = c.Prepare()
	if err != nil {
		panic(err)
	}

	// Dump LocalSDP
	fmt.Println("Fetching local SDP...")
	sdp, err := c.LocalSDP()
	if err != nil {
		panic(err)
	}

	localSDPDeflated := sdp.Deflate([]net.IP{
		net.ParseIP("10.0.0.11"),
		// net.ParseIP("2601:281:8400:37e0:4586:eff8:7f37:f9c9"),
		// net.ParseIP("2601:281:8400:37e0:18a0:28b:b836:22b6"),
	})
	fmt.Println("Register at station...")
	register(c.Seed(), localSDPDeflated.String())

	time.Sleep(time.Second * 5)

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelFunc()

	phantom1 := net.ParseIP("192.168.217.1")
	phantom2 := net.ParseIP("2601:281:8400:37e0:4586:eff8:7f37:f9c9")
	phantom3 := net.ParseIP("2601:281:8400:37e0:18a0:28b:b836:22b6")
	phantoms := []*net.IP{
		&phantom1,
		&phantom2,
		&phantom3,
	}
	fmt.Println("Connecting...")
	conn, err := c.Connect(ctx, phantoms)
	if err != nil {
		panic(err)
	}
	fmt.Println("Connected.")

	conn.Write([]byte("Hello, World!"))
	select {}
}

func register(seed, sdp string) {
	data := url.Values{
		"sdp":  {sdp},
		"seed": {seed},
	}

	_, err := http.PostForm("http://localhost:8443/submitsdp", data)
	if err != nil {
		panic(err)
	}
}
