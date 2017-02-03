package proxy

import (
	"log"
	"time"
	"errors"
	"math/rand"
	"net"
	"net/http"
)


type Proxy struct {
	addr *net.TCPAddr
	conn *net.TCPConn
}

var proxies []*Proxy

//remote, err := proxy.SelectFromRequest(req)
//return remote.GetConn()

func AddProxy(addr string) error {
	proxy, err := New(addr)
	if err != nil {
		return err
	}
	
	proxies = append(proxies, proxy)
	return nil
}

func SelectRandom() (*Proxy, error) {
	rand.Seed(time.Now().Unix())

	r := rand.Intn(len(proxies))
	
	log.Printf("Random : %v with \n", r, proxies[r].addr.String())
	return proxies[r], nil
}

func SelectFromRequest(request *http.Request) (*Proxy, error) {
	return nil, errors.New("SelectFromRequest not implemented")
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
	p.conn.Close()
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
