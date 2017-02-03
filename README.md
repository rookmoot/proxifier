# Proxifier
An intelligent proxy dispatcher perfect for crawling and scraping public data. Proxifier act as a proxy and remotely send and receive requests and responses from other proxies.

# Getting Started
Firstly, just download and install proxifier.

```
go get github.com/rookmoot/proxifier
```

# Example

```go
package main

import (
        "net"
        "net/http"

        "github.com/rookmoot/proxifier/logger"
        "github.com/rookmoot/proxifier/forward"
)
var (
        log = logger.ColorLogger{}
)

func handleRequest(conn net.Conn) {
        log.Info("New client connected")

        fwd, err := forward.New(conn, log)
        if err != nil {
                log.Warn("%v", err)
                return
        }
        defer fwd.Close()
        err = fwd.To(func(req *http.Request) (*net.TCPConn, error) {
                addr, err := net.ResolveTCPAddr("tcp", "A PROXY IP:PORT HERE")
                if err != nil {
                        panic(err)
                }
                return net.DialTCP("tcp", nil, addr)
        })

        if err != nil {
                log.Warn("%v", err)
        }
}

func main() {
        log.Verbose = true
        log.Color = true

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

                go handleRequest(conn)
        }
}
```

You NEED to change `"A PROXY IP:PORT HERE"` to a real proxy `"IP:PORT"` address here, otherwise your application will fail while trying to contact another proxy.

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
This project has just started, fill free to provide any feedback or pull requests.
