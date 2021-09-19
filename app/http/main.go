package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jseow5177/tcp-pool/internal/model"
	"github.com/jseow5177/tcp-pool/internal/tcp"
	"github.com/jseow5177/tcp-pool/internal/util"

	conf "github.com/jseow5177/tcp-pool/config"
)

type application struct {
	TcpPool *tcp.TcpConnPool
}

func main() {
	c := conf.InitConfig()

	// Create tcp connection pool
	tcpPool, err := tcp.CreateTcpConnPool(c.TcpConfig)
	if err != nil {
		log.Fatalf("fail to connect to TCP server, err: %v", err)
	}

	app := &application{
		TcpPool: tcpPool,
	}

	// Register a handler
	r := mux.NewRouter()
	r.HandleFunc("/user/login", app.handleUserLogin).Methods("POST")

	// Start HTTP server
	log.Printf("http server started at port %d", c.HttpConfig.Port)
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", c.HttpConfig.Host, c.HttpConfig.Port), r)

	if err != nil {
		log.Fatalf("error starting http server, err: %v", err)
	}
}

// handleUserLogin() is a HTTP handler for user login
func (app *application) handleUserLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// Read user input
	err := util.ReadJSON(r, &input)
	if err != nil {
		util.ServerErrorResponse(w, err)
		return
	}

	req := &model.LoginUserRequest{
		Username: input.Username,
		Password: input.Password,
	}
	res := &model.LoginUserResponse{}
	// Proxy request to TCP server
	err = app.sendToTCPServer(conf.CmdLoginUser, req, res)
	if err != nil {
		util.ServerErrorResponse(w, err)
		return
	}

	// Can create more sophisticated error codes in TCP server
	// and map the codes back to HTTP codes
	// I'll skip this approach in this dummy application.
	status := http.StatusOK
	if res.ErrCode != 0 {
		status = http.StatusInternalServerError
	}

	// Return response back to client
	util.WriteJSON(w, status, util.Envelope{"msg": res.Message}, nil)
}

// sendToTCPServer() is a helper method that sends reqData to the TCP server
// and unmarshal the response into resDst.
func (app *application) sendToTCPServer(command string, reqData interface{}, resDst interface{}) error {
	requestData, err := json.Marshal(reqData)
	if err != nil {
		return err
	}

	packet := &tcp.Packet{
		Command: command,
		Data:    requestData,
	}
	tcpPacket, err := json.Marshal(packet)
	if err != nil {
		return err
	}

	res, err := app.TcpPool.SendData(tcpPacket)
	if err != nil {
		return err
	}

	err = json.Unmarshal(res, resDst)
	if err != nil {
		return err
	}

	return nil
}
