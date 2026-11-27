package colodict

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"os"
	"strings"
)

func ParseGeofeed(raw []byte) ([]GeofeedEntry, int, error) {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	entries := make([]GeofeedEntry, 0)
	invalid := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return entries, invalid, err
		}
		if len(record) == 0 {
			continue
		}
		first := strings.TrimSpace(record[0])
		if first == "" || strings.HasPrefix(first, "#") {
			continue
		}
		if strings.EqualFold(first, "ip_prefix") {
			continue
		}
		if len(record) < 4 {
			invalid++
			continue
		}
		prefix, err := netip.ParsePrefix(first)
		if err != nil {
			invalid++
			continue
		}
		entry := GeofeedEntry{
			Prefix:  prefix.Masked(),
			Country: strings.TrimSpace(record[1]),
			Region:  strings.TrimSpace(record[2]),
			City:    strings.TrimSpace(record[3]),
		}
		if len(record) > 4 {
			entry.Postal = strings.TrimSpace(record[4])
		}
		entries = append(entries, entry)
	}
	return entries, invalid, nil
}

func ParseLocations(raw []byte) ([]LocationEntry, error) {
	var records []struct {
		CCA2   string `json:"cca2"`
		City   string `json:"city"`
		IATA   string `json:"iata"`
		Region string `json:"region"`
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, err
	}
	entries := make([]LocationEntry, 0, len(records))
	for _, record := range records {
		colo := normalizeColo(record.IATA)
		country := strings.ToUpper(strings.TrimSpace(record.CCA2))
		city := strings.TrimSpace(record.City)
		if colo == "" || country == "" || city == "" {
			continue
		}
		entries = append(entries, LocationEntry{
			Country: country,
			City:    city,
			Colo:    colo,
			Region:  strings.TrimSpace(record.Region),
		})
	}
	if len(entries) == 0 {
		return nil, errors.New("COLO locations 未包含可用记录")
	}
	return entries, nil
}

func ParseCountries(raw []byte) (map[string]string, error) {
	var countries map[string]string
	if err := json.Unmarshal(raw, &countries); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(countries))
	for code, name := range countries {
		code = strings.ToUpper(strings.TrimSpace(code))
		name = strings.TrimSpace(name)
		if len(code) != 2 || name == "" {
			continue
		}
		result[code] = name
	}
	if len(result) == 0 {
		return nil, errors.New("country 数据未包含可用记录")
	}
	return result, nil
}

func EncodeColoEntries(entries []ColoEntry) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{"ip_prefix", "colo", "country", "region", "city"}); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if err := writer.Write([]string{
			entry.Prefix.String(),
			entry.Colo,
			entry.Country,
			entry.Region,
			entry.City,
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func LoadColoEntries(path string) ([]ColoEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	entries := make([]ColoEntry, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return entries, err
		}
		if len(record) == 0 {
			continue
		}
		first := strings.TrimSpace(record[0])
		if first == "" || strings.HasPrefix(first, "#") || strings.EqualFold(first, "ip_prefix") {
			continue
		}
		if len(record) < 2 {
			continue
		}
		prefix, err := netip.ParsePrefix(first)
		if err != nil {
			continue
		}
		colo := normalizeColo(strings.TrimSpace(record[1]))
		if colo == "" {
			continue
		}
		entry := ColoEntry{
			Prefix: prefix.Masked(),
			Colo:   colo,
		}
		if len(record) > 2 {
			entry.Country = strings.TrimSpace(record[2])
		}
		if len(record) > 3 {
			entry.Region = strings.TrimSpace(record[3])
		}
		if len(record) > 4 {
			entry.City = strings.TrimSpace(record[4])
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func loadOptionalColoRowCount(path string) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, nil
	}
	rows, err := LoadColoEntries(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return len(rows), nil
}

func splitColoEntriesByAddressFamily(entries []ColoEntry) ([]ColoEntry, []ColoEntry) {
	ipv4Entries := make([]ColoEntry, 0, len(entries))
	ipv6Entries := make([]ColoEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Prefix.Addr().Is4() {
			ipv4Entries = append(ipv4Entries, entry)
			continue
		}
		ipv6Entries = append(ipv6Entries, entry)
	}
	return ipv4Entries, ipv6Entries
}

func writeColoEntries(path string, entries []ColoEntry) error {
	raw, err := EncodeColoEntries(entries)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, raw, 0o600)
}
