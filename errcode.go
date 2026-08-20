package mix

import (
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// SysErr ErrCode = -1
	Success            ErrCode = 0
	Canceled           ErrCode = 1
	Unknown            ErrCode = 2
	InvalidArgument    ErrCode = 3
	DeadlineExceeded   ErrCode = 4
	NotFound           ErrCode = 5
	AlreadyExists      ErrCode = 6
	PermissionDenied   ErrCode = 7
	ResourceExhausted  ErrCode = 8
	FailedPrecondition ErrCode = 9
	Aborted            ErrCode = 10
	OutOfRange         ErrCode = 11
	Unimplemented      ErrCode = 12
	Internal           ErrCode = 13
	Unavailable        ErrCode = 14
	DataLoss           ErrCode = 15
	Unauthenticated    ErrCode = 16
)

var codeMsgMap = map[ErrCode]string{
	Success:            "Success",
	Canceled:           "Canceled",
	Unknown:            "Unknown",
	InvalidArgument:    "InvalidArgument",
	DeadlineExceeded:   "DeadlineExceeded",
	NotFound:           "NotFound",
	AlreadyExists:      "AlreadyExists",
	PermissionDenied:   "PermissionDenied",
	ResourceExhausted:  "ResourceExhausted",
	FailedPrecondition: "FailedPrecondition",
	Aborted:            "Aborted",
	OutOfRange:         "OutOfRange",
	Unimplemented:      "Unimplemented",
	Internal:           "Internal",
	Unavailable:        "Unavailable",
	DataLoss:           "DataLoss",
	Unauthenticated:    "Unauthenticated",
}

// 不是并发安全的，在初始化的时候做
func RegisterErrCode(code ErrCode, msg string) {
	codeMsgMap[code] = msg
}

func RegisterErrCodeMap(enum map[int32]string) {
	for code, msg := range enum {
		codeMsgMap[ErrCode(code)] = msg
	}
}

type ErrCode int32

// ErrCode 布局：低两位是标准 gRPC code，高位是业务码（bizcode*100+grpcCode，
// 与 HTTP 状态码 2xx/3xx/4xx 分组同思路，十进制直读：100107 = UserErrLogin+InvalidArgument）。
// 定义错误码直接写算术：`ErrCode(bizCode)*100 + ErrCode(codes.X)`，
// 用 GRPCCode()/BizCode() 拆解。纯 gRPC 错误（业务码为 0）值即 gRPC code，
// 上方常量与既有注册行为完全兼容。
// gRPC 通道上 status.code 只放 <100 的标准枚举；业务码经 status.msg（composite
// 整值字符串）与 ErrorInfo detail（reason/metadata）传给客户端。
const grpcCodeBase int32 = 100

// GRPCCode 返回低两位的标准 gRPC code；纯 gRPC 错误即为自身。
func (x ErrCode) GRPCCode() codes.Code {
	return codes.Code(int32(x) % grpcCodeBase)
}

// BizCode 返回高位业务码；纯 gRPC 错误为 0。
func (x ErrCode) BizCode() int32 {
	return int32(x) / grpcCodeBase
}

func (x ErrCode) String() string {
	if value, ok := codeMsgMap[x]; ok {
		return value
	}
	// composite：业务码以纯值注册过（RegisterErrCodeMap 注册 proto 枚举）
	if biz := x.BizCode(); biz != 0 {
		if value, ok := codeMsgMap[ErrCode(biz)]; ok {
			return value
		}
	}
	return "Unknown Error, Code:" + strconv.Itoa(int(x))
}

func (x ErrCode) ErrResp() *ErrResp {
	return &ErrResp{Code: x, Msg: x.String()}
}

func (x ErrCode) Msg(msg string, data map[string]string) *ErrResp {
	return &ErrResp{Code: x, Msg: msg, Data: data}
}

func (x ErrCode) Wrap(err error) *ErrResp {
	if err == nil {
		return x.ErrResp()
	}
	errResp := ErrRespFrom(err)
	errResp.Code = x
	return errResp
}

func (x ErrCode) Error() string {
	return x.String()
}

// GRPCStatus 拆开上 gRPC：code 用 <100 的标准枚举（保证合法），msg 放 composite
// 整值，客户端解析数字即可还原业务码。
func (x ErrCode) GRPCStatus() *status.Status {
	return status.New(x.GRPCCode(), strconv.Itoa(int(x)))
}
