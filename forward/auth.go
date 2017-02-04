package forward

import (
	"strings"
	"encoding/base64"
)

func (fwd *Forward)ProxyBasicAuth() (username, password string, ok bool) {
	auth := fwd.request.Header.Get("Proxy-Authorization")
	if auth == "" {
		return
	}
	return parseProxyBasicAuth(auth)
}

func parseProxyBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}
