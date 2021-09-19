package main

import (
	"fmt"
	"log"
	"net"

	"github.com/jseow5177/tcp-pool/internal/model"
	"github.com/jseow5177/tcp-pool/internal/tcp"

	conf "github.com/jseow5177/tcp-pool/config"
)

func main() {
	c := conf.InitConfig()

	// Register a handler for the login_user command
	tcp.RegisterHandler(&tcp.HandlerConfig{
		Command: conf.CmdLoginUser,
		Handler: func(req interface{}, res interface{}) {
			LoginUser(req.(*model.LoginUserRequest), res.(*model.LoginUserResponse))
		},
		Request:  &model.LoginUserRequest{},
		Response: &model.LoginUserResponse{},
	})

	// Start TCP server
	addr := fmt.Sprintf("%s:%d", c.TcpConfig.Host, c.TcpConfig.Port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("error starting tcp server: %s", err.Error())
	}
	defer l.Close()

	log.Printf("tcp server started at port %d", c.TcpConfig.Port)

	// Accept TCP connections
	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatalf("tcp server failed to connect, err: %v\n", err)
		}
		go tcp.HandleClientConnection(c)
	}
}

func LoginUser(req *model.LoginUserRequest, res *model.LoginUserResponse) {
	res.Message = "Got it!"
}
