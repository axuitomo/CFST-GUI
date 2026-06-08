package appcore

import (
	"fmt"
	"sort"
	"strings"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

func ResolveConfiguredColos(paths colodict.Paths, raw string, label string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	resolved, unmatched, err := colodict.ResolveTokensToColos(paths, raw)
	if err != nil {
		return nil, fmt.Errorf("%s需要先更新/处理 COLO 词典：%w", label, err)
	}
	if len(unmatched) > 0 {
		sort.Strings(unmatched)
		return nil, fmt.Errorf("%s包含未匹配的国家/COLO 筛选词：%s", label, strings.Join(unmatched, ", "))
	}
	result := make([]string, 0, len(resolved))
	for code := range resolved {
		result = append(result, code)
	}
	sort.Strings(result)
	return result, nil
}
