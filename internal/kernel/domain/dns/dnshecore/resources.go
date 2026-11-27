package dnshecore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// ListSubdomains reads every subdomain available to the API key.
func (c *Client) ListSubdomains(ctx context.Context) ([]Subdomain, error) {
	result := make([]Subdomain, 0)
	for page := 1; page <= 100; page++ {
		query := url.Values{}
		query.Set("m", moduleName)
		query.Set("endpoint", "subdomains")
		query.Set("action", "list")
		query.Set("page", strconv.Itoa(page))
		query.Set("per_page", "500")
		var response subdomainsResponse
		if err := c.do(ctx, http.MethodGet, query, nil, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Subdomains {
			result = append(result, Subdomain{
				ID:         item.ID,
				FullDomain: strings.TrimSpace(item.FullDomain),
				RootDomain: strings.TrimSpace(item.RootDomain),
				Status:     strings.TrimSpace(item.Status),
				Subdomain:  strings.TrimSpace(item.Subdomain),
			})
		}
		if !response.Pagination.HasMore {
			break
		}
	}
	return result, nil
}

// GetSubdomain reads one subdomain by its module ID.
func (c *Client) GetSubdomain(ctx context.Context, id int) (Subdomain, error) {
	if id <= 0 {
		return Subdomain{}, errors.New("缺少 DNSHE 子域 ID")
	}
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "subdomains")
	query.Set("action", "get")
	query.Set("subdomain_id", strconv.Itoa(id))
	var response subdomainResponse
	if err := c.do(ctx, http.MethodGet, query, nil, &response); err != nil {
		return Subdomain{}, err
	}
	item := response.Subdomain
	if item.ID == 0 {
		return Subdomain{}, fmt.Errorf("DNSHE 未返回子域 %d 的信息", id)
	}
	return Subdomain{
		ID:         item.ID,
		FullDomain: strings.TrimSpace(item.FullDomain),
		RootDomain: strings.TrimSpace(item.RootDomain),
		Status:     strings.TrimSpace(item.Status),
		Subdomain:  strings.TrimSpace(item.Subdomain),
	}, nil
}

// ResolveSubdomain finds the subdomain that owns fullDomain, preferring the
// longest matching suffix so nested records still resolve to their zone.
func (c *Client) ResolveSubdomain(ctx context.Context, fullDomain string) (Subdomain, error) {
	fullDomain = normalizeDomain(fullDomain)
	if fullDomain == "" {
		return Subdomain{}, errors.New("缺少 DNSHE 域名")
	}
	items, err := c.ListSubdomains(ctx)
	if err != nil {
		return Subdomain{}, err
	}
	best := Subdomain{}
	for _, item := range items {
		item.FullDomain = normalizeDomain(item.FullDomain)
		if item.FullDomain == "" {
			continue
		}
		if item.FullDomain != fullDomain && !strings.HasSuffix(fullDomain, "."+item.FullDomain) {
			continue
		}
		if len(item.FullDomain) > len(best.FullDomain) {
			best = item
		}
	}
	if best.ID == 0 {
		return Subdomain{}, fmt.Errorf("DNSHE 账号下找不到与 %s 匹配的子域名，请先在自由域名管理中注册", fullDomain)
	}
	return best, nil
}

// ResolveSubdomainByName finds a subdomain by its label (for example "test")
// or its full domain (for example "test.example.com"). A bare label that
// exists under several roots is reported as ambiguous.
func (c *Client) ResolveSubdomainByName(ctx context.Context, value string) (Subdomain, error) {
	normalized := normalizeDomain(value)
	if normalized == "" {
		return Subdomain{}, errors.New("缺少 DNSHE 子域名")
	}
	items, err := c.ListSubdomains(ctx)
	if err != nil {
		return Subdomain{}, err
	}
	matches := make([]Subdomain, 0, 2)
	for _, item := range items {
		fullDomain := normalizeDomain(item.FullDomain)
		if fullDomain == "" {
			fullDomain = normalizeDomain(item.Subdomain + "." + item.RootDomain)
		}
		if fullDomain == "" || item.ID == 0 {
			continue
		}
		item.FullDomain = fullDomain
		if fullDomain == normalized || strings.EqualFold(strings.TrimSpace(item.Subdomain), normalized) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return Subdomain{}, fmt.Errorf("%w %s", errSubdomainNotFound, value)
	default:
		fullDomains := make([]string, 0, len(matches))
		for _, item := range matches {
			fullDomains = append(fullDomains, item.FullDomain)
		}
		slices.Sort(fullDomains)
		return Subdomain{}, fmt.Errorf("DNSHE 子域名 %s 对应多个完整域名，请改为填写完整域名：%s", value, strings.Join(fullDomains, ", "))
	}
}

