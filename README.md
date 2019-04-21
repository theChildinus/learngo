# LearnGo

## ipc

`ipc` 包中实现了简单的ipc框架

![ipc](https://raw.githubusercontent.com/theChildinus/Note/master/image/ipc.png)

`cg` 包中实现了以下几个角色：

- 玩家（player.go）：接收消息
- 中心服务器（center.go）：处理请求
- 中心客户端（centerclient.go）：发起请求

请求包括：登录、登出、查看在线玩家、广播消息

## net/http

一个Go最简单的Http服务器 - [参考学习这里](https://www.jianshu.com/p/be3d9cdc680b)

简单流程为：

```txt
Client -> Requests -> Multiplexer(router) -> Handler -> Response -> Client
```

```go
package main

import (
    "fmt"
    "net/http"
    "strings"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "hello kong!")
}

func main() {
    http.HandleFunc("/", IndexHandler)
    http.ListenAndServe(":9090", nil)
}

```

Go net/http 流程图

![net/http](https://raw.githubusercontent.com/theChildinus/Note/master/image/11043-b203aff690e35cfc.png)

### 关键结构

#### Handler

Golang没有继承，类多态的方式可以通过接口实现，所谓接口则是定义声明了函数签名，任何结构只要实现了与接口函数签名相同的方法，就等同于实现了接口，go的http服务都是基于handler进行处理的

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

任何结构体，只要实现了 `ServeHTTP` 方法，这个结构就可以称之为 Handler 对象。ServeMux会使用 Handler 并调用其ServeHTTP方法处理请求并返回响应。

#### ServeMux

ServeMux结构中重要的是 `m`，这是一个map，key是一些url模式，value是一个muxEntry结构体，后者定义了具体的 `url` 模式和 `Handler`

```go
// ServeMux is an HTTP request multiplexer.
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that
// most closely matches the URL.
type ServeMux struct {
    mu    sync.RWMutex
    m     map[string]muxEntry
    hosts bool
}

type muxEntry struct {
    explicit bool
    h        Handler
    pattern  string
}
```

#### Server

在 `http.ListenAndServe` 源代码可以看出它创建了一个Server对象，并调用对象的 `ListenAndServe` 方法

```go
func ListenAndServe(addr string, handler Handler) error {
    server := &Server{Addr: addr, Handler: handler}
    return server.ListenAndServe()
}
```

```go
type Server struct {
    Addr    string  // TCP address to listen on, ":http" if empty
    Handler Handler // handler to invoke, http.DefaultServeMux if nil

    TLSConfig *tls.Config
    ReadTimeout time.Duration
    ReadHeaderTimeout time.Duration
    WriteTimeout time.Duration
    IdleTimeout time.Duration
    MaxHeaderBytes int
    TLSNextProto map[string]func(*Server, *tls.Conn, Handler)
    ConnState func(net.Conn, ConnState)
    ErrorLog *log.Logger

    disableKeepAlives int32     // accessed atomically.
    inShutdown        int32     // accessed atomically (non-zero means we're in Shutdown)
    nextProtoOnce     sync.Once // guards setupHTTP2_* init
    nextProtoErr      error     // result of http2.ConfigureServer if used

    mu         sync.Mutex
    listeners  map[net.Listener]struct{}
    activeConn map[*conn]struct{}
    doneChan   chan struct{}
    onShutdown []func()
}
```

### 创建 HTTP 服务

创建一个http服务，大致需要经历两个过程，首先需要注册路由，即提供 url模式 和 handler函数 的映射，其次就是实例化一个 Server对象，并开启对客户端的监听。

### 注册路由

net/http暴露的注册路由API很简单：

```go
http.HandleFunc("/", IndexHandler)
```

`http.HandleFunc` 选取了 `DefaultServeMux` 作为 `multiplexer` 路由器

```go
func HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
    DefaultServeMux.HandleFunc(pattern, handler)
}
```

`DefaultServeMux` 实际上就是 `ServeMux` 的实例，创建过程为：

```go
// NewServeMux allocates and returns a new ServeMux.
func NewServeMux() *ServeMux { return new(ServeMux) }

// DefaultServeMux is the default ServeMux used by Serve.
var DefaultServeMux = &defaultServeMux

var defaultServeMux ServeMux
```

因此，`DefaultServeMux` 的 `HandleFunc(pattern, handler)` 是定义在 `ServeMux` 下的 （**注意区分 HandleFunc 和 HandlerFunc**）：

```go
// HandleFunc registers the handler function for the given pattern.
func (mux *ServeMux) HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
    mux.Handle(pattern, HandlerFunc(handler))
}
```

上面的 `HandlerFunc` 定义如下，是一个将普通函数用作 HTTP 处理程序的适配器，同时还实现了 `Handler` 接口的 `ServeHttp` 方法，旨在让 `handler` 函数也具有 `ServeHTTP` 方法：

```go
// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as HTTP handlers. If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler that calls f.
type HandlerFunc func(ResponseWriter, *Request)

// ServeHTTP calls f(w, r).
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)
}
```

回到 `ServeMux` 的方法 `HandleFunc` 中，在 `ServeMux` 的 `Handle` 方法中，将会对 `pattern` 和 handler 函数做一个映射：

```go
// Handle registers the handler for the given pattern.
// If a handler already exists for pattern, Handle panics.
func (mux *ServeMux) Handle(pattern string, handler Handler) {
    mux.mu.Lock()
    defer mux.mu.Unlock()

    if pattern == "" {
        panic("http: invalid pattern")
    }
    if handler == nil {
        panic("http: nil handler")
    }
    if _, exist := mux.m[pattern]; exist {
        panic("http: multiple registrations for " + pattern)
    }

    if mux.m == nil {
        mux.m = make(map[string]muxEntry)
    }
    mux.m[pattern] = muxEntry{h: handler, pattern: pattern}

    if pattern[0] != '/' {
        mux.hosts = true
    }
}
```

由此可见，`Handle` 函数的主要目的在于把 handler 和 pattern 模式绑定到 `map[string]muxEntry` 的map上，此时 `pattern` 和 `handler` 的路由注册完成，接下来就可以开始 `server` 监听，接收 客户端的请求

### 开启监听

`http` 的 `ListenAndServer` 方法中可以看到创建了一个 Server对象，并调用了Server对象的同名方法，在方法中调用了 `net.Listen("tcp", addr)` 监听我们设置的端口

```go
// ListenAndServe listens on the TCP network address addr
// and then calls Serve with handler to handle requests
// on incoming connections.
// Accepted connections are configured to enable TCP keep-alives.
// Handler is typically nil, in which case the DefaultServeMux is
// used.
func ListenAndServe(addr string, handler Handler) error {
    server := &Server{Addr: addr, Handler: handler}
    return server.ListenAndServe()
}

// ListenAndServe listens on the TCP network address srv.Addr and then
// calls Serve to handle requests on incoming connections.
// Accepted connections are configured to enable TCP keep-alives.
// If srv.Addr is blank, ":http" is used.
// ListenAndServe always returns a non-nil error.
func (srv *Server) ListenAndServe() error {
    addr := srv.Addr
    if addr == "" {
        addr = ":http"
    }
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        return err
    }
    return srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
}
```

Server 的 `ListenAndServe` 方法中，会初始化监听地址 `Addr`，同时调用 `Listen` 方法设置监听，`ln` 为监听器 `Listener`，将`ln` 的 `tcp` 对象传入 `Serve` 方法：

```go
// Serve accepts incoming connections on the Listener l, creating a
// new service goroutine for each. The service goroutines read requests and
// then call srv.Handler to reply to them.
func (srv *Server) Serve(l net.Listener) error {
    defer l.Close()
    ...

    baseCtx := context.Background() // base is always background, per Issue 16220
    ctx := context.WithValue(baseCtx, ServerContextKey, srv)
    for {
        rw, e := l.Accept()
        ...

        c := srv.newConn(rw)
        c.setState(c.rwc, StateNew) // before Serve can return
        go c.serve(ctx)
    }
}
```

在上面可以看到，Go为了实现高并发和高性能, 使用了 `goroutines` 来处理Conn的读写事件, 这样每个请求都能保持独立，相互不会阻塞，可以高效的响应网络事件。这是Go高效的保证。

这里我们可以看到客户端的每次请求都会创建一个Conn，这个Conn里面保存了该次请求的信息，然后再传递到对应的 `handler`，该handler中便可以读取到相应的header信息，这样保证了每个请求的独立性。

```go
// A conn represents the server side of an HTTP connection.
type conn struct {
    // server is the server on which the connection arrived.
    // Immutable; never nil.
    server *Server

    // cancelCtx cancels the connection-level context.
    cancelCtx context.CancelFunc
    tlsState *tls.ConnectionState
    werr error
    r *connReader
    bufr *bufio.Reader
    bufw *bufio.Writer
    lastMethod string
    curReq atomic.Value // of *response (which has a Request in it)
    curState atomic.Value // of ConnState
    mu sync.Mutex
    hijackedv bool
}
```

那么如何 具体分配到 相应的函数上来处理请求呢？

### 处理请求

在 `go c.serve(ctx)` 中，一个连接就是一个协程，`conn` 首先会解析 request，调用 `c.readRequest()`

```go
// Serve a new connection.
func (c *conn) serve(ctx context.Context) {
    ...
    // HTTP/1.x from here on.

    ctx, cancelCtx := context.WithCancel(ctx)
    c.cancelCtx = cancelCtx
    defer cancelCtx()

    c.r = &connReader{conn: c}
    c.bufr = newBufioReader(c.r)
    c.bufw = newBufioWriterSize(checkConnErrorWriter{c}, 4<<10)

    for {
        w, err := c.readRequest(ctx)
        if c.r.remain != c.server.initialReadLimitSize() {
            // If we read any bytes off the wire, we're active.
            c.setState(c.rwc, StateActive)
        }
        if err != nil {
            ...
        }
        // HTTP cannot have multiple simultaneous active requests.[*]
        // Until the server replies to this request, it can't read another,
        // so we might as well run the handler in this goroutine.
        // [*] Not strictly true: HTTP pipelining. We could let them all process
        // in parallel even if their responses need to be serialized.
        // But we're not going to implement HTTP pipelining because it
        // was never deployed in the wild and the answer is HTTP/2.
        serverHandler{c.server}.ServeHTTP(w, w.req)
        w.cancelCtx()

        w.finishRequest()
        if !w.shouldReuseConnection() {
            if w.requestBodyLimitHit || w.closedRequestBodyEarly() {
                c.closeWriteAndWait()
            }
            return
        }
        c.setState(c.rwc, StateIdle)
        c.curReq.Store((*response)(nil))

        if !w.conn.server.doKeepAlives() {
            // We're in shutdown mode. We might've replied
            // to the user without "Connection: close" and
            // they might think they can send another
            // request, but such is life with HTTP/1.1.
            return
        }
        ...
    }
```

`serverHandler` 定义如下：

```go
// serverHandler delegates to either the server's Handler or
// DefaultServeMux and also handles "OPTIONS *" requests.
type serverHandler struct {
    srv *Server
}

func (sh serverHandler) ServeHTTP(rw ResponseWriter, req *Request) {
    handler := sh.srv.Handler
    if handler == nil {
        handler = DefaultServeMux
    }
    ...
    handler.ServeHTTP(rw, req)
}
```

然后获取相应的handler, 在 `serverHandler{c.server}.ServeHTTP(w, w.req)` 中有 `handler := c.server.Handler`，也就是我们刚才在调用函数 `ListenAndServe` 时候的第二个参数，我们前面例子传递的是 `nil`，也就是为空，那么默认获取 `handler = DefaultServeMux`，那么这个变量用来做什么的呢？这个变量就是一个路由器，它用来匹配 `url` 跳转到其相应的 `handle` 函数

我们调用的代码里面第一句调用了 `http.HandleFunc("/", IndexHandler)`，这个作用就是注册了请求 `/` 的路由规则，当请求url为 `/`，路由就会转到函数 `IndexHandler`，`DefaultServeMux` 会调用ServeHTTP方法，这个方法内部其实就是调用 `IndexHandler` 本身，最后通过写入`response` 的信息反馈到客户端。

`serverHandler{c.server}.ServeHTTP(w, w.req)` 运行结束后就是对请求处理完毕之后上希望和连接断开的相关逻辑

至此，Golang中一个完整的http服务介绍完毕