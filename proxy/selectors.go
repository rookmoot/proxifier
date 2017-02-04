package proxy

import (
	"time"
	"errors"
	"math/rand"
	"net/http"
)

func SelectRandom() (*Proxy, error) {
	rand.Seed(time.Now().Unix())

	r := rand.Intn(len(proxies))

	return proxies[r], nil
}

func SelectFromRequest(request *http.Request) (*Proxy, error) {
	return nil, errors.New("SelectFromRequest not implemented")
}