// ResolveSubdomainTarget resolves the zone that owns recordName and returns the
// record name to write. A bare label that is not a registered subdomain is read
// as a host label under recordName: "test" with "axuitomo.l.cd" targets
// "test.axuitomo.l.cd". A full domain or an exact subdomain label keeps the
// zone-selector meaning so existing configurations stay valid.
func (c *Client) ResolveSubdomainTarget(ctx context.Context, subdomainValue, recordName string) (Subdomain, string, error) {
	value := normalizeDomain(subdomainValue)
	base := normalizeDomain(recordName)
	if value == "" {
		return Subdomain{}, "", errors.New("缺少 DNSHE 子域名")
	}
	subdomain, err := c.ResolveSubdomainByName(ctx, value)
	if err == nil {
		return subdomain, base, nil
	}
	// Only a bare label may act as a host label. A full domain miss stays an
	// error so a typo can never silently retarget another zone.
	if !errors.Is(err, errSubdomainNotFound) || strings.Contains(value, ".") || base == "" {
		return Subdomain{}, "", err
	}
	target := base
	if !strings.HasPrefix(base, value+".") {
		target = value + "." + base
	}
	subdomain, resolveErr := c.ResolveSubdomain(ctx, target)
	if resolveErr != nil {
		return Subdomain{}, "", fmt.Errorf("%w；按完整域名 %s 解析时同样失败：%v", err, target, resolveErr)
	}
	return subdomain, target, nil
}

func (c *Client) ListRecords(ctx context.Context, subdomainID int) ([]Record, error) {
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "dns_records")
	query.Set("action", "list")
	query.Set("subdomain_id", strconv.Itoa(subdomainID))
	var response recordsResponse
	if err := c.do(ctx, http.MethodGet, query, nil, &response); err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(response.Records))
	for _, item := range response.Records {
		records = append(records, fromAPIRecord(item))
	}
	return records, nil
}

func (c *Client) CreateRecord(ctx context.Context, subdomainID int, record Record) (Record, error) {
	body := map[string]any{
		"subdomain_id": subdomainID,
		"content":      record.Content,
		"name":         record.Name,
		"ttl":          record.TTL,
		"type":         record.Type,
	}
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "dns_records")
	query.Set("action", "create")
	var response recordResponse
	if err := c.do(ctx, http.MethodPost, query, body, &response); err != nil {
		return Record{}, err
	}
	created := fromAPIRecord(response.Record)
	if created.ID == "" {
		if response.ID > 0 {
			created.ID = strconv.Itoa(response.ID)
		}
		created.ProviderRecordID = strings.TrimSpace(response.RecordID)
	}
	return created, nil
}

func (c *Client) UpdateRecord(ctx context.Context, record Record) (Record, error) {
	if strings.TrimSpace(record.ID) == "" && strings.TrimSpace(record.ProviderRecordID) == "" {
		return Record{}, errors.New("缺少 DNSHE 记录 ID")
	}
	body := map[string]any{
		"name":    record.Name,
		"ttl":     record.TTL,
		"type":    record.Type,
		"content": record.Content,
	}
	if record.ID != "" {
		if id, err := strconv.Atoi(record.ID); err == nil {
			body["id"] = id
		} else {
			body["record_id"] = record.ID
		}
	} else {
		body["record_id"] = record.ProviderRecordID
	}
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "dns_records")
	query.Set("action", "update")
	var response recordResponse
	if err := c.do(ctx, http.MethodPost, query, body, &response); err != nil {
		return Record{}, err
	}
	updated := fromAPIRecord(response.Record)
	if updated.ID == "" {
		updated = record
	}
	return updated, nil
}

func (c *Client) DeleteRecord(ctx context.Context, record Record) error {
	body := map[string]any{}
	if record.ID != "" {
		if id, err := strconv.Atoi(record.ID); err == nil {
			body["id"] = id
		} else {
			body["record_id"] = record.ID
		}
	} else if record.ProviderRecordID != "" {
		body["record_id"] = record.ProviderRecordID
	} else {
		return errors.New("缺少 DNSHE 记录 ID")
	}
	query := url.Values{}
	query.Set("m", moduleName)
	query.Set("endpoint", "dns_records")
	query.Set("action", "delete")
	var response recordResponse
	return c.do(ctx, http.MethodPost, query, body, &response)
}
