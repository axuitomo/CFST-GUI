package colodict

import (
	"math/big"
	"net/netip"
	"strings"
)

func tokenPrefix(token string) (netip.Prefix, bool) {
	if strings.Contains(token, "/") {
		prefix, err := netip.ParsePrefix(token)
		if err != nil {
			return netip.Prefix{}, false
		}
		return prefix.Masked(), true
	}
	addr, err := netip.ParseAddr(token)
	if err != nil {
		return netip.Prefix{}, false
	}
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32), true
	}
	return netip.PrefixFrom(addr, 128), true
}

func intersectPrefixes(left, right netip.Prefix) (netip.Prefix, bool) {
	left = left.Masked()
	right = right.Masked()
	if left.Addr().Is4() != right.Addr().Is4() {
		return netip.Prefix{}, false
	}
	if left.Bits() >= right.Bits() {
		if right.Contains(left.Addr()) {
			return left, true
		}
		return netip.Prefix{}, false
	}
	if left.Contains(right.Addr()) {
		return right, true
	}
	return netip.Prefix{}, false
}

func subtractPrefix(source, remove netip.Prefix) []netip.Prefix {
	source = source.Masked()
	remove = remove.Masked()
	if !prefixesOverlap(source, remove) {
		return []netip.Prefix{source}
	}
	if remove.Bits() <= source.Bits() && remove.Contains(source.Addr()) {
		return nil
	}
	maxBits := prefixMaxBits(source)
	if source.Bits() >= maxBits {
		return []netip.Prefix{source}
	}
	left, right := splitPrefix(source)
	result := make([]netip.Prefix, 0, 2)
	result = append(result, subtractPrefix(left, remove)...)
	result = append(result, subtractPrefix(right, remove)...)
	return result
}

func prefixesOverlap(left, right netip.Prefix) bool {
	left = left.Masked()
	right = right.Masked()
	if left.Addr().Is4() != right.Addr().Is4() {
		return false
	}
	return left.Contains(right.Addr()) || right.Contains(left.Addr())
}

func splitPrefix(prefix netip.Prefix) (netip.Prefix, netip.Prefix) {
	prefix = prefix.Masked()
	nextBits := prefix.Bits() + 1
	step := new(big.Int).Lsh(big.NewInt(1), uint(prefixMaxBits(prefix)-nextBits))
	secondAddr := bigToAddr(new(big.Int).Add(addrToBig(prefix.Addr()), step), prefix.Addr().Is4())
	return netip.PrefixFrom(prefix.Addr(), nextBits).Masked(), netip.PrefixFrom(secondAddr, nextBits).Masked()
}

func prefixMaxBits(prefix netip.Prefix) int {
	if prefix.Addr().Is4() {
		return 32
	}
	return 128
}

func addrToBig(addr netip.Addr) *big.Int {
	if addr.Is4() {
		raw := addr.As4()
		return new(big.Int).SetBytes(raw[:])
	}
	raw := addr.As16()
	return new(big.Int).SetBytes(raw[:])
}

func bigToAddr(value *big.Int, ipv4 bool) netip.Addr {
	size := 16
	if ipv4 {
		size = 4
	}
	raw := value.Bytes()
	if len(raw) > size {
		raw = raw[len(raw)-size:]
	}
	padded := make([]byte, size)
	copy(padded[size-len(raw):], raw)
	if ipv4 {
		return netip.AddrFrom4([4]byte{padded[0], padded[1], padded[2], padded[3]})
	}
	return netip.AddrFrom16([16]byte{
		padded[0], padded[1], padded[2], padded[3],
		padded[4], padded[5], padded[6], padded[7],
		padded[8], padded[9], padded[10], padded[11],
		padded[12], padded[13], padded[14], padded[15],
	})
}

func prefixStringForToken(prefix netip.Prefix, original string) string {
	prefix = prefix.Masked()
	if !strings.Contains(original, "/") {
		return prefix.Addr().String()
	}
	return prefix.String()
}
