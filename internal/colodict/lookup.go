package colodict

import (
	"net/netip"
	"strings"
)

// ColoLookup answers prefix lookups in time proportional to the dictionary
// size divided by 256 instead of scanning every entry per IP.
//
// Entries are bucketed by address family and the first address byte. A prefix
// shorter than eight bits belongs to every bucket it covers, and each bucket
// keeps the dictionary order, so a lookup returns the same entry the linear
// scan in LookupColo would return.
type ColoLookup struct {
	v4 [256][]ColoEntry
	v6 [256][]ColoEntry
}

func NewColoLookup(entries []ColoEntry) *ColoLookup {
	lookup := &ColoLookup{}
	for _, entry := range entries {
		lookup.add(entry)
	}
	return lookup
}

func (lookup *ColoLookup) add(entry ColoEntry) {
	prefix := entry.Prefix
	if !prefix.IsValid() {
		return
	}
	addr := prefix.Addr()
	bits := prefix.Bits()
	if bits < 0 {
		return
	}
	buckets := &lookup.v6
	if addr.Is4() {
		buckets = &lookup.v4
	}
	first := firstAddressByte(addr)
	if bits >= 8 {
		buckets[first] = append(buckets[first], entry)
		return
	}
	mask := byte(0xFF) << uint(8-bits)
	for index := 0; index < 256; index++ {
		if byte(index)&mask == first&mask {
			buckets[index] = append(buckets[index], entry)
		}
	}
}

// Lookup returns the colo of the first dictionary entry that contains rawIP,
// or an empty string when nothing matches.
func (lookup *ColoLookup) Lookup(rawIP string) string {
	if lookup == nil {
		return ""
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(rawIP))
	if err != nil {
		return ""
	}
	if !addr.Is4() {
		first := firstAddressByte(addr)
		return lookup.firstMatching(lookup.v6[first], addr)
	}
	first := firstAddressByte(addr)
	return lookup.firstMatching(lookup.v4[first], addr)
}

func (lookup *ColoLookup) firstMatching(entries []ColoEntry, addr netip.Addr) string {
	for _, entry := range entries {
		if entry.Prefix.Contains(addr) {
			return entry.Colo
		}
	}
	return ""
}

func firstAddressByte(addr netip.Addr) byte {
	if addr.Is4() {
		octets := addr.As4()
		return octets[0]
	}
	octets := addr.As16()
	return octets[0]
}
