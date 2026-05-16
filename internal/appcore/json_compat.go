package appcore

import (
	"bytes"
	"encoding/json"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type JSONCompatInfo struct {
	IgnoredTrailingContent bool
	TrimmedUTF8BOM         bool
}

func UnmarshalJSONCompat(raw []byte, target any) (JSONCompatInfo, error) {
	info := JSONCompatInfo{}
	normalized := bytes.TrimSpace(raw)
	if bytes.HasPrefix(normalized, utf8BOM) {
		info.TrimmedUTF8BOM = true
		normalized = bytes.TrimSpace(bytes.TrimPrefix(normalized, utf8BOM))
	}
	if err := json.Unmarshal(normalized, target); err == nil {
		return info, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(normalized))
	if err := decoder.Decode(target); err != nil {
		return info, err
	}
	if trailing := bytes.TrimSpace(normalized[decoder.InputOffset():]); len(trailing) > 0 {
		info.IgnoredTrailingContent = true
	}
	return info, nil
}
