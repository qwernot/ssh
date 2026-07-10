package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shelly-app/shelly/internal/database"
	"github.com/shelly-app/shelly/internal/model"
	"github.com/shelly-app/shelly/internal/ssh"
	"github.com/shelly-app/shelly/internal/websocket"
	"github.com/shelly-app/shelly/pkg/asciicast"
)

var wsHub *websocket.Hub

func InitWSHub() *websocket.Hub {
	wsHub = websocket.NewHub()
	return wsHub
}

// ConnectTerminal establishes a terminal WebSocket connection
func ConnectTerminal(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assetIDStr := c.Query("asset_id")
	assetID, _ := strconv.Atoi(assetIDStr)

	if assetID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset_id required"})
		return
	}

	asset, err := GetDecryptedAsset(uint(assetID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}

	termID := uuid.New().String()

	wsHub.HandleWebSocket(c, termID, func(session *websocket.TerminalSession) error {
		var sshClient *ssh.SSHClient

		// Check for proxy/jump host
		if asset.ProxyAssetID != nil && *asset.ProxyAssetID > 0 {
			proxyAsset, err := GetDecryptedAsset(*asset.ProxyAssetID, userID.(uint))
			if err != nil {
				return fmt.Errorf("proxy asset not found: %w", err)
			}
			sshClient, err = ssh.ConnectViaProxy(asset, proxyAsset)
			if err != nil {
				return err
			}
		} else {
			sshClient, err = ssh.ConnectSSH(asset)
			if err != nil {
				return err
			}
		}

		// Setup session recording
		recorder := asciicast.NewRecorder(80, 24, asset.Name)

		// Input handler
		session.OnInput = func(data []byte) {
			sshClient.Write(data)
			recorder.RecordInput(data)
		}

		// Resize handler
		session.OnResize = func(cols, rows uint16) {
			sshClient.Resize(cols, rows)
			recorder.Resize(cols, rows)
		}

		// Close handler
		session.OnClose = func() {
			sshClient.Close()
			// Save recording
			recording := recorder.Stop()
			if recording != nil && len(recording.Events) > 0 {
				saveRecording(userID.(uint), uint(assetID), asset.Name, recording)
			}
		}

		// Read stdout in background
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := sshClient.Stdout.Read(buf)
				if n > 0 {
					output := make([]byte, n)
					copy(output, buf[:n])
					session.WriteOutput(output)
					recorder.RecordOutput(output)
				}
				if err != nil {
					if err != io.EOF {
						log.Printf("SSH read error: %v", err)
					}
					session.Close()
					return
				}
			}
		}()

		// Read stderr in background
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := sshClient.Stderr.Read(buf)
				if n > 0 {
					output := make([]byte, n)
					copy(output, buf[:n])
					session.WriteOutput(output)
					recorder.RecordOutput(output)
				}
				if err != nil {
					return
				}
			}
		}()

		// Wait for SSH session to end
		go func() {
			sshClient.Wait()
			session.Close()
		}()

		return nil
	})
}

func saveRecording(userID, assetID uint, assetName string, recording *asciicast.Recording) {
	rec := model.SessionRecord{
		UserID:    userID,
		AssetID:   assetID,
		AssetName: assetName,
		Title:     fmt.Sprintf("%s - %s", assetName, time.Now().Format("2006-01-02 15:04:05")),
		Duration:  int(recording.Duration.Seconds()),
	}

	data, err := json.Marshal(recording)
	if err != nil {
		log.Printf("marshal recording: %v", err)
		return
	}

	rec.FileSize = int64(len(data))
	filePath := fmt.Sprintf("data/recordings/%d_%s.cast", time.Now().UnixMilli(), uuid.New().String()[:8])
	rec.FilePath = filePath

	if err := database.DB.Create(&rec).Error; err != nil {
		log.Printf("save recording: %v", err)
		return
	}

	dir := filepath.Dir(filePath)
	os.MkdirAll(dir, 0755)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("write recording file: %v", err)
	}
}

// ListSessions returns session recordings
func ListSessions(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var records []model.SessionRecord
	database.DB.Where("user_id = ?", userID).Order("id DESC").Find(&records)
	c.JSON(http.StatusOK, gin.H{"data": records})
}

// GetSessionRecording returns a session recording file
func GetSessionRecording(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var record model.SessionRecord
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&record).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording not found"})
		return
	}

	c.File(record.FilePath)
}

// DownloadSession downloads a recording file
func DownloadSession(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var record model.SessionRecord
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&record).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording not found"})
		return
	}

	c.FileAttachment(record.FilePath, record.Title+".cast")
}

// DeleteSession deletes a recording
func DeleteSession(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")

	var record model.SessionRecord
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&record).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "recording not found"})
		return
	}

	database.DB.Delete(&record)
	os.Remove(record.FilePath)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
