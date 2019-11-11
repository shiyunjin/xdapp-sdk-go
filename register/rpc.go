package register

import (
	"github.com/hprose/hprose-golang/io"
	"github.com/hprose/hprose-golang/rpc"
	"reflect"
)

var (
	HproseService *rpc.TCPService		 // rpc 服务
	HproseContext *rpc.SocketContext	 // 上下文
)

func init() {
	HproseService = rpc.NewTCPService()
	HproseContext = new(rpc.SocketContext)
	HproseContext.InitServiceContext(HproseService)
}

// 屏蔽列表输出
func DoFunctionList() string {
	return "Fa{}z"
}

// 执行结果
func RpcHandle(data []byte) []byte {
	return HproseService.Handle(data, HproseContext)
}

// 已注册的rpc方法
func GetHproseAddedFunc() []string {
	return HproseService.MethodNames
}

func AddInstanceMethods(obj interface{}, namespace string) {
	HproseService.AddInstanceMethods(obj, rpc.Options{NameSpace: namespace})
}

func AddFunction(name string, function interface{}, namespace string) {
	HproseService.AddFunction(name, function, rpc.Options{NameSpace: namespace})
}

func rpcEncode(name string, args []reflect.Value) []byte {
	writer := io.NewWriter(false)
	writer.WriteByte(io.TagCall)
	writer.WriteString(name)
	writer.Reset()
	writer.WriteSlice(args)
	writer.WriteByte(io.TagEnd)
	return writer.Bytes()
}

func rpcDecode(data []byte) (interface{}, string) {
	reader := io.AcquireReader(data, false)
	defer io.ReleaseReader(reader)
	tag, _ := reader.ReadByte()
	switch tag {
	case io.TagResult:
		var e interface{}
		reader.Unserialize(&e)
		return e, ""
	case io.TagError:
		return nil, "RPC 系统调用 Agent 返回错误信息: " + reader.ReadString()
	default:
		return nil, "RPC 系统调用收到一个未定义的方法返回: " + string(tag) + reader.ReadString()
	}
}