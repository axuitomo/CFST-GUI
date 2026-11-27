package task

import (
	"net"
	"os"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/colodict"
)

type SourceColoFilter struct {
	MatchNone    bool
	Mode         string
	Unrestricted bool
	Allowed      map[string]struct{}
	Denied       map[string]struct{}
}

type SourceColoFilterMap map[string]SourceColoFilter

func NewSourceColoFilter(allowRaw string) SourceColoFilter {
	return NewSourceColoFilterWithMode(allowRaw, ColoFilterModeAllow)
}

func NewSourceColoFilterWithMode(raw string, mode string) SourceColoFilter {
	codes := ParseColoAllowList(raw)
	return NewSourceColoFilterForColos(codes, mode, strings.TrimSpace(raw) != "")
}

func NewSourceColoFilterForColos(codes []string, mode string, active bool) SourceColoFilter {
	mode = NormalizeColoFilterMode(mode)
	if len(codes) == 0 {
		if active && mode != ColoFilterModeDeny {
			return SourceColoFilter{MatchNone: true, Mode: ColoFilterModeAllow}
		}
		return SourceColoFilter{Unrestricted: true}
	}
	values := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		normalized := normalizeColoCode(code)
		if normalized != "" {
			values[normalized] = struct{}{}
		}
	}
	if len(values) == 0 {
		if active && mode != ColoFilterModeDeny {
			return SourceColoFilter{MatchNone: true, Mode: ColoFilterModeAllow}
		}
		return SourceColoFilter{Unrestricted: true}
	}
	if mode == ColoFilterModeDeny {
		return SourceColoFilter{Mode: mode, Denied: values}
	}
	return SourceColoFilter{Mode: ColoFilterModeAllow, Allowed: values}
}

func MergeSourceColoFiltersWithResolvedColos(target SourceColoFilterMap, ips []string, colos []string, mode string, active bool) {
	if target == nil || len(ips) == 0 {
		return
	}
	incoming := NewSourceColoFilterForColos(colos, mode, active)
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		existing := target[ip]
		existingEmpty := !existing.MatchNone && !existing.Unrestricted && existing.Mode == "" && len(existing.Allowed) == 0 && len(existing.Denied) == 0
		if existing.Unrestricted || incoming.Unrestricted {
			target[ip] = SourceColoFilter{Unrestricted: true}
			continue
		}
		if existing.Mode == "" {
			existing.Mode = incoming.Mode
		}
		if existing.Mode != incoming.Mode {
			target[ip] = SourceColoFilter{Unrestricted: true}
			continue
		}
		if existing.MatchNone {
			target[ip] = incoming
			continue
		}
		if incoming.MatchNone {
			if existingEmpty {
				target[ip] = incoming
			}
			continue
		}
		if incoming.Mode == ColoFilterModeDeny {
			if existing.Denied == nil {
				existing.Denied = make(map[string]struct{})
			}
			for code := range incoming.Denied {
				existing.Denied[code] = struct{}{}
			}
		} else {
			if existing.Allowed == nil {
				existing.Allowed = make(map[string]struct{})
			}
			for code := range incoming.Allowed {
				existing.Allowed[code] = struct{}{}
			}
		}
		target[ip] = existing
	}
}

func CloneSourceColoFilterMap(source SourceColoFilterMap) SourceColoFilterMap {
	if len(source) == 0 {
		return nil
	}
	cloned := make(SourceColoFilterMap, len(source))
	for ip, filter := range source {
		next := SourceColoFilter{MatchNone: filter.MatchNone, Mode: NormalizeColoFilterMode(filter.Mode), Unrestricted: filter.Unrestricted}
		if len(filter.Allowed) > 0 {
			next.Allowed = make(map[string]struct{}, len(filter.Allowed))
			for code := range filter.Allowed {
				next.Allowed[code] = struct{}{}
			}
		}
		if len(filter.Denied) > 0 {
			next.Denied = make(map[string]struct{}, len(filter.Denied))
			for code := range filter.Denied {
				next.Denied[code] = struct{}{}
			}
		}
		cloned[ip] = next
	}
	return cloned
}

