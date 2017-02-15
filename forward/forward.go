package forward

import (
	"time"
	"bytes"
	"bufio"
	"io/ioutil"
	"net"
	"errors"
	"strings"
	"strconv"
	"net/http"
	"net/http/httputil"
	
	"github.com/rookmoot/proxifier/logger"
)

type Remote interface {
	GetRemoteAddr() (*net.TCPAddr, error)
}

type User interface {
	Limit() (count, reset int64, allow bool)
	IsConnected() bool
}

type Forward struct {
	log           logger.Logger

	conn          net.Conn
	user	      User
	
	request      *http.Request
	response     *http.Response

	authHandler   OnAuthenticationHandlerFunc
	remoteHandler OnToHandlerFunc
	httpHandler   []OnHandlerFunc
	
	maxRetry      int
	data          interface{}

	rate int64
	reset int64
	allowed bool
}

var unauthorizedMsg = []byte("Proxy Authentication Required")
var errorMsg        = []byte("Proxy internal error")

type OnAuthenticationHandlerFunc func(req *http.Request, username string, password string) (User, error)
type OnToHandlerFunc func(req *http.Request) (Remote, error)
type OnHandlerFunc func(resp *http.Response, req *http.Request) (error)

func New(conn net.Conn, log logger.Logger) (*Forward, error) {
	fwd := Forward{
		conn: conn,
		log: log,
		maxRetry: 20,
	}
	return &fwd, nil
}

func (fwd *Forward)SetData(data interface{}) {
	fwd.data = data
}

func (fwd *Forward)GetData() interface{} {
	return fwd.data
}

func (fwd *Forward)GetUser() User {
	return fwd.user
}

func (fwd *Forward)Close() {
	fwd.conn.Close()
}

func (fwd *Forward)On(cb OnHandlerFunc) {
	fwd.httpHandler = append(fwd.httpHandler, cb)
}

func (fwd *Forward)OnAuthentication(cb OnAuthenticationHandlerFunc) {
	fwd.authHandler = cb
}

func (fwd *Forward)OnSelectRemote(cb OnToHandlerFunc) {
	fwd.remoteHandler = cb
}

func (fwd *Forward)Forward() error {
	// read client request from socket
	// Here we can check for proxy authentication
	// and others headers sent like X-PROXIFIER-
	// then, we NEED to remove those header to
	// send only clean headers
	err := fwd.readRequest()
	if err != nil {
		fwd.createErrorResponse(500, []byte("Failed to read sent request."))
		return err
	}


	// check for Ratelimited user acccess
	if fwd.user != nil && fwd.user.IsConnected() {
		fwd.rate, fwd.reset, fwd.allowed = fwd.user.Limit()
		if fwd.allowed == false {
			fwd.createErrorResponse(400, []byte("User rate limits exceeded."))
			return errors.New("User rate limits exceeded.")
		}
	}

	err = nil
	// Forward the request to select proxy remote
	// and get the according response
	for fwd.maxRetry > 0 {
		err = fwd.forward()
		if err == nil {
			break
		}
		fwd.maxRetry--
	}
	if err != nil {
		fwd.createErrorResponse(500, []byte(err.Error()))
		return err
	}

	// Send request and response to callbacks
	// The user can manage request and response
	// before they are sent back.
	for _, cb := range fwd.httpHandler {
		err = cb(fwd.response, fwd.request)
		if err != nil {
			return err
		}
	}
	
	// Send back remote proxy host response to initial
	// client.
	return fwd.response.Write(fwd.conn)
}

func (fwd *Forward)getRemoteConn(timeout time.Duration) (net.Conn, error) {
	if fwd.remoteHandler == nil {
		return nil, errors.New("No callback for fwd.OnSelectRemote() found. Can't perform request.")
	}
	
	remote, err := fwd.remoteHandler(fwd.request)
	if err != nil {
		return nil, err
	}

	remote_addr, err := remote.GetRemoteAddr()
	if err != nil {
		return nil, err
	}

	fwd.log.Info("Trying with remote %v", remote_addr.String())
	conn, err := net.DialTimeout("tcp", remote_addr.String(), timeout)
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(timeout))

	return conn, nil
}

func (fwd *Forward)forward() error {
	timeout_delta := 5 * time.Second;
	remote, err := fwd.getRemoteConn(timeout_delta)
	if err != nil {
		return err
	}
	
	// Forward request to remote proxy host.
	err = fwd.request.WriteProxy(remote)
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
	err = fwd.readResponse(remote)
	if err != nil {
		return err
	}
	return remote.Close()
}

func (fwd *Forward)authenticate() error {
	if fwd.authHandler == nil {		
		return nil
	}

	username, password, ok := fwd.ProxyBasicAuth()
	if ok == false {
		return errors.New("No authentication header found.")
	}
	
	_user, err := fwd.authHandler(fwd.request, username, password)
	if _user == nil {
		return errors.New("Returned nil user during authentication")
	}
	
	if err != nil {
		return err
	}

	if _user.IsConnected() == false {
		return errors.New("Failed to log user in. No user found.")
	}
	
	fwd.user = _user
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

	// clean up necessary stuff
	fwd.request.Header.Del("Connection")
	fwd.request.Header.Del("Accept-Encoding")
	
	// check for headers specifics operations
	for k, _ := range fwd.request.Header {
		if (strings.HasPrefix(k, "Proxy-")) {
			// Handles the following headers and remove them
			// Proxy-Authorization: Basic dGVzdDp0ZXN0
			// Proxy-Connection: Keep-Alive

			switch k {
  			   case "Proxy-Authorization":
				err := fwd.authenticate()
				if err != nil {
					fwd.createErrorResponse(407, unauthorizedMsg)
					return err
				}
				
			   default:
			}
			
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
	}

	// Check if we have a callback for authentication. if true, then we need to have
	// a valid user set.
	if fwd.authHandler != nil && fwd.user == nil {
		fwd.createErrorResponse(407, unauthorizedMsg)
		return errors.New("You need to send your authentication credentials")
	}
	
	return nil
}

func (fwd *Forward)readResponse(remote net.Conn) error {
	resp, err := http.ReadResponse(bufio.NewReader(remote), fwd.request);
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

	fwd.response.Header.Set("X-RateLimit-Limit", strconv.FormatInt(60, 10))
	fwd.response.Header.Set("X-RateLimit-Remaining", strconv.FormatInt((60 - fwd.rate), 10))
	fwd.response.Header.Set("X-RateLimit-Reset", strconv.FormatInt(fwd.reset, 10))

	if fwd.response.StatusCode != 200 {
		return errors.New("No 200 status code response")
	}
	return nil
}


func (fwd *Forward)createErrorResponse(code int, reason []byte) {
	reason = append(reason, byte('\n'))
	fwd.response = &http.Response{
		StatusCode:    code,
		ProtoMajor:    1,
		ProtoMinor:    1,
		Request:       fwd.request,
		Body:          ioutil.NopCloser(bytes.NewBuffer(reason)),
		ContentLength: int64(len(reason)),
	}

	if code == 407 {
		// Automaticaly add a Proxy-Authenticate Header when the client need to
		// be logged.
		fwd.response.Header = http.Header{"Proxy-Authenticate": []string{"Basic realm="}};
	}

	fwd.response.Write(fwd.conn)
}
