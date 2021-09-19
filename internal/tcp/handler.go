package tcp

import (
	"encoding/json"
	"net"
)

// A map of commands to handler functions
var handlers map[string]*HandlerConfig

func init() {
	handlers = make(map[string]*HandlerConfig)
}

// HandlerConfig is a set of configurations for a request handler in TCP server.
type HandlerConfig struct {
	Command  string
	Handler  func(req interface{}, res interface{})
	Request  interface{}
	Response interface{}
}

func RegisterHandler(hc *HandlerConfig) {
	handlers[hc.Command] = hc
}

// HandleClientConnection() handles a connection from client
// It runs in a loop while waiting to read data
func HandleClientConnection(conn net.Conn) {
	var (
		err        error
		data       []byte
		response   []byte
		clientConn = &tcpConn{conn: conn}
	)

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	for {
		data, err = clientConn.Read()
		if err != nil {
			return
		}

		packet := &Packet{}
		err = json.Unmarshal(data, &packet)
		if err != nil {
			return
		}

		handlerCfg, exist := handlers[packet.Command]
		if !exist {
			err = ErrMissingHandler
			return
		}

		err = json.Unmarshal(packet.Data, handlerCfg.Request)
		if err != nil {
			return
		}

		// Route client request to handler
		handlerCfg.Handler(handlerCfg.Request, handlerCfg.Response)

		response, err = json.Marshal(handlerCfg.Response)
		if err != nil {
			return
		}

		_, err = clientConn.Write(response)
		if err != nil {
			return
		}
	}
}
