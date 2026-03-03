package core

import "fmt"

// ErrCode 统一错误码
type ErrCode int

const (
	ErrInternal      ErrCode = 5000
	ErrK8sConnection ErrCode = 5001
	ErrK8sAPICall    ErrCode = 5002
	ErrDBConnection  ErrCode = 5003
	ErrDBQuery       ErrCode = 5004
	ErrConfigLoad    ErrCode = 5005
	ErrCollectFailed ErrCode = 5006
)

// AppError 应用统一错误类型
type AppError struct {
	Code    ErrCode
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewError 创建应用错误
func NewError(code ErrCode, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

// WrapK8sError 包装 K8s API 调用错误
func WrapK8sError(operation string, err error) *AppError {
	return NewError(ErrK8sAPICall, fmt.Sprintf("K8s API 调用失败: %s", operation), err)
}