func (e *Engine) sourceAllowsColo(ip *net.IPAddr, colo string) bool {
	filter, ok := e.config.SourceColoFilters[ip.String()]
	if !ok || filter.Unrestricted {
		return true
	}
	if filter.MatchNone {
		return false
	}
	mode := NormalizeColoFilterMode(filter.Mode)
	colo = normalizeColoCode(colo)
	if colo == "" {
		return mode == ColoFilterModeDeny || len(filter.Allowed) == 0
	}
	if mode == ColoFilterModeDeny {
		if len(filter.Denied) == 0 {
			return true
		}
		_, ok = filter.Denied[colo]
		return !ok
	}
	if len(filter.Allowed) == 0 {
		return true
	}
	_, ok = filter.Allowed[colo]
	return ok
}

func (e *Engine) sourceRequiresColo(ip *net.IPAddr) bool {
	filter, ok := e.config.SourceColoFilters[ip.String()]
	if !ok || filter.Unrestricted {
		return false
	}
	if filter.MatchNone {
		return true
	}
	if NormalizeColoFilterMode(filter.Mode) == ColoFilterModeDeny {
		return false
	}
	return len(filter.Allowed) > 0
}

func (e *Engine) lookupColoFromDictionary(ip *net.IPAddr) string {
	path := strings.TrimSpace(e.config.ColoDictionaryPath)
	if path == "" {
		return ""
	}
	lookup, err := e.cachedColoDictionaryLookup(path)
	if err != nil {
		e.debugEvent("stage.detail", map[string]any{
			"error":   err.Error(),
			"ip":      ip.String(),
			"message": "COLO 词典兜底不可用。",
			"reason":  "colo_dictionary_unavailable",
			"stage":   "stage2_trace",
			"trace": map[string]any{
				"colo_dictionary_path": path,
			},
		})
		return ""
	}
	return normalizeColoCode(lookup.Lookup(ip.String()))
}

// coloDictionaryRecheckInterval bounds how often the dictionary file is stat'ed.
// The lookup runs once per traced IP, so a stat per IP is pure syscall noise; a
// dictionary replaced on disk is still picked up within this window.
const coloDictionaryRecheckInterval = 5 * time.Second

// cachedColoDictionaryLookup keeps the parsed dictionary and its prefix index
// in memory so each traced IP only pays a bucket lookup, not a full scan.
func (e *Engine) cachedColoDictionaryLookup(path string) (*colodict.ColoLookup, error) {
	now := time.Now()
	e.coloCache.Lock()
	if e.coloCache.path == path && e.coloCache.lookup != nil && now.Sub(e.coloCache.checkedAt) < coloDictionaryRecheckInterval {
		lookup := e.coloCache.lookup
		e.coloCache.Unlock()
		return lookup, nil
	}
	e.coloCache.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	e.coloCache.Lock()
	if e.coloCache.path == path &&
		e.coloCache.size == info.Size() &&
		e.coloCache.modTime.Equal(info.ModTime()) &&
		e.coloCache.lookup != nil {
		e.coloCache.checkedAt = now
		lookup := e.coloCache.lookup
		e.coloCache.Unlock()
		return lookup, nil
	}
	e.coloCache.Unlock()

	entries, err := colodict.LoadColoEntries(path)
	if err != nil {
		return nil, err
	}
	lookup := colodict.NewColoLookup(entries)
	e.coloCache.Lock()
	e.coloCache.path = path
	e.coloCache.size = info.Size()
	e.coloCache.modTime = info.ModTime()
	e.coloCache.checkedAt = now
	e.coloCache.lookup = lookup
	e.coloCache.Unlock()
	return lookup, nil
}
