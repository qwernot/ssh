package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shelly-app/shelly/internal/config"
	"github.com/shelly-app/shelly/internal/database"
	"github.com/shelly-app/shelly/internal/middleware"
	"github.com/shelly-app/shelly/internal/model"
	syncpkg "github.com/shelly-app/shelly/internal/sync"
)

type ChatRequest struct {
	SessionID uint   `json:"session_id"`
	Message   string `json:"message" binding:"required"`
	Model     string `json:"model,omitempty"`
	Context   string `json:"context,omitempty"` // terminal context
}

type ChatResponse struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

// Chat handles AI chat with streaming (SSE)
func Chat(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get or create session
	var session model.AIChatSession
	if req.SessionID > 0 {
		database.DB.Where("id = ? AND user_id = ?", req.SessionID, userID).First(&session)
	}
	if session.ID == 0 {
		session = model.AIChatSession{
			UserID:     userID.(uint),
			TerminalID: "",
			Title:      truncate(req.Message, 50),
			Model:      config.Global.AI.Model,
		}
		database.DB.Create(&session)
	}

	// Save user message
	userMsg := model.AIChatMessage{
		SessionID: session.ID,
		Role:      "user",
		Content:   req.Message,
	}
	database.DB.Create(&userMsg)

	// Get conversation history
	var messages []model.AIChatMessage
	database.DB.Where("session_id = ?", session.ID).Order("id ASC").Find(&messages)

	// Build messages for API
	var apiMessages []map[string]string

	// Add terminal context as system message
	if req.Context != "" {
		apiMessages = append(apiMessages, map[string]string{
			"role":    "system",
			"content": fmt.Sprintf("You are an AI assistant helping with terminal operations. Current terminal context:\n```\n%s\n```", req.Context),
		})
	}

	apiMessages = append(apiMessages, map[string]string{
		"role":    "system",
		"content": "You are Shelly AI, a helpful terminal assistant. Help users with commands, troubleshooting, and system administration. When suggesting commands, wrap them in code blocks. Always be concise and accurate.",
	})

	for _, msg := range messages {
		apiMessages = append(apiMessages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	modelName := req.Model
	if modelName == "" {
		modelName = session.Model
	}

	// Setup SSE streaming
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// Call AI API
	var fullResponse string
	err := streamAIResponse(apiMessages, modelName, func(chunk string) {
		fullResponse += chunk
		data, _ := json.Marshal(ChatResponse{Content: chunk, Done: false})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	})

	if err != nil {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Save assistant response
	assistantMsg := model.AIChatMessage{
		SessionID: session.ID,
		Role:      "assistant",
		Content:   fullResponse,
	}
	database.DB.Create(&assistantMsg)

	// Send done
	data, _ := json.Marshal(ChatResponse{Done: true})
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	flusher.Flush()
}

func streamAIResponse(messages []map[string]string, model string, onChunk func(string)) error {
	cfg := config.Global.AI

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 6 || line[:6] != "data: " {
			continue
		}

		data := line[6:]
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if json.Unmarshal([]byte(data), &chunk) == nil {
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				onChunk(chunk.Choices[0].Delta.Content)
			}
		}
	}

	return nil
}

// AI Session management
func ListChatSessions(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var sessions []model.AIChatSession
	database.DB.Where("user_id = ?", userID).Order("id DESC").Find(&sessions)
	c.JSON(http.StatusOK, gin.H{"data": sessions})
}

func GetChatHistory(c *gin.Context) {
	userID, _ := c.Get("user_id")
	sessionID, _ := strconv.Atoi(c.Param("session_id"))

	// Verify session ownership
	var session model.AIChatSession
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	var messages []model.AIChatMessage
	database.DB.Where("session_id = ?", sessionID).Order("id ASC").Find(&messages)

	c.JSON(http.StatusOK, gin.H{"data": messages})
}

func DeleteChatSession(c *gin.Context) {
	userID, _ := c.Get("user_id")
	sessionID := c.Param("session_id")

	database.DB.Where("session_id = ? AND user_id IN (?)", sessionID,
		database.DB.Model(&model.AIChatSession{}).Select("id").Where("id = ? AND user_id = ?", sessionID, userID),
	).Delete(&model.AIChatMessage{})

	database.DB.Where("id = ? AND user_id = ?", sessionID, userID).Delete(&model.AIChatSession{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Sync API
func GetSyncConfig(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var cfg model.SyncConfig
	database.DB.Where("user_id = ?", userID).First(&cfg)
	c.JSON(http.StatusOK, gin.H{"data": cfg})
}

func UpdateSyncConfig(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var cfg model.SyncConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg.UserID = userID.(uint)
	now := time.Now()

	var existing model.SyncConfig
	if err := database.DB.Where("user_id = ?", userID).First(&existing).Error; err != nil {
		database.DB.Create(&cfg)
	} else {
		cfg.ID = existing.ID
		cfg.LastSync = &now
		database.DB.Save(&cfg)
	}

	c.JSON(http.StatusOK, gin.H{"data": cfg})
}

func TriggerSync(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var cfg model.SyncConfig
	if err := database.DB.Where("user_id = ?", userID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sync not configured"})
		return
	}
	if !cfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sync not enabled"})
		return
	}

	provider, err := syncpkg.NewSyncerFromConfig(cfg.Provider, cfg.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cryptoKey := config.Global.Crypto.Key
	engine, err := syncpkg.NewSyncEngine(provider, cryptoKey, time.Duration(cfg.Interval)*time.Second, userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := engine.SyncData(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	cfg.LastSync = &now
	database.DB.Save(&cfg)

	c.JSON(http.StatusOK, gin.H{"message": "sync completed"})
}

// Settings API
func GetSettings(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var settings model.AppSettings
	database.DB.Where("user_id = ?", userID).First(&settings)

	if settings.ID == 0 {
		c.JSON(http.StatusOK, gin.H{"data": map[string]interface{}{}})
		return
	}

	var parsed map[string]interface{}
	json.Unmarshal([]byte(settings.Settings), &parsed)
	c.JSON(http.StatusOK, gin.H{"data": parsed})
}

func UpdateSettings(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jsonData, _ := json.Marshal(data)

	var settings model.AppSettings
	if err := database.DB.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		settings = model.AppSettings{
			UserID:   userID.(uint),
			Settings: string(jsonData),
		}
		database.DB.Create(&settings)
	} else {
		settings.Settings = string(jsonData)
		database.DB.Save(&settings)
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// App Lock API
func GetAppLockStatus(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var settings model.AppSettings
	err := database.DB.Where("user_id = ?", userID).First(&settings).Error

	if err != nil || settings.ID == 0 {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}

	var parsed map[string]interface{}
	json.Unmarshal([]byte(settings.Settings), &parsed)

	enabled, _ := parsed["lock_enabled"].(bool)
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

func SetAppLock(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		PIN string `json:"pin" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PIN required"})
		return
	}

	// Hash the PIN
	hash := middleware.HashPIN(req.PIN)

	// Get or create settings
	var settings model.AppSettings
	var parsed map[string]interface{}
	if err := database.DB.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		parsed = make(map[string]interface{})
	} else {
		json.Unmarshal([]byte(settings.Settings), &parsed)
	}

	parsed["lock_enabled"] = true
	parsed["lock_hash"] = hash
	jsonData, _ := json.Marshal(parsed)

	if settings.ID == 0 {
		settings = model.AppSettings{
			UserID:   userID.(uint),
			Settings: string(jsonData),
		}
		database.DB.Create(&settings)
	} else {
		settings.Settings = string(jsonData)
		database.DB.Save(&settings)
	}

	c.JSON(http.StatusOK, gin.H{"message": "app lock enabled"})
}

func UnlockApp(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		PIN string `json:"pin" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PIN required"})
		return
	}

	var settings model.AppSettings
	if err := database.DB.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no lock configured"})
		return
	}

	var parsed map[string]interface{}
	json.Unmarshal([]byte(settings.Settings), &parsed)
	hash, _ := parsed["lock_hash"].(string)

	if !middleware.VerifyPIN(req.PIN, hash) {
		c.JSON(http.StatusForbidden, gin.H{"error": "incorrect PIN"})
		return
	}

	// Generate unlock token (valid for this session)
	unlockToken := middleware.HashPIN(hash + time.Now().Format("2006-01-02"))
	c.JSON(http.StatusOK, gin.H{"unlock_token": unlockToken})
}

func DisableAppLock(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		PIN string `json:"pin" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PIN required"})
		return
	}

	var settings model.AppSettings
	if err := database.DB.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no lock configured"})
		return
	}

	var parsed map[string]interface{}
	json.Unmarshal([]byte(settings.Settings), &parsed)
	hash, _ := parsed["lock_hash"].(string)

	if !middleware.VerifyPIN(req.PIN, hash) {
		c.JSON(http.StatusForbidden, gin.H{"error": "incorrect PIN"})
		return
	}

	parsed["lock_enabled"] = false
	delete(parsed, "lock_hash")
	jsonData, _ := json.Marshal(parsed)
	settings.Settings = string(jsonData)
	database.DB.Save(&settings)

	c.JSON(http.StatusOK, gin.H{"message": "app lock disabled"})
}
