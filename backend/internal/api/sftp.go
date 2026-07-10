package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	sftppkg "github.com/pkg/sftp"
	"github.com/shelly-app/shelly/internal/database"
	"github.com/shelly-app/shelly/internal/model"
	sshpkg "github.com/shelly-app/shelly/internal/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// SFTPHandler manages SFTP operations
type SFTPHandler struct {
	client *sftppkg.Client
	ssh    *gossh.Client
}

func newSFTPFromAsset(assetID, userID uint) (*SFTPHandler, error) {
	asset, err := GetDecryptedAsset(assetID, userID)
	if err != nil {
		return nil, err
	}

	sshClient, err := sshpkg.ConnectSSH(asset)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftppkg.NewClient(sshClient.Client)
	if err != nil {
		sshClient.Close()
		return nil, err
	}

	return &SFTPHandler{client: sftpClient, ssh: sshClient.Client}, nil
}

func (h *SFTPHandler) Close() {
	h.client.Close()
}

// SFTPList lists directory contents
func SFTPList(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assetID, _ := strconv.Atoi(c.Param("asset_id"))
	path := c.DefaultQuery("path", "/")

	sftp, err := newSFTPFromAsset(uint(assetID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer sftp.Close()

	entries, err := sftp.client.ReadDir(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type FileInfo struct {
		Name  string `json:"name"`
		Size  int64  `json:"size"`
		IsDir bool   `json:"is_dir"`
		Mode  string `json:"mode"`
		ModTime int64 `json:"mod_time"`
	}

	var files []FileInfo
	for _, entry := range entries {
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Size:    entry.Size(),
			IsDir:   entry.IsDir(),
			Mode:    entry.Mode().String(),
			ModTime: entry.ModTime().Unix(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"path": path, "files": files})
}

// SFTPUpload handles file upload
func SFTPUpload(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assetID, _ := strconv.Atoi(c.Param("asset_id"))
	remotePath := c.PostForm("path")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}
	defer file.Close()

	sftp, err := newSFTPFromAsset(uint(assetID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer sftp.Close()

	destPath := filepath.Join(remotePath, header.Filename)
	dstFile, err := sftp.client.Create(destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "uploaded", "path": destPath})
}

// SFTPDownload downloads a file
func SFTPDownload(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assetID, _ := strconv.Atoi(c.Param("asset_id"))
	filePath := c.Query("path")

	sftp, err := newSFTPFromAsset(uint(assetID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer sftp.Close()

	srcFile, err := sftp.client.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer srcFile.Close()

	filename := filepath.Base(filePath)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", "application/octet-stream")
	io.Copy(c.Writer, srcFile)
}

// SFTPBatchDownload downloads multiple files as zip
func SFTPBatchDownload(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assetID, _ := strconv.Atoi(c.Param("asset_id"))

	var req struct {
		Paths []string `json:"paths" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sftp, err := newSFTPFromAsset(uint(assetID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer sftp.Close()

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="files.zip"`)

	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	for _, filePath := range req.Paths {
		srcFile, err := sftp.client.Open(filePath)
		if err != nil {
			continue
		}

		writer, err := zipWriter.Create(filepath.Base(filePath))
		if err != nil {
			srcFile.Close()
			continue
		}

		io.Copy(writer, srcFile)
		srcFile.Close()
	}
}

// SFTPMkdir creates a directory
func SFTPMkdir(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assetID, _ := strconv.Atoi(c.Param("asset_id"))

	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sftp, err := newSFTPFromAsset(uint(assetID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer sftp.Close()

	if err := sftp.client.MkdirAll(req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "created"})
}

// SFTRename renames a file/directory
func SFTRename(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assetID, _ := strconv.Atoi(c.Param("asset_id"))

	var req struct {
		OldPath string `json:"old_path" binding:"required"`
		NewPath string `json:"new_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sftp, err := newSFTPFromAsset(uint(assetID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer sftp.Close()

	if err := sftp.client.Rename(req.OldPath, req.NewPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "renamed"})
}

// SFTPDelete deletes files/directories
func SFTPDelete(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assetID, _ := strconv.Atoi(c.Param("asset_id"))

	var req struct {
		Paths []string `json:"paths" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sftp, err := newSFTPFromAsset(uint(assetID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer sftp.Close()

	for _, path := range req.Paths {
		info, err := sftp.client.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			sftp.client.RemoveDirectory(path)
		} else {
			sftp.client.Remove(path)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// BatchExec handles batch command execution
func BatchExec(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		AssetIDs []uint `json:"asset_ids" binding:"required"`
		Command  string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type Result struct {
		AssetID uint   `json:"asset_id"`
		Name    string `json:"name"`
		Output  string `json:"output"`
		Error   string `json:"error,omitempty"`
	}

	results := make(chan Result, len(req.AssetIDs))

	for _, assetID := range req.AssetIDs {
		go func(aid uint) {
			asset, err := GetDecryptedAsset(aid, userID.(uint))
			if err != nil {
				results <- Result{AssetID: aid, Error: "asset not found"}
				return
			}

			sshClient, err := sshpkg.ConnectSSH(asset)
			if err != nil {
				results <- Result{AssetID: aid, Name: asset.Name, Error: err.Error()}
				return
			}
			defer sshClient.Close()

			session := sshClient.Session
			output, err := session.CombinedOutput(req.Command)

			result := Result{
				AssetID: aid,
				Name:    asset.Name,
				Output:  string(output),
			}
			if err != nil {
				result.Error = err.Error()
			}
			results <- result
		}(assetID)
	}

	var allResults []Result
	for i := 0; i < len(req.AssetIDs); i++ {
		allResults = append(allResults, <-results)
	}

	c.JSON(http.StatusOK, gin.H{"data": allResults})
}

// Port Forward API
func ListPortForwardRules(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var rules []model.PortForwardRule
	database.DB.Where("user_id = ?", userID).Find(&rules)
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

func CreatePortForwardRule(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var rule model.PortForwardRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.UserID = userID.(uint)
	database.DB.Create(&rule)
	c.JSON(http.StatusCreated, gin.H{"data": rule})
}

func DeletePortForwardRule(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")
	database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.PortForwardRule{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func StartPortForward(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id, _ := strconv.Atoi(c.Param("id"))

	var rule model.PortForwardRule
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&rule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	// Check if already running
	_, exists := sshpkg.ActiveForwards.Get(uint(id))
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "forward already running"})
		return
	}

	asset, err := GetDecryptedAsset(rule.AssetID, userID.(uint))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}

	sshClient, err := sshpkg.ConnectSSH(asset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var pf *sshpkg.PortForward
	if rule.Type == "local" {
		pf, err = sshpkg.SetupLocalForward(sshClient.Client, rule.BindHost, rule.BindPort, rule.RemoteHost, rule.RemotePort)
	} else {
		pf, err = sshpkg.SetupRemoteForward(sshClient.Client, rule.BindHost, rule.BindPort, rule.RemoteHost, rule.RemotePort)
	}

	if err != nil {
		sshClient.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Register in active forwards
	sshpkg.ActiveForwards.Set(uint(id), pf)

	c.JSON(http.StatusOK, gin.H{"message": "started", "rule": rule})
}

func StopPortForward(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	pf, exists := sshpkg.ActiveForwards.Get(uint(id))
	if exists {
		pf.Close()
		sshpkg.ActiveForwards.Delete(uint(id))
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "forward not running"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "stopped"})
}

func PortForwardStatus(c *gin.Context) {
	status := sshpkg.GetForwardStatus()
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// API Token management
func ListAPITokens(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var tokens []model.APIToken
	database.DB.Where("user_id = ?", userID).Find(&tokens)
	c.JSON(http.StatusOK, gin.H{"data": tokens})
}

func CreateAPIToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token := &model.APIToken{
		UserID: userID.(uint),
		Name:   req.Name,
		Token:  generateAPIToken(),
	}
	database.DB.Create(token)
	c.JSON(http.StatusCreated, gin.H{"data": token})
}

func DeleteAPIToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")
	database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.APIToken{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func generateAPIToken() string {
	// Using uuid as base
	return fmt.Sprintf("shelly_%s", uuid.New().String())
}

// BatchExecWS handles batch command execution via WebSocket with real-time output
func BatchExecWS(c *gin.Context) {
	userID, _ := c.Get("user_id")

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("batch exec ws upgrade: %v", err)
		return
	}
	defer conn.Close()

	// Read init message with asset_ids and command
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, initMsg, err := conn.ReadMessage()
	if err != nil {
		conn.WriteJSON(map[string]string{"type": "error", "error": "read init: " + err.Error()})
		return
	}

	var req struct {
		AssetIDs []uint `json:"asset_ids"`
		Command  string `json:"command"`
	}
	if err := json.Unmarshal(initMsg, &req); err != nil {
		conn.WriteJSON(map[string]string{"type": "error", "error": "invalid request"})
		return
	}

	type assetResult struct {
		assetID uint
		name    string
		done    bool
		err     string
	}

	var wg sync.WaitGroup
	results := make(chan map[string]interface{}, len(req.AssetIDs)*10)

	// Send start message
	conn.WriteJSON(map[string]interface{}{
		"type":  "start",
		"total": len(req.AssetIDs),
	})

	for _, assetID := range req.AssetIDs {
		wg.Add(1)
		go func(aid uint) {
			defer wg.Done()

			asset, err := GetDecryptedAsset(aid, userID.(uint))
			if err != nil {
				results <- map[string]interface{}{
					"type": "output", "asset_id": aid, "data": "",
					"error": "asset not found", "done": true,
				}
				return
			}

			// Notify connecting
			results <- map[string]interface{}{
				"type": "connecting", "asset_id": aid, "name": asset.Name,
			}

			sshClient, err := sshpkg.ConnectSSH(asset)
			if err != nil {
				results <- map[string]interface{}{
					"type": "output", "asset_id": aid, "name": asset.Name,
					"data": "", "error": err.Error(), "done": true,
				}
				return
			}
			defer sshClient.Close()

			// Notify connected
			results <- map[string]interface{}{
				"type": "connected", "asset_id": aid, "name": asset.Name,
			}

			// Read output in real-time
			doneCh := make(chan struct{})
			go func() {
				buf := make([]byte, 4096)
				for {
					n, err := sshClient.Stdout.Read(buf)
					if n > 0 {
						results <- map[string]interface{}{
							"type": "output", "asset_id": aid, "name": asset.Name,
							"data": string(buf[:n]),
						}
					}
					if err != nil {
						break
					}
				}
				close(doneCh)
			}()

			// Send command
			if err := sshClient.Session.Start(req.Command); err != nil {
				results <- map[string]interface{}{
					"type": "output", "asset_id": aid, "name": asset.Name,
					"data": "", "error": err.Error(), "done": true,
				}
				return
			}

			// Wait for completion
			sshClient.Session.Wait()
			<-doneCh

			results <- map[string]interface{}{
				"type": "done", "asset_id": aid, "name": asset.Name,
			}
		}(assetID)
	}

	// Forward results to WebSocket
	go func() {
		wg.Wait()
		close(results)
	}()

	for msg := range results {
		if err := conn.WriteJSON(msg); err != nil {
			return
		}
	}

	// Send complete
	conn.WriteJSON(map[string]interface{}{"type": "complete"})
}
