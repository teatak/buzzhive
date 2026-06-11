package protocol

import "strings"

func ShouldPassthrough(inbound, outbound string) bool {
	return normalize(inbound) == normalize(outbound)
}

func normalize(protocol string) string {
	return strings.ToLower(strings.TrimSpace(protocol))
}
