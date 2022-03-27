package main

import (
	"context"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/Gaukas/seed2sdp"
	"github.com/GaukasWang/conjuRTC/transport"
	"github.com/gin-gonic/gin"
	"github.com/pion/ice/v2"
)

var (
	mapMux   = map[uint16]ice.UDPMux{}
	mapMutex = &sync.Mutex{}
	mapSdp   = map[string]*seed2sdp.SDP{}
	mapConn  = map[string]net.Conn{}
)

func initGin(router *gin.Engine) {
	router.POST("/submitsdp", func(c *gin.Context) {
		sdp := c.PostForm("sdp")
		seed := c.PostForm("seed")

		if sdp == "" || seed == "" {
			c.AbortWithStatusJSON(400, gin.H{
				"message": "parameter sdp and seed are required",
			})
			return
		}

		var deflatedSDP seed2sdp.SDPDeflated
		deflatedSDP, err := seed2sdp.SDPDeflatedFromString(sdp)
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{
				"message": "invalid deflated sdp",
			})
			return
		}

		sdpParsed := &seed2sdp.SDP{}
		sdpParsed, err = deflatedSDP.Inflate()
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{
				"message": "sdp does not inflate",
			})
			return
		}

		mapMutex.Lock()
		mapSdp[seed] = sdpParsed
		mapMutex.Unlock()

		c.JSON(200, gin.H{
			"message": "success",
		})
	})

	// acceptconn will accept 1 connection on a certain port
	router.GET("/acceptconn", func(c *gin.Context) {
		portstr := c.Query("port")
		var mux ice.UDPMux
		var port uint16

		// Convert port into uint16
		port64, err := strconv.ParseUint(portstr, 10, 16)
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{
				"message": "invalid port",
			})
			return
		}
		port = uint16(port64)

		if port > 8880 && port <= 8890 {
			mux = mapMux[port]
		} else {
			c.AbortWithStatusJSON(400, gin.H{
				"message": "invalid port",
			})
			return
		}

		go acceptConn(mux, port)
	})
}

func acceptConn(mux ice.UDPMux, port uint16) {
	// Build seed-sdp slices
	ctx := context.Background()
	var seeds []string
	var clientSDPs []*seed2sdp.SDP
	mapMutex.Lock()
	defer mapMutex.Unlock()
	for seed, sdp := range mapSdp {
		seeds = append(seeds, seed)
		clientSDPs = append(clientSDPs, sdp)
	}
	clientSetup := transport.CLIENT_SETUP_ACTPASS

	conn, id, err := transport.Mux2WebRTC(ctx, mux, seeds, clientSDPs, clientSetup)
	if err != nil {
		log.Printf("acceptConn: %s\n", err.Error())
		return
	}

	log.Printf("acceptConn() established WebRTConn with seed[%s] on port[%d]. Verifying the connectivity...\n", seeds[id], port)
	recv := make([]byte, 1024)
	n, err := conn.Read(recv)
	if err != nil {
		log.Printf("conn.Read: %s\n", err.Error())
		return
	}
	log.Printf("#%s: %s\n", seeds[id], string(recv[:n]))
	log.Printf("acceptConn() successfully received %d bytes from seed[%s] on port[%d]\n", n, seeds[id], port)

	mapConn[seeds[id]] = conn
}

func main() {
	var port uint16
	for port = 8881; port <= 8890; port++ {
		rs, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.IPv4(10, 0, 0, 11),
			Port: int(port),
		})
		if err != nil {
			panic(err)
		}
		mapMux[port] = transport.Conn2Mux(rs)
	}

	// Create a Gin router to receive POSTed Client SDP
	router := gin.Default()
	initGin(router)

	select {}
}
