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

func (x ErrCode) String() string {
	value, ok := codeMsgMap[x]
	if ok {
		return value
	}
	return "Unknown Error, Code:" + strconv.Itoa(int(x))
}

func (x ErrCode) ErrResp() *ErrResp {
	return &ErrResp{Code: x, Msg: x.String()}
}

func (x ErrCode) Msg(msg string) *ErrResp {
	return &ErrResp{Code: x, Msg: msg}
}

func (x ErrCode) Wrap(err error) *ErrResp {
	return &ErrResp{Code: x, Msg: err.Error()}
}

func (x ErrCode) Error() string {
	return x.String()
}

func (x ErrCode) GRPCStatus() *status.Status {
	return status.New(codes.Code(x), x.String())
}
