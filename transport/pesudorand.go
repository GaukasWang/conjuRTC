package transport

import "github.com/Gaukas/seed2sdp"

// File: utils/seed2sdp.go
// Package: utils
// Author: Gaukas <i@gauk.as>
// Description: Some helper functions to utilize package seed2sdp.

var (
	clientIdentifier = "Client"
	serverIdentifier = "Server"
	defaultRandSalt  = "testSalt"
)

func GetHKDFParamPair(seed string) (server, client *seed2sdp.HKDFParams) {
	server = seed2sdp.NewHKDFParams().SetSecret(seed).SetSalt(defaultRandSalt).SetInfoPrefix(serverIdentifier)
	client = seed2sdp.NewHKDFParams().SetSecret(seed).SetSalt(defaultRandSalt).SetInfoPrefix(clientIdentifier)

	return server, client
}
