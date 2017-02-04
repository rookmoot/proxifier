package proxy

import (
	"net"
)


type Proxy struct {
	addr *net.TCPAddr
	conn *net.TCPConn
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
		conn: nil,
	}
	return &p, nil
}

func (p *Proxy)Close() {
	if p.conn != nil {
		p.conn.Close()
	}
	p.conn = nil
}

func (p *Proxy)GetConn() (*net.TCPConn, error) {
	if p.conn == nil {
		_conn, err := net.DialTCP("tcp", nil, p.addr)
		if err != nil {
			return nil, err
		}
		p.conn = _conn
	}
	return p.conn, nil
}
