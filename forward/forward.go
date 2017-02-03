package forward

import (
	"bufio"
	"net"
	"errors"
	"strings"
	"net/http"
	"net/http/httputil"
	
	"github.com/rookmoot/proxifier/logger"
)

type Forward struct {
	conn      net.Conn
	remote	  Remote

	log       logger.Logger

	request  *http.Request
	response *http.Response

	MaxRetry  int
	data     interface{}
}

type Remote interface {
	Close()
	GetConn() (*net.TCPConn, error)
}

type OnAuthenticationHandlerFunc func(req *http.Request, username string, password string) (bool, error)
type OnToHandlerFunc func(req *http.Request) (Remote, error)
type OnHandlerFunc func(resp *http.Response, req *http.Request) (error)

func New(conn net.Conn, log logger.Logger) (*Forward, error) {
	fwd := Forward{
		conn: conn,
		remote: nil,
		log: log,
		MaxRetry: 1,
	}
	return &fwd, nil
}

func (fwd *Forward)SetData(data interface{}) {
	fwd.data = data
}

func (fwd *Forward)GetData() interface{} {
	return fwd.data
}

func (fwd *Forward)Close() {
	fwd.conn.Close()
	if fwd.remote != nil {
		fwd.remote.Close()
	}
}

func (fwd *Forward)On(cb OnHandlerFunc) error {
	return cb(fwd.response, fwd.request)
}

func (fwd *Forward)OnAuthentication(cb OnAuthenticationHandlerFunc) (bool, error) {
	return cb(fwd.request, "test", "test")
}

func (fwd *Forward)To(cb OnToHandlerFunc) error {
	// read client request from socket
	// Here we can check for proxy authentication
	// and others headers sent like X-PROXIFIER-
	// then, we NEED to remove those header to
	// send only clean headers
	err := fwd.readRequest()
	if err != nil {
		return err
	}

	// use the callback func to get the remote
	// net tcp connection to forward requests to
	remote, err := cb(fwd.request)
	if err != nil {
		return err
	}	
	fwd.remote = remote

	// Forward the request to select proxy remote
	// and get the according response
	err = fwd.forward()
	if err != nil {
		return err
	}
	
	// Send back remote proxy host response to initial
	// client.
	return fwd.response.Write(fwd.conn)
}

func (fwd *Forward)forward() error {
	if (fwd.MaxRetry) < 0 {
		return errors.New("Max retry reached.")
	}
	fwd.MaxRetry--


	remote_conn, err := fwd.remote.GetConn()
	if err != nil {
		return err
	}
	
	// Forward request to remote proxy host.
	err = fwd.request.WriteProxy(remote_conn)
	if err != nil {
		return err
	}
	
	// Read response from remote proxy host.
	// Here we NEED to check status code and other stuff
	// to get clean request and be able to serve only
	// valid content.
	// Status code to check :
	//   301 -> redirection
	//   4/5xx -> Check for error and retry
	err = fwd.readResponse()
	if err != nil {
		return err
	}

	return nil
}

func (fwd *Forward)readRequest() error {
	req, err := http.ReadRequest(bufio.NewReader(fwd.conn))
	if err != nil {
		return err
	}
	fwd.request = req

	dump, err := httputil.DumpRequest(fwd.request, false)
	if err == nil {
		fwd.log.Trace("Request :\n%v", string(dump))
	}

	err = fwd.filterRequest()
	if err != nil {
		return err
	}
	return nil
}

func (fwd *Forward)filterRequest() error {
	if fwd.request == nil {
		return errors.New("Can't filter forward request. Request is nil.")
	}
	
	for k, v := range fwd.request.Header {
		if (strings.HasPrefix(k, "Proxy-")) {
			// Handles the following headers and remove them
			// Proxy-Authorization: Basic dGVzdDp0ZXN0
			// Proxy-Connection: Keep-Alive
			
			fwd.request.Header.Del(k)			
		}
		
		if (strings.HasPrefix(k, "X-Proxifier")) {
			switch k {
			   // X-Proxifier-Https:
			   // This header made the http initial request to be transformed
			   // to an https request.
    			   case "X-Proxifier-Https":
				fwd.request.URL.Scheme = "https"
				r := strings.NewReplacer("http://", "https://")
				fwd.request.RequestURI = r.Replace(fwd.request.RequestURI)
			   default:
			}
		
			fwd.request.Header.Del(k)
		}
		v = v
	}
	return nil
}

func (fwd *Forward)readResponse() error {
	remote_conn, err := fwd.remote.GetConn()
	if err != nil {
		return err
	}
	
	resp, err := http.ReadResponse(bufio.NewReader(remote_conn), fwd.request);
	if err != nil {
		return err
	}
	fwd.response = resp

	dump, err := httputil.DumpResponse(fwd.response, false)
	if err == nil {
		fwd.log.Trace("Response :\n%v", string(dump))
	}

	err = fwd.filterResponse()
	if err != nil {
		return err
	}
	return nil
}

func (fwd *Forward)filterResponse() error {
        if fwd.response == nil {
		return errors.New("Can't filter forwarded response. Response is nil.")
	}

	// In case of redirect, perform the redirect.
	if fwd.response.StatusCode == 301 {
		url, err := fwd.response.Location()
		if err != nil {
			return err
		}
		fwd.request.URL = url
		fwd.request.RequestURI = url.String()
		fwd.forward()
	}

	return nil
}
