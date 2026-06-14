package web

import (
	"net"
	"strings"
)

// lanIPv4 はこのマシンの非ループバックなプライベート IPv4 を1つ返す。
// iPhone に提示する「Wi-Fi プロキシのサーバ」欄に使う。見つからなければ ""。
//
// Docker bridge や VPN などの仮想インターフェースを誤って返さないよう、候補を
// スコア付けして最も「実際の LAN」らしいものを選ぶ。
func lanIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	bestIP := ""
	bestScore := -1
	for _, iface := range ifaces {
		// 停止中・ループバックは除外する。
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil || !ip.IsPrivate() {
				continue
			}
			if score := lanScore(iface.Name, ip); score > bestScore {
				bestScore = score
				bestIP = ip.String()
			}
		}
	}
	return bestIP
}

// lanScore は LAN らしさを採点する。値が大きいほど iPhone との同一 LAN らしい。
func lanScore(name string, ip net.IP) int {
	score := 0

	// 仮想・トンネル系インターフェースは強く減点する(macOS の代表例)。
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "en"): // 物理 Ethernet / Wi-Fi
		score += 100
	case hasAnyPrefix(lower, "bridge", "docker", "utun", "vnic", "vmenet",
		"awdl", "llw", "ap", "tap", "tun", "veth"):
		score -= 100
	}

	// アドレス帯でも補正する。192.168.0.0/16・10.0.0.0/8 を優先し、Docker が好む
	// 172.16.0.0/12 は下げる。net.IPv4 は16バイト表現を返すので 4 バイトに正規化する。
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 192 && v4[1] == 168:
			score += 20
		case v4[0] == 10:
			score += 10
		case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
			score -= 20
		}
	}
	return score
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
