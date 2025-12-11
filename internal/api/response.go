package api

import (
	"encoding/json"
	"net/http"
)

// APIResponse 统一的API响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// respondJSON 发送JSON响应
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// respondSuccess 发送成功响应
func respondSuccess(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// respondSuccessMsg 发送成功响应带消息
func respondSuccessMsg(w http.ResponseWriter, message string) {
	respondJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: message,
	})
}

// respondSuccessData 发送成功响应带数据和消息
func respondSuccessData(w http.ResponseWriter, data interface{}, message string) {
	respondJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
		Message: message,
	})
}

// respondError 发送错误响应
func respondError(w http.ResponseWriter, statusCode int, message string) {
	respondJSON(w, statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}

// respondBadRequest 发送400错误
func respondBadRequest(w http.ResponseWriter, message string) {
	respondError(w, http.StatusBadRequest, message)
}

// respondNotFound 发送404错误
func respondNotFound(w http.ResponseWriter, message string) {
	respondError(w, http.StatusNotFound, message)
}

// respondInternalError 发送500错误
func respondInternalError(w http.ResponseWriter, message string) {
	respondError(w, http.StatusInternalServerError, message)
}

// respondServiceUnavailable 发送503错误
func respondServiceUnavailable(w http.ResponseWriter, message string) {
	respondError(w, http.StatusServiceUnavailable, message)
}

// respondRaw 直接发送原始数据结构（不包装在APIResponse中）
func respondRaw(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
