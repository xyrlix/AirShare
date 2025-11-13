package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/AirShare/backend/internal/transfer"
	"github.com/AirShare/backend/pkg/models"
	"github.com/gorilla/mux"
)

// API处理函数

func (s *Server) handleGetDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.discoveryService.GetDevices()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"devices": devices,
	})
}

func (s *Server) handleSendFile(w http.ResponseWriter, r *http.Request) {
	var req models.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 验证目标设备
	if !s.discoveryService.DeviceExists(req.TargetDeviceID) {
		respondError(w, http.StatusNotFound, "Target device not found")
		return
	}

	// 开始传输
	transferID, err := s.transferService.StartTransfer(req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to start transfer")
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"transfer_id": transferID,
		"status":      "started",
	})
}

func (s *Server) handleGetTransferStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transfer_id"]

	status, err := s.transferService.GetTransferStatus(transferID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Transfer not found")
		return
	}

	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleCancelTransfer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transfer_id"]

	if err := s.transferService.CancelTransfer(transferID); err != nil {
		respondError(w, http.StatusNotFound, "Transfer not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "cancelled",
	})
}

func (s *Server) handleGetFiles(w http.ResponseWriter, r *http.Request) {
	files, err := s.transferService.ListFiles()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list files")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"files": files,
	})
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["filename"]

	filePath := filepath.Join(s.config.Storage.Directory, filename)

	// 安全检查：防止路径遍历攻击
	if !isSafePath(filePath, s.config.Storage.Directory) {
		respondError(w, http.StatusBadRequest, "Invalid filename")
		return
	}

	// 设置下载头
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, filePath)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["filename"]

	if err := s.transferService.DeleteFile(filename); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete file")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "deleted",
	})
}

// WebSocket处理函数
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级到WebSocket连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// 处理WebSocket连接
	go s.handleWebSocketConnection(conn)
}

// 辅助函数
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": message,
	})
}

func isSafePath(filePath, baseDir string) bool {
	rel, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && rel != ".." && rel[:3] != "../"
}