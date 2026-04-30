package cidr

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	mrand "math/rand"
	"net/netip"
	"os"
	"strings"
)

func ReadCIDRsFromFile(path string) ([]netip.Prefix, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ReadCIDRs(f)
}

func ReadCIDRs(r io.Reader) ([]netip.Prefix, error) {
	var out []netip.Prefix
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// allow inline comments after space-#
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		p, err := netip.ParsePrefix(line)
		if err != nil {
			return nil, fmt.Errorf("parse cidr %q: %w", line, err)
		}
		out = append(out, p.Masked())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ParseCIDRs(strs []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(strs))
	for _, s := range strs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, fmt.Errorf("parse cidr %q: %w", s, err)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// SplitPrefix splits a prefix into sub-prefixes by increasing the prefix length by step.
// For example, IPv4 /16 with step=2 yields 4 sub-prefixes of /18.
func SplitPrefix(p netip.Prefix, step int) ([]netip.Prefix, error) {
	p = p.Masked()
	if step <= 0 {
		return nil, fmt.Errorf("invalid step: %d", step)
	}
	newBits := p.Bits() + step
	maxBits := 128
	if p.Addr().Is4() {
		maxBits = 32
	}
	if newBits > maxBits {
		return nil, fmt.Errorf("cannot split %s by step %d", p.String(), step)
	}

	parts := 1 << step
	out := make([]netip.Prefix, 0, parts)
	base := p.Addr()

	for i := 0; i < parts; i++ {
		childAddr := childPrefixAddr(base, p.Bits(), step, i)
		out = append(out, netip.PrefixFrom(childAddr, newBits).Masked())
	}
	return out, nil
}

// RandomAddr returns a uniformly random address inside prefix p.
// It uses math/rand for speed; caller controls seed.
func RandomAddr(p netip.Prefix, r *mrand.Rand) netip.Addr {
	p = p.Masked()
	if p.Addr().Is4() {
		return randomAddr4(p, r)
	}
	return randomAddr6(p, r)
}

func randomAddr4(p netip.Prefix, r *mrand.Rand) netip.Addr {
	a := p.Addr().As4()
	base := binary.BigEndian.Uint32(a[:])
	hostBits := 32 - p.Bits()
	var host uint32
	if hostBits == 0 {
		host = 0
	} else {
		host = uint32(r.Uint64() & ((uint64(1) << hostBits) - 1))
	}
	ip := base | host
	var out [4]byte
	binary.BigEndian.PutUint32(out[:], ip)
	return netip.AddrFrom4(out)
}

func randomAddr6(p netip.Prefix, r *mrand.Rand) netip.Addr {
	a := p.Addr().As16()
	base := a[:]
	hostBits := 128 - p.Bits()
	if hostBits <= 0 {
		return netip.AddrFrom16(a)
	}

	// Fill a random 128-bit value, then keep only host bits and OR into base.
	var rand128 [16]byte
	// Use math/rand to keep sampling reproducible under --seed (important for A/B tuning).
	u0 := r.Uint64()
	u1 := r.Uint64()
	binary.BigEndian.PutUint64(rand128[0:8], u0)
	binary.BigEndian.PutUint64(rand128[8:16], u1)

	keepHostBits(&rand128, hostBits)
	applyHostBits(base, rand128)
	var out [16]byte
	copy(out[:], base)
	return netip.AddrFrom16(out)
}

// keepHostBits zeroes the top (128-hostBits) bits.
func keepHostBits(b *[16]byte, hostBits int) {
	topBits := 128 - hostBits
	if topBits <= 0 {
		return
	}
	fullBytes := topBits / 8
	remBits := topBits % 8
	for i := 0; i < fullBytes; i++ {
		b[i] = 0
	}
	if remBits > 0 && fullBytes < 16 {
		mask := byte(0xFF >> remBits)
		b[fullBytes] &= mask
	}
}

// applyHostBits ORs random host bits into base (which already has network bits fixed).
func applyHostBits(base []byte, host [16]byte) {
	for i := 0; i < 16; i++ {
		base[i] |= host[i]
	}
}

// childPrefixAddr computes the i-th child prefix address when splitting.
// It assumes base is already masked to parentBits so those step bits are 0 and can be ORed.
func childPrefixAddr(base netip.Addr, parentBits int, step int, index int) netip.Addr {
	newBits := parentBits + step
	if base.Is4() {
		a := base.As4()
		v := binary.BigEndian.Uint32(a[:])
		shift := 32 - newBits
		v += uint32(index) << shift
		var out [4]byte
		binary.BigEndian.PutUint32(out[:], v)
		return netip.AddrFrom4(out)
	}
	a := base.As16()
	var out [16]byte
	copy(out[:], a[:])
	shift := 128 - newBits
	orShiftedIndex(&out, uint64(index), shift, step)
	return netip.AddrFrom16(out)
}

func orShiftedIndex(dst *[16]byte, index uint64, shiftFromLSB int, bits int) {
	// Set up to 'bits' bits from index into dst at bit positions [shiftFromLSB, shiftFromLSB+bits).
	for b := 0; b < bits; b++ {
		if (index>>uint(b))&1 == 1 {
			setBitFromLSB(dst, shiftFromLSB+b)
		}
	}
}

// setBitFromLSB sets the bit at position posFromLSB (0 = least significant bit) in big-endian 128-bit buffer.
func setBitFromLSB(dst *[16]byte, posFromLSB int) {
	if posFromLSB < 0 || posFromLSB >= 128 {
		return
	}
	posFromMSB := 127 - posFromLSB
	byteIdx := posFromMSB / 8
	bitInByte := 7 - (posFromMSB % 8)
	dst[byteIdx] |= 1 << uint(bitInByte)
}
