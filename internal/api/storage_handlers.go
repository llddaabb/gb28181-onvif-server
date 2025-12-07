package api

import (
	"encoding/json"
	"net/http"

	"gb28181-onvif-server/internal/storage"
	"github.com/gorilla/mux"
)

// handleGetDisks 获取磁盘列表
func (s *Server) handleGetDisks(w http.ResponseWriter, r *http.Request) {
	if s.diskManager == nil {
		s.jsonError(w, http.StatusInternalServerError, "磁盘管理器未初始化")
		return
	}

	disks := s.diskManager.GetDisks()
	stats := s.diskManager.GetDiskStats()

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"disks":   disks,
		"stats":   stats,
	})
}

// handleAddDisk 添加磁盘
func (s *Server) handleAddDisk(w http.ResponseWriter, r *http.Request) {
	if s.diskManager == nil {
		s.jsonError(w, http.StatusInternalServerError, "磁盘管理器未初始化")
		return
	}

	var disk storage.Disk
	if err := json.NewDecoder(r.Body).Decode(&disk); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.diskManager.AddDisk(&disk); err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "磁盘添加成功",
		"disk":    &disk,
	})
}

// handleRemoveDisk 移除磁盘
func (s *Server) handleRemoveDisk(w http.ResponseWriter, r *http.Request) {
	if s.diskManager == nil {
		s.jsonError(w, http.StatusInternalServerError, "磁盘管理器未初始化")
		return
	}

	params := mux.Vars(r)
	diskID := params["id"]

	if err := s.diskManager.RemoveDisk(diskID); err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "磁盘移除成功",
	})
}

// handleUpdateDisk 更新磁盘信息
func (s *Server) handleUpdateDisk(w http.ResponseWriter, r *http.Request) {
	if s.diskManager == nil {
		s.jsonError(w, http.StatusInternalServerError, "磁盘管理器未初始化")
		return
	}

	params := mux.Vars(r)
	diskID := params["id"]

	var disk storage.Disk
	if err := json.NewDecoder(r.Body).Decode(&disk); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	disk.ID = diskID

	// 先移除旧的，再添加新的（更新）
	s.diskManager.RemoveDisk(diskID)
	if err := s.diskManager.AddDisk(&disk); err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "磁盘更新成功",
		"disk":    &disk,
	})
}

// handleGetRecyclePolicy 获取循环录制策略
func (s *Server) handleGetRecyclePolicy(w http.ResponseWriter, r *http.Request) {
	if s.diskManager == nil {
		s.jsonError(w, http.StatusInternalServerError, "磁盘管理器未初始化")
		return
	}

	policy := s.diskManager.GetRecyclePolicy()

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"policy":  policy,
	})
}

// handleSetRecyclePolicy 设置循环录制策略
func (s *Server) handleSetRecyclePolicy(w http.ResponseWriter, r *http.Request) {
	if s.diskManager == nil {
		s.jsonError(w, http.StatusInternalServerError, "磁盘管理器未初始化")
		return
	}

	var policy storage.RecyclePolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		s.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.diskManager.SetRecyclePolicy(&policy)

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "循环录制策略设置成功",
		"policy":  &policy,
	})
}

// handleGetDiskStats 获取磁盘统计信息
func (s *Server) handleGetDiskStats(w http.ResponseWriter, r *http.Request) {
	if s.diskManager == nil {
		s.jsonError(w, http.StatusInternalServerError, "磁盘管理器未初始化")
		return
	}

	stats := s.diskManager.GetDiskStats()

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"stats":   stats,
	})
}
