package importblogir

import "regexp"

// protocolRelativeURL matches src="//host/..." and href="//host/..." attributes.
// blog.ir posts link images and files as protocol-relative URLs (e.g.
// bayanbox.ir), but waldi's HTML sanitizer only allows explicit http(s)
// schemes, so these must be rewritten before rendering or they get stripped.
var protocolRelativeURL = regexp.MustCompile(`((?:src|href)=")//`)

func rewriteProtocolRelativeURLs(html string) string {
	return protocolRelativeURL.ReplaceAllString(html, "${1}https://")
}
