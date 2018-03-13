# Proxifier
A fast, modern and intelligent proxy rotator perfect for crawling and scraping public data. Proxifier act as a proxy and remotely send and receive requests and responses from other proxies.

# Getting Started
Firstly, just download and install proxifier.

```
go get github.com/rookmoot/proxifier
```

# Example

First create a file that will contain the list of proxies. You NEED to change the `ipAddress` to a real proxy IP address. Likewise update `port` to the actual PORT. Otherwise your application will fail while trying to contact the proxy.
```json
[
    {
       "ipAddress":"1.1.1.1",
       "port":9000,
       "protocols":[
          "https"
       ],
       "anonymityLevel":"elite",
       "source":"optional",
       "country":"us"
    },
    {
       "ipAddress":"2.2.2.2",
       "port":9000,
       "protocols":[
          "http"
       ],
       "anonymityLevel":"elite",
       "source":"-",
       "country":"us"
    }
]
```

Update the following example to change `PROXY_PATH` to the path of the list of proxies.

```go
package main

import (
        "net"
        "net/http"

        redis "gopkg.in/redis.v5"

        "github.com/rookmoot/proxifier/forward"
        "github.com/rookmoot/proxifier/logger"
        "github.com/rookmoot/proxifier/proxy"
)

const (
        PROXY_PATH = "/home/joseph/work/src/github.com/rookmoot/proxifier/proxy_data.json"
)

var (
        log = logger.ColorLogger{}
)

type SimpleHandler struct {
        M *proxy.Manager
}

func (t *SimpleHandler) handleRequest(conn net.Conn) {
        log.Info("New client connected")

        fwd, err := forward.New(conn, log)
        if err != nil {
                log.Warn("%v", err)
                return
        }
        defer fwd.Close()

        fwd.OnSelectRemote(func(req *http.Request) (forward.Remote, error) {
                return t.M.GetProxy()
        })

        err = fwd.Forward()
        if err != nil {
                log.Warn("%v", err)
        }
}

func main() {
        log.Verbose = true
        log.Color = true

        r := redis.NewClient(
                &redis.Options{
                        Network:  "unix",
                        Addr:     "/var/run/redis/redis.sock",
                        Password: "",
                        DB:       0,
                },
        )

        proxyManager, err := proxy.NewManager(r, log)

        if err != nil {
                panic(err)
        }

        proxyManager.UpdateProxies(PROXY_PATH)

        t := SimpleHandler{
                M: proxyManager,
        }

        addr, err := net.ResolveTCPAddr("tcp", "localhost:8080")
        if err != nil {
                panic(err)
        }

        listener, err := net.ListenTCP("tcp", addr)
        if err != nil {
                panic(err)
        }
        defer listener.Close()

        for {
                conn, err := listener.AcceptTCP()
                if err != nil {
                        log.Warn("%v", err)
                }

                go t.handleRequest(conn)
        }
}

```

# Try it !

To test it, just run for example :
```
curl -vx http://127.0.0.1:8080 http://httpbin.org/ip
```

If you want to try some HTTPS server, you may need to add a `X-Proxifier-Https` header in order to change your http:// url to https. For example if you want to test : `https://httpbin.org/ip`

```
curl -vx http://127.0.0.1:8080 -H "X-Proxifier-Https: On" http://httpbin.org/ip
```

# Still in Beta
This project has just started, feel free to provide any feedback or pull requests.
