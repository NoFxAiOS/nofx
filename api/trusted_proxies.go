package api

import (
	"fmt"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// Forwarded headers are untrusted unless the TCP peer is an explicitly
// configured reverse proxy. Never use a client-controlled IP as a throttle key.
func configureTrustedProxies(router *gin.Engine, raw string) error {
	var proxies []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			if ip.IsUnspecified() {
				return fmt.Errorf("unspecified proxy address is not allowed")
			}
		} else {
			_, block, err := net.ParseCIDR(entry)
			if err != nil {
				return fmt.Errorf("proxy must be an IP address or CIDR")
			}
			if ones, _ := block.Mask.Size(); ones == 0 {
				return fmt.Errorf("trusting every address is not allowed")
			}
		}
		proxies = append(proxies, entry)
	}
	return router.SetTrustedProxies(proxies)
}
