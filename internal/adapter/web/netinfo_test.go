package web

import (
	"net"
	"testing"
)

func TestLanScorePrefersPhysicalLAN(t *testing.T) {
	en0WiFi := lanScore("en0", net.IPv4(192, 168, 1, 10))
	dockerBridge := lanScore("docker0", net.IPv4(172, 17, 0, 1))
	utun := lanScore("utun3", net.IPv4(10, 8, 0, 2))

	if en0WiFi <= dockerBridge {
		t.Fatalf("expected en0 (%d) to outscore docker bridge (%d)", en0WiFi, dockerBridge)
	}
	if en0WiFi <= utun {
		t.Fatalf("expected en0 (%d) to outscore VPN tunnel (%d)", en0WiFi, utun)
	}
}

func TestLanScorePrefers192Over172(t *testing.T) {
	homeLAN := lanScore("en0", net.IPv4(192, 168, 0, 5))
	dockerOnEn := lanScore("en0", net.IPv4(172, 20, 0, 5))

	if homeLAN <= dockerOnEn {
		t.Fatalf("expected 192.168 (%d) to outscore 172.x (%d)", homeLAN, dockerOnEn)
	}
}
