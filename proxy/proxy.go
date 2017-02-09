package proxy

import (
	"net"
	"fmt"
)

// INCR proxies_next_id
// HMSET proxy:1 username [USERNAME] password [MD5HASH]
// HSET proxies [ip:port] 1                  
// {"ipAddress":"50.232.240.134","port":3128,"protocols":["http"],"anonymityLevel":"elite","source":"freeproxylists","country":"us"}

type Proxy struct {
	id int
	addr *net.TCPAddr
	infos map[string]string
}

func (p *Proxy)GetAddress() string {
	return fmt.Sprintf("%s:%v", p.infos["ipaddress"], p.infos["port"])
}

func (p *Proxy)GetAnonymityLevel() string {
	return p.infos["anonymitylevel"]
}

func (p *Proxy)GetProtocol() string {
	return p.infos["protocol"]
}

func (p *Proxy)GetRemoteAddr() (*net.TCPAddr, error) {
	return p.addr, nil
}
