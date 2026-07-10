package api

import (
	"github.com/gin-gonic/gin"
	"github.com/shelly-app/shelly/internal/middleware"
)

// SetupRouter configures all API routes
func SetupRouter(r *gin.Engine) {
	// Init components
	InitWSHub()
	
	// Public routes
	api := r.Group("/api")
	{
		api.POST("/auth/register", Register)
		api.POST("/auth/login", Login)
	}

	// Protected routes
	auth := api.Group("")
	auth.Use(middleware.AuthRequired())
	{
		// Profile
		auth.GET("/profile", GetProfile)
		auth.PUT("/profile/password", ChangePassword)

		// Assets
		auth.GET("/assets", ListAssets)
		auth.POST("/assets", CreateAsset)
		auth.GET("/assets/:id", GetAsset)
		auth.PUT("/assets/:id", UpdateAsset)
		auth.DELETE("/assets/:id", DeleteAsset)
		auth.POST("/assets/batch-delete", BatchDeleteAssets)

		// Asset Groups
		auth.GET("/groups", ListGroups)
		auth.POST("/groups", CreateGroup)
		auth.PUT("/groups/:id", UpdateGroup)
		auth.DELETE("/groups/:id", DeleteGroup)

		// Command Snippets
		auth.GET("/snippets", ListSnippets)
		auth.POST("/snippets", CreateSnippet)
		auth.DELETE("/snippets/:id", DeleteSnippet)

		// Highlight Rules
		auth.GET("/highlights", ListHighlightRules)
		auth.POST("/highlights", CreateHighlightRule)
		auth.DELETE("/highlights/:id", DeleteHighlightRule)

		// Terminal WebSocket
		auth.GET("/ws/terminal", ConnectTerminal)

		// SFTP
		auth.GET("/sftp/:asset_id/list", SFTPList)
		auth.POST("/sftp/:asset_id/upload", SFTPUpload)
		auth.GET("/sftp/:asset_id/download", SFTPDownload)
		auth.POST("/sftp/:asset_id/batch-download", SFTPBatchDownload)
		auth.POST("/sftp/:asset_id/mkdir", SFTPMkdir)
		auth.POST("/sftp/:asset_id/rename", SFTRename)
		auth.POST("/sftp/:asset_id/delete", SFTPDelete)

		// Batch Execution
		auth.POST("/batch/exec", BatchExec)
		auth.GET("/ws/batch-exec", BatchExecWS)

		// Port Forwarding
		auth.GET("/port-forward/rules", ListPortForwardRules)
		auth.POST("/port-forward/rules", CreatePortForwardRule)
		auth.DELETE("/port-forward/rules/:id", DeletePortForwardRule)
		auth.POST("/port-forward/rules/:id/start", StartPortForward)
		auth.POST("/port-forward/rules/:id/stop", StopPortForward)
		auth.GET("/port-forward/status", PortForwardStatus)

		// Session Recordings
		auth.GET("/sessions", ListSessions)
		auth.GET("/sessions/:id/record", GetSessionRecording)
		auth.GET("/sessions/:id/download", DownloadSession)
		auth.DELETE("/sessions/:id", DeleteSession)

		// AI Chat
		auth.POST("/ai/chat", Chat)
		auth.GET("/ai/sessions", ListChatSessions)
		auth.GET("/ai/sessions/:session_id/history", GetChatHistory)
		auth.DELETE("/ai/sessions/:session_id", DeleteChatSession)

		// Sync
		auth.GET("/sync/config", GetSyncConfig)
		auth.PUT("/sync/config", UpdateSyncConfig)
		auth.POST("/sync/trigger", TriggerSync)

		// Settings
		auth.GET("/settings", GetSettings)
		auth.PUT("/settings", UpdateSettings)

		// API Tokens (for CLI)
		auth.GET("/tokens", ListAPITokens)
		auth.POST("/tokens", CreateAPIToken)
		auth.DELETE("/tokens/:id", DeleteAPIToken)

		// App Lock
		auth.GET("/applock/status", GetAppLockStatus)
		auth.POST("/applock/set", SetAppLock)
		auth.POST("/applock/unlock", UnlockApp)
		auth.POST("/applock/disable", DisableAppLock)
	}
}
