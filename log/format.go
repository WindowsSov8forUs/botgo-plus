package log

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var logURL = regexp.MustCompile(`(?i)(?:https?|wss?)://[^\s<>"']+|(?:data:|internal:)[^\s<>"']+`)
var logCredential = regexp.MustCompile(`(?i)((?:access_token|app_secret|authorization|client_secret|refresh_token|token|secret)["']?\s*[=:]\s*)(?:"[^"]*"|'[^']*'|(?:Bearer|QQBot|Bot)\s+[^\s,;}]+|[^\s,;}]+)`)

// SafeText escapes control characters in diagnostic values without treating them as format strings.
func SafeText(value string) string {
	value = logURL.ReplaceAllStringFunc(value, SafeURL)
	value = logCredential.ReplaceAllString(value, "${1}[redacted]")
	var out strings.Builder
	for _, r := range value {
		if r < 32 || r == 127 || r == '\u2028' || r == '\u2029' {
			quoted := strconv.QuoteRune(r)
			out.WriteString(quoted[1 : len(quoted)-1])
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// SafeURL keeps only the origin; paths and query strings may contain credentials.
func SafeURL(value string) string {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" {
		return "[resource address omitted]"
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "ws", "wss":
		return u.Scheme + "://" + u.Host
	default:
		return "[resource address omitted]"
	}
}

// SafeError does not change the error returned to the caller.
func SafeError(err error) string {
	if err == nil {
		return ""
	}
	var requestError *url.Error
	if errors.As(err, &requestError) && requestError != nil {
		reason := "no error details were provided"
		if requestError.Err != nil {
			reason = SafeText(requestError.Err.Error())
		}
		return SafeText(requestError.Op) + " " + SafeURL(requestError.URL) + ": " + reason
	}
	return SafeText(err.Error())
}
