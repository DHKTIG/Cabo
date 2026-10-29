package server

import "errors"

var (
	errNotReady   = errors.New("还没有开始游戏")
	errBadRequest = errors.New("请求格式不对")
)
