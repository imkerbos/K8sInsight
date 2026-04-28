package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一 API 响应结构
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// OK 200 成功
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// Created 201 创建成功
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

// BadRequest 400 请求参数错误
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    4000,
		Message: msg,
	})
}

// Unauthorized 401 未认证
func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    4010,
		Message: msg,
	})
}

// Forbidden 403 无权限
func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, Response{
		Code:    4030,
		Message: msg,
	})
}

// NotFound 404 资源不存在
func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, Response{
		Code:    4040,
		Message: msg,
	})
}

// ServerError 500 服务端错误
func ServerError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    5000,
		Message: msg,
	})
}
