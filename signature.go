package unionewrapper

import (
	"bytes"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"regexp"
	"strings"
)

var authField = regexp.MustCompile(`"auth"\s*:\s*"([0-9a-fA-F]{32})"`)

// VerifySignature checks the "auth" field of a webhook notification: the MD5 of the request body in which
// the "auth" value is replaced by the API key of the user/project of the webhook.
func VerifySignature(body []byte, apiKey string) bool {
	m := authField.FindSubmatch(body)
	if apiKey == "" || m == nil {
		return false
	}
	sum := md5.Sum(bytes.ReplaceAll(body, m[1], []byte(apiKey)))
	expected := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(string(m[1]))), []byte(expected)) == 1
}

// SignaturePlaceholder is the "auth" value Sign replaces.
const SignaturePlaceholder = "00000000000000000000000000000000"

// Sign signs a body whose "auth" is SignaturePlaceholder, e.g. for tests.
func Sign(body []byte, apiKey string) []byte {
	sum := md5.Sum(bytes.ReplaceAll(body, []byte(SignaturePlaceholder), []byte(apiKey)))
	return bytes.ReplaceAll(body, []byte(SignaturePlaceholder), []byte(hex.EncodeToString(sum[:])))
}
