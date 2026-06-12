package domain

import (
	"fmt"
	"net/netip"
	"strings"
)

// AllowList はプロキシ接続を許可するクライアントの集合（IP または CIDR）。
// 空の場合は全クライアントを許可する。
type AllowList struct {
	prefixes []netip.Prefix
}

// NewAllowList は IP または CIDR の指定群から AllowList を生成する。
// 空文字や空のスライスは「全許可」を意味する。
func NewAllowList(specs []string) (AllowList, error) {
	var prefixes []netip.Prefix
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if strings.Contains(spec, "/") {
			p, err := netip.ParsePrefix(spec)
			if err != nil {
				return AllowList{}, fmt.Errorf("invalid --allow %q: %w", spec, err)
			}
			prefixes = append(prefixes, p.Masked())
			continue
		}
		addr, err := netip.ParseAddr(spec)
		if err != nil {
			return AllowList{}, fmt.Errorf("invalid --allow %q: %w", spec, err)
		}
		// IPv4射影アドレス（::ffff:a.b.c.d）は IPv4 に正規化する。
		// クライアント側も Allows で Unmap するため、両者を揃える。
		addr = addr.Unmap()
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return AllowList{prefixes: prefixes}, nil
}

// AllowsAll は許可リストが空（全クライアント許可）の場合に true を返す。
func (a AllowList) AllowsAll() bool { return len(a.prefixes) == 0 }

// Allows は addr が許可対象かを返す。許可リストが空なら常に true。
func (a AllowList) Allows(addr netip.Addr) bool {
	if a.AllowsAll() {
		return true
	}
	addr = addr.Unmap()
	for _, p := range a.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
