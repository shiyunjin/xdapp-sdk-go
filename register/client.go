package register

import (
	"crypto/tls"
	"fmt"
	"github.com/leesper/tao"
	"net"
	"sync"
	"time"
)

var (
	request        Request
	requestId      *tao.AtomicInt64 // 请求id 原子递增
	receiveBuff     *safeReceiveBuff
	receiveChanMap = make(map[string]chan interface{})
)

type safeReceiveBuff struct {
	bufMap map[string][]byte
	mu    sync.Mutex
}

func init() {
	receiveBuff = &safeReceiveBuff{}
}

func NewClient(host string, port int, ssl bool) *tao.ClientConn {
	if host == "" {
		panic("缺少tcp host")
	}

	c := doConnect(host, port, ssl)
	onConnect := tao.OnConnectOption(func(c tao.WriteCloser) bool {
		return true
	})

	onError := tao.OnErrorOption(func(c tao.WriteCloser) {})

	// 连接关闭 1秒后重连
	onClose := tao.OnCloseOption(func(c tao.WriteCloser) {
		Logger.Debug("RPC服务连接关闭，等待重新连接")
		doConnect(host, port, ssl)
	})

	onMessage := tao.OnMessageOption(func(msg tao.Message, c tao.WriteCloser) {
		ver     := msg.(Request).Ver
		flag    := msg.(Request).Flag
		body    := msg.(Request).Body
		header  := msg.(Request).Header
		context := msg.(Request).Context

		// 返回数据的模式
		if (flag & FlagResultMode) == FlagResultMode {
			sendRpcReceive(flag, header, body)
			return
		}
		transportRpcRequest(c, flag, ver, header, context, RpcHandle(body))
	})

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
	}
	requestId = tao.NewAtomicInt64(0)
	options := []tao.ServerOption{
		onConnect,
		onError,
		onClose,
		onMessage,
		tao.TLSCredsOption(tlsConf),
		tao.ReconnectOption(),
		tao.CustomCodecOption(request),
	}

	tao.Register(request.MessageNumber(), Unserialize, nil)

	return tao.NewClientConn(0, c, options...)
}

func doConnect(host string, port int, ssl bool) net.Conn {
	address := fmt.Sprintf("%s:%s", host, IntToStr(port))
	if ssl {
		c, err := tls.Dial("tcp", address, &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			Logger.Warn("RPC服务连接错误，等待重新连接" + err.Error())
			time.Sleep(2 * time.Second)
			return doConnect(host, port, ssl)
		}
		return c
	} else {
		c, err := net.Dial("tcp", address)
		if err != nil {
			Logger.Warn("RPC服务连接错误，等待重新连接" + err.Error())
			time.Sleep(2 * time.Second)
			return doConnect(host, port, ssl)
		}
		return c
	}
}

// rpc 请求返回
func sendRpcReceive(flag byte, header Header, body[]byte) {
	reqId := IntToStr(header.RequestId)
	finish := (flag & FlagFinish) == FlagFinish

	if finish == false {
		receiveBuff.mu.Lock()
		receiveBuff.bufMap[reqId] = body
		receiveBuff.mu.Unlock()

		// 30秒后清理数据
		Conn.RunAt(time.Now().Add(RpcClearBufTime * time.Second), func(i time.Time, closer tao.WriteCloser) {
			receiveBuff.mu.Lock()
			delete(receiveBuff.bufMap, reqId)
			receiveBuff.mu.Unlock()
		})
		return
	} else if receiveBuff.bufMap[reqId] != nil {
		body = BytesCombine(receiveBuff.bufMap[reqId], body)
		receiveBuff.mu.Lock()
		delete(receiveBuff.bufMap, reqId)
		receiveBuff.mu.Unlock()
	}

	if resp, error := rpcDecode(body); error != "" {
		Logger.Warn(error)
	} else {
		receiveChanMap[reqId] = make (chan interface{})
		receiveChanMap[reqId]<-resp
	}
}