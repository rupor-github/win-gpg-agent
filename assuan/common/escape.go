package common

import (
	"net/url"
	"strings"
)

var escaper = strings.NewReplacer("\r", "%0D", "\n", "%0A", "%", "%25", "\\", "%5C")

// EscapeParameters percent-encodes CR, LF, % and backslash at end as required by protocol.
// Note: We escape all backslashes to keep code simple.
// Ref.: https//www.gnupg.org/documentation/manuals/assuan/Client-requests.html.
func EscapeParameters(raw string) string {
	return escaper.Replace(raw)
}

// UnescapeParameters reverses EscapeParameters function.
// Note: It does unescape any escaped character, not only CR, LF, % and backslashes.
// Ref.: https//www.gnupg.org/documentation/manuals/assuan/Client-requests.html.
func UnescapeParameters(encoded string) (string, error) {
	// Percent-encoding used in Assuan is same as percent-encoding used in path part of URL.
	return url.PathUnescape(encoded)
}
