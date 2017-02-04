package proxy

import (
	"net"
)


type Proxy struct {
	addr *net.TCPAddr
}

var proxies []*Proxy

func AddProxy(addr string) error {
	proxy, err := New(addr)
	if err != nil {
		return err
	}
	
	proxies = append(proxies, proxy)
	return nil
}

func New(addr string) (*Proxy, error) {
	_addr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	
	p := Proxy {
		addr: _addr,
	}
	return &p, nil
}

func (p *Proxy)GetRemoteAddr() (*net.TCPAddr, error) {
	return p.addr, nil
}
