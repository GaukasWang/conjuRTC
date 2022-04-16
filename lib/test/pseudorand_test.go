package conjurtc_test

import (
	"testing"

	conjurtc "github.com/GaukasWang/conjuRTC/lib"
)

// Test Pseudorand functions

func TestRandPort(t *testing.T) {
	basePort := uint16(9000)
	portRange := int64(100)

	seed1 := "HiConjureHi"
	seed2 := "HelloConjureHello"
	seed3 := "Greetings"
	seed1Clone := seed1

	conjurtc.SetBasePort(basePort)

	port1 := conjurtc.RandPort(seed1)
	if port1 >= basePort+uint16(portRange) || port1 < basePort {
		t.Errorf("RandPort(%s) = %d, expected %d <= %d <= %d", seed1, port1, basePort, port1, basePort+uint16(portRange))
	}
	port2 := conjurtc.RandPort(seed2)
	if port2 >= basePort+uint16(portRange) || port2 < basePort {
		t.Errorf("RandPort(%s) = %d, expected %d <= %d <= %d", seed2, port2, basePort, port2, basePort+uint16(portRange))
	}
	port3 := conjurtc.RandPort(seed3)
	if port3 >= basePort+uint16(portRange) || port3 < basePort {
		t.Errorf("RandPort(%s) = %d, expected %d <= %d <= %d", seed3, port3, basePort, port3, basePort+uint16(portRange))
	}
	port1Clone := conjurtc.RandPort(seed1Clone)
	if port1 != port1Clone {
		t.Errorf("RandPort(%s) = %d, expected %d", seed1, port1, port1Clone)
	}
}
