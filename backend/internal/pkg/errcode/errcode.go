package errcode

// 业务错误码（10000+）
const (
	ErrUserExists    = 10001 // 用户名已存在
	ErrUserNotFound  = 10002 // 用户不存在
	ErrPasswordWrong = 10003 // 密码错误
	ErrInvalidToken  = 10004 // 令牌无效或过期
	ErrParamInvalid  = 10005 // 参数错误
)

// 错误码对应的中文消息
var codeMsg = map[int]string{
	ErrUserExists:    "用户名已存在",
	ErrUserNotFound:  "用户不存在",
	ErrPasswordWrong: "用户名或密码错误",
	ErrInvalidToken:  "令牌无效或已过期",
	ErrParamInvalid:  "参数错误",
}

// GetMsg 根据错误码获取消息
func GetMsg(code int) string {
	if msg, ok := codeMsg[code]; ok {
		return msg
	}
	return "未知错误"
}
