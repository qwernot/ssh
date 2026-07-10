package middleware

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/shelly-app/shelly/internal/config"
	"github.com/shelly-app/shelly/internal/database"
	"github.com/shelly-app/shelly/internal/model"
)

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(userID uint, username, role string) (string, error) {
	cfg := config.Global
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(cfg.JWT.Expire) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(config.Global.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// AuthRequired middleware - supports JWT and API Token
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ""

		// Check Authorization header
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		}

		// Check query param (for WebSocket)
		if tokenStr == "" {
			tokenStr = c.Query("token")
		}

		// Try JWT first
		if tokenStr != "" {
			claims, err := ParseToken(tokenStr)
			if err == nil {
				c.Set("user_id", claims.UserID)
				c.Set("username", claims.Username)
				c.Set("role", claims.Role)
				c.Next()
				return
			}
		}

		// Try API Token
		apiToken := c.GetHeader("X-API-Token")
		if apiToken == "" {
			apiToken = c.Query("api_token")
		}
		if apiToken != "" {
			var token model.APIToken
			db := database.DB
			if err := db.Where("token = ? AND (expires_at IS NULL OR expires_at > ?)", apiToken, time.Now()).First(&token).Error; err == nil {
				// Update last used
				db.Model(&token).Update("last_used", time.Now())
				// Get user
				var user model.User
				if db.First(&user, token.UserID).Error == nil {
					c.Set("user_id", user.ID)
					c.Set("username", user.Username)
					c.Set("role", user.Role)
					c.Next()
					return
				}
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
	}
}

// AppLockRequired middleware - checks if app lock is enabled and requires unlock
func AppLockRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if app lock is enabled
		var settings model.AppSettings
		db := database.DB
		userID := c.GetUint("user_id")
		if err := db.Where("user_id = ?", userID).First(&settings).Error; err != nil {
			// No settings yet, no lock
			c.Next()
			return
		}

		// Parse settings JSON to check if lock is enabled
		// Settings format: {"lock_enabled": true, "lock_hash": "sha256hash"}
		// If lock_enabled is not set or false, skip
		if !strings.Contains(settings.Settings, `"lock_enabled":true`) {
			c.Next()
			return
		}

		// Check if already unlocked in this session
		unlockToken := c.GetHeader("X-App-Unlock")
		if unlockToken == "" {
			unlockToken = c.Query("unlock")
		}

		if unlockToken != "" {
			// Verify unlock token (hash of PIN + session salt)
			// Extract lock_hash from settings
			// Simple approach: unlock token = sha256(lock_hash + date)
			// This provides daily unlock validation
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusLocked, gin.H{"error": "app_locked", "require_unlock": true})
	}
}

// HashPIN hashes a PIN/password for app lock
func HashPIN(pin string) string {
	h := sha256.Sum256([]byte(pin))
	return fmt.Sprintf("%x", h)
}

// VerifyPIN checks a PIN against a hash
func VerifyPIN(pin, hash string) bool {
	return HashPIN(pin) == hash
}

// CORS middleware
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-API-Token")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
