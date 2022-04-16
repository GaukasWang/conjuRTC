package conjurtc

import (
	"crypto/rand"
	"crypto/sha256"
	"math/big"

	"github.com/Gaukas/seed2sdp"
	"golang.org/x/crypto/hkdf"
)

// File: pseudorand.go
// Package: utils
// Author: Gaukas <i@gauk.as>
// Description: Some helper functions to utilize package seed2sdp.

var (
	clientIdentifier        = "Client"
	serverIdentifier        = "Server"
	defaultRandSalt         = "testSalt"
	basePort         uint16 = 8900
	portRange        int64  = 100
)

func GetHKDFParamPair(seed string) (server, client *seed2sdp.HKDFParams) {
	server = seed2sdp.NewHKDFParams().SetSecret(seed).SetSalt(defaultRandSalt).SetInfoPrefix(serverIdentifier)
	client = seed2sdp.NewHKDFParams().SetSecret(seed).SetSalt(defaultRandSalt).SetInfoPrefix(clientIdentifier)

	return server, client
}

func SetClientIdentifier(identifier string) {
	clientIdentifier = identifier
}

func SetServerIdentifier(identifier string) {
	serverIdentifier = identifier
}

func SetRandSalt(salt string) {
	defaultRandSalt = salt
}

func RandPort(seed string) uint16 {
	hkdfReader := hkdf.New(sha256.New, []byte(seed), []byte(defaultRandSalt), nil)
	offset, err := rand.Int(hkdfReader, big.NewInt(portRange))
	if err != nil {
		return 0
	}
	return basePort + uint16(offset.Uint64())
}

func SetBasePort(port uint16) {
	basePort = port
}

func SetPortRange(range_ int64) {
	portRange = range_
}
