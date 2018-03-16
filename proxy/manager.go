package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	redis "gopkg.in/redis.v5"

	"github.com/rookmoot/proxifier/logger"
)

type RedisInterface interface {
	SMembers(key string) *redis.StringSliceCmd
	SAdd(key string, members ...interface{}) *redis.IntCmd
	HMGet(key string, fields ...string) *redis.SliceCmd
	HMSet(key string, fields map[string]string) *redis.StatusCmd
	HSet(key, field string, value interface{}) *redis.BoolCmd
	HGet(key, field string) *redis.StringCmd
	Incr(key string) *redis.IntCmd
}

type Manager struct {
	db      RedisInterface
	log     logger.Logger
	proxies []*Proxy
}

func NewManager(db RedisInterface, log logger.Logger) (*Manager, error) {
	m := Manager{
		db:  db,
		log: log,
	}

	err := m.loadProxyList()
	if err != nil {
		return nil, err
	}

	return &m, nil
}

func (m *Manager) UpdateProxies(filepath string) error {
	proxies, err := m.readProxiesFromFile(filepath)
	if err != nil {
		return err
	}

	m.log.Info("%v", proxies[0])

	for _, proxy := range proxies {
		if proxy.GetAnonymityLevel() == "elite" && (proxy.GetProtocol() == "http" || proxy.GetProtocol() == "https") {
			if m.proxyExists(proxy) == false {
				m.log.Info("proxy: %v (%v, %v)", proxy.GetAddress(), proxy.GetProtocol(), proxy.GetAnonymityLevel())
				err = m.proxySave(proxy)
				if err != nil {
					m.log.Warn("proxy: %v", err)
				}
			}
		}
	}

	err = m.loadProxyList()
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) GetProxy() (*Proxy, error) {
	rand.Seed(time.Now().Unix())
	r := rand.Intn(len(m.proxies))
	return m.proxies[r], nil
}

func (m *Manager) loadProxyList() error {
	ret, err := m.db.SMembers("proxies").Result()
	if err != nil {
		return err
	}

	for _, v := range ret {
		pid, _ := strconv.Atoi(v)
		p, err := m.loadProxy(pid)
		if err != nil {
			m.log.Warn("proxy: %v", err)
		} else {
			m.proxies = append(m.proxies, p)
		}
	}

	return nil
}

func (m *Manager) loadProxy(pid int) (*Proxy, error) {
	data, err := m.db.HMGet(fmt.Sprintf("proxy:%d", pid), "ipaddress", "port", "protocol", "anonymitylevel", "source", "country").Result()
	if err != nil {
		return nil, err
	}

	infos := make(map[string]string, 6)
	infos["ipaddress"] = data[0].(string)
	infos["port"] = data[1].(string)
	infos["protocol"] = data[2].(string)
	infos["anonymitylevel"] = data[3].(string)
	infos["source"] = data[4].(string)
	infos["country"] = data[5].(string)

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%s", infos["ipaddress"], infos["port"]))
	if err != nil {
		return nil, err
	}

	p := Proxy{
		id:    pid,
		addr:  addr,
		infos: infos,
	}
	return &p, nil
}

func (m *Manager) proxySave(p Proxy) error {
	// INCR proxies_next_id
	// HMSET proxy:[ID] username [USERNAME] password [MD5HASH]
	// SADD proxies [ID]

	next_id, err := m.db.Incr("proxies_next_id").Result()
	if err != nil {
		return err
	}

	_, err = m.db.HMSet(fmt.Sprintf("proxy:%d", next_id), p.infos).Result()
	if err != nil {
		return err
	}

	_, err = m.db.SAdd("proxies", next_id).Result()
	if err != nil {
		return err
	}

	_, err = m.db.HSet("proxies_ids", p.GetAddress(), next_id).Result()
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) proxyExists(p Proxy) bool {
	_, err := m.db.HGet("proxies_ids", p.GetAddress()).Result()
	if err != nil {
		return false
	}
	return true
}

func (m *Manager) readProxiesFromFile(filepath string) ([]Proxy, error) {
	file, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var proxies []Proxy
	var values []map[string]interface{}

	err = json.Unmarshal([]byte(file), &values)
	if err != nil {
		return nil, err
	}

	for _, data := range values {

		infos := make(map[string]string, 6)
		for k, v := range data {
			if k == "protocols" {
				for _, tmp := range v.([]interface{}) {
					infos["protocol"] = tmp.(string)
				}
			} else if k == "port" {
				infos["port"] = fmt.Sprintf("%v", v)
			} else {
				infos[strings.ToLower(k)] = strings.ToLower(v.(string))
			}
		}

		p := Proxy{
			id:    0,
			addr:  nil,
			infos: infos,
		}
		proxies = append(proxies, p)
	}

	return proxies, nil
}
