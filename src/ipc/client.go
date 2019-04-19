package ipc

import "encoding/json"

type IpcClient struct {
	conn chan string
}

// 返回channel 作为客户端绑定的channel
func NewIpcClient(server *IpcServer) *IpcClient {
	c := server.Connect()
	return &IpcClient{c}
}

func (client *IpcClient) Call(method, params string) (resp *Response, err error) {
	req := &Request{method, params}

	var b []byte
	b, err = json.Marshal(req)

	if err != nil {
		return
	}

	// 通过客户端会话信道发送到服务器，阻塞
	client.conn <- string(b)
	// 直到服务器端向信道写入消息
	str := <-client.conn

	var resp1 Response
	err = json.Unmarshal([]byte(str), &resp1)
	resp = &resp1
	return
}

func (client *IpcClient) Close() {
	client.conn <- "CLOSE"
}
