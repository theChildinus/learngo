package ipc

import (
	"encoding/json"
	"fmt"
)

type Request struct {
	Method string `json:"method"`
	Params string `json:"params"`
}

type Response struct {
	Code string `json:"code"`
	Body string `josn:"body"`
}

type Server interface {
	Name() string
	Handle(method, params string) *Response
}

type IpcServer struct {
	Server
}

func NewIpcServer(server Server) *IpcServer {
	return &IpcServer{server}
}

// 返回信道给客户端写入/读取
func (server *IpcServer) Connect() chan string {
	// session 是一个 channel
	session := make(chan string, 0)
	go func(c chan string) {
		for {
			// 当客户端未发送请求时，协程在这里阻塞
			request := <-c
			if request == "CLOSE" {
				break
			}

			var req Request
			err := json.Unmarshal([]byte(request), &req)
			if err != nil {
				fmt.Println("Invilid request format:", request)
			} else {
				fmt.Println(req)
			}
			resp := server.Handle(req.Method, req.Params)

			b, err := json.Marshal(resp)

			c <- string(b)
		}
		fmt.Println("session closed.")
	}(session)
	fmt.Println("A new session has been creted sucessful")
	// 客户端第一次调用 Connect函数 时，协程阻塞，主线程从这里返回session
	return session
}
