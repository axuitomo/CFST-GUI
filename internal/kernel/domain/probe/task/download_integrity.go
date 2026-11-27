package task

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"net/http"
	"strings"
)

type downloadIntegrity struct {
	expectedLength int64
	headerFailure  string
	contentFailure string
	contentPrefix  []byte
	read           int64
	hashes         []downloadHashCheck
}

type downloadHashCheck struct {
	name     string
	hash     hash.Hash
	expected []byte
}

func newDownloadIntegrity(response *http.Response, expectedLength int64) *downloadIntegrity {
	if expectedLength < 0 && response != nil && response.ContentLength >= 0 {
		expectedLength = response.ContentLength
	}
	integrity := &downloadIntegrity{
		expectedLength: expectedLength,
	}
	if response == nil {
		return integrity
	}
	integrity.contentFailure = unexpectedDownloadContentReason(response.Header.Get("Content-Type"), nil)
	if response.ContentLength >= 0 && expectedLength >= 0 && response.ContentLength != expectedLength {
		integrity.headerFailure = "content_length_mismatch"
	}
	integrity.hashes = downloadHashChecks(response.Header)
	return integrity
}

func (v *downloadIntegrity) write(data []byte) {
	if v == nil || len(data) == 0 {
		return
	}
	v.read += int64(len(data))
	if v.contentFailure == "" && len(v.contentPrefix) < downloadContentSniffBytes {
		remaining := downloadContentSniffBytes - len(v.contentPrefix)
		if remaining > len(data) {
			remaining = len(data)
		}
		v.contentPrefix = append(v.contentPrefix, data[:remaining]...)
		v.contentFailure = unexpectedDownloadContentReason("", v.contentPrefix)
	}
	for index := range v.hashes {
		_, _ = v.hashes[index].hash.Write(data)
	}
}

func (v *downloadIntegrity) validate() string {
	if v == nil {
		return ""
	}
	if v.headerFailure != "" {
		return v.headerFailure
	}
	if v.contentFailure != "" {
		return v.contentFailure
	}
	if v.expectedLength >= 0 && v.read != v.expectedLength {
		return "content_length_mismatch"
	}
	for _, check := range v.hashes {
		if !bytes.Equal(check.hash.Sum(nil), check.expected) {
			return check.name + "_mismatch"
		}
	}
	return ""
}

func (v *downloadIntegrity) streamFailureReason() string {
	if v == nil {
		return ""
	}
	return v.contentFailure
}

func integrityFailureReason(integrity *downloadIntegrity) string {
	if integrity == nil {
		return ""
	}
	return integrity.validate()
}

func isDownloadIntegrityFailure(reason string) bool {
	return reason == "unexpected_html_response" || reason == "content_length_mismatch" || strings.HasSuffix(reason, "_mismatch")
}

func downloadHashChecks(header http.Header) []downloadHashCheck {
	if header == nil {
		return nil
	}
	checks := make([]downloadHashCheck, 0, 2)
	checks = appendHashCheck(checks, "content_md5", "md5", header.Get("Content-MD5"))
	checks = appendHashCheck(checks, "x_checksum_md5", "md5", header.Get("X-Checksum-MD5"))
	for _, headerName := range []string{
		"X-Checksum-SHA256",
		"X-Checksum-Sha256",
		"X-Content-SHA256",
		"X-Content-Sha256",
		"X-Amz-Content-Sha256",
	} {
		checks = appendHashCheck(checks, strings.ToLower(strings.ReplaceAll(headerName, "-", "_")), "sha256", header.Get(headerName))
	}
	for _, value := range header.Values("Digest") {
		checks = appendDigestHashChecks(checks, value)
	}
	return checks
}

func appendDigestHashChecks(checks []downloadHashCheck, value string) []downloadHashCheck {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, encoded, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		algorithm := normalizeHashAlgorithm(name)
		if algorithm == "" {
			continue
		}
		checkName := "digest_" + strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "-", "_")
		checks = appendHashCheck(checks, checkName, algorithm, encoded)
	}
	return checks
}

func appendHashCheck(checks []downloadHashCheck, name, algorithm, encoded string) []downloadHashCheck {
	hasher, size := newDownloadHasher(algorithm)
	if hasher == nil {
		return checks
	}
	expected, ok := decodeHashValue(encoded, size)
	if !ok {
		return checks
	}
	return append(checks, downloadHashCheck{
		name:     name,
		hash:     hasher,
		expected: expected,
	})
}

func newDownloadHasher(algorithm string) (hash.Hash, int) {
	switch normalizeHashAlgorithm(algorithm) {
	case "md5":
		return md5.New(), md5.Size
	case "sha256":
		return sha256.New(), sha256.Size
	default:
		return nil, 0
	}
}

func normalizeHashAlgorithm(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	switch normalized {
	case "md5", "contentmd5":
		return "md5"
	case "sha256", "sha2256":
		return "sha256"
	default:
		return ""
	}
}

func decodeHashValue(value string, expectedSize int) ([]byte, bool) {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	if strings.HasPrefix(value, ":") && strings.HasSuffix(value, ":") && len(value) >= 2 {
		value = strings.TrimPrefix(strings.TrimSuffix(value, ":"), ":")
	}
	if value == "" || strings.EqualFold(value, "UNSIGNED-PAYLOAD") {
		return nil, false
	}
	for _, decoder := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := decoder.DecodeString(value); err == nil && len(decoded) == expectedSize {
			return decoded, true
		}
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == expectedSize {
		return decoded, true
	}
	return nil, false
}
