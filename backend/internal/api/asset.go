package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shelly-app/shelly/internal/database"
	"github.com/shelly-app/shelly/internal/model"
	"github.com/shelly-app/shelly/pkg/crypto"
)

var cryptoUtil *crypto.AESCrypto

func InitCrypto(hexKey string) error {
	var err error
	cryptoUtil, err = crypto.NewAESCrypto(hexKey)
	return err
}

func ListAssets(c *gin.Context) {
	userID, _ := c.Get("user_id")
	groupID := c.Query("group_id")
	search := c.Query("search")
	assetType := c.Query("type")

	query := database.DB.Where("user_id = ?", userID)
	if groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}
	if search != "" {
		query = query.Where("name LIKE ? OR host LIKE ? OR tags LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if assetType != "" {
		query = query.Where("type = ?", assetType)
	}

	var assets []model.Asset
	query.Preload("Group").Order("sort_order ASC, id DESC").Find(&assets)

	c.JSON(http.StatusOK, gin.H{"data": assets})
}

func GetAsset(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var asset model.Asset
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).Preload("Group").First(&asset).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}

	// Decrypt sensitive fields for display (mask them)
	asset.Password = "********"
	asset.PrivateKey = "********"

	c.JSON(http.StatusOK, gin.H{"data": asset})
}

func CreateAsset(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var asset model.Asset
	if err := c.ShouldBindJSON(&asset); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	asset.UserID = userID.(uint)

	// Encrypt sensitive fields
	if asset.Password != "" {
		enc, _ := cryptoUtil.Encrypt(asset.Password)
		asset.Password = enc
	}
	if asset.PrivateKey != "" {
		enc, _ := cryptoUtil.Encrypt(asset.PrivateKey)
		asset.PrivateKey = enc
	}
	if asset.Passphrase != "" {
		enc, _ := cryptoUtil.Encrypt(asset.Passphrase)
		asset.Passphrase = enc
	}

	if err := database.DB.Create(&asset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create asset: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": asset})
}

func UpdateAsset(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var asset model.Asset
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&asset).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}

	var update model.Asset
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update allowed fields
	asset.Name = update.Name
	asset.Type = update.Type
	asset.Host = update.Host
	asset.Port = update.Port
	asset.Username = update.Username
	asset.AuthType = update.AuthType
	asset.GroupID = update.GroupID
	asset.Tags = update.Tags
	asset.Note = update.Note
	asset.SortOrder = update.SortOrder
	asset.ProxyAssetID = update.ProxyAssetID
	asset.SerialPort = update.SerialPort
	asset.BaudRate = update.BaudRate
	asset.KeepaliveInterval = update.KeepaliveInterval
	asset.KeepaliveCount = update.KeepaliveCount
	asset.LegacyAlgorithms = update.LegacyAlgorithms
	asset.Encoding = update.Encoding

	// Only re-encrypt if password was changed (not masked)
	if update.Password != "" && update.Password != "********" {
		enc, _ := cryptoUtil.Encrypt(update.Password)
		asset.Password = enc
	}
	if update.PrivateKey != "" && update.PrivateKey != "********" {
		enc, _ := cryptoUtil.Encrypt(update.PrivateKey)
		asset.PrivateKey = enc
	}
	if update.Passphrase != "" && update.Passphrase != "********" {
		enc, _ := cryptoUtil.Encrypt(update.Passphrase)
		asset.Passphrase = enc
	}

	database.DB.Save(&asset)
	c.JSON(http.StatusOK, gin.H{"data": asset})
}

func DeleteAsset(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Asset{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete asset"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func BatchDeleteAssets(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	database.DB.Where("id IN ? AND user_id = ?", req.IDs, userID).Delete(&model.Asset{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Asset Groups
func ListGroups(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var groups []model.AssetGroup
	database.DB.Where("user_id = ?", userID).Order("sort_order ASC, id ASC").Find(&groups)
	c.JSON(http.StatusOK, gin.H{"data": groups})
}

func CreateGroup(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var group model.AssetGroup
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group.UserID = userID.(uint)
	database.DB.Create(&group)
	c.JSON(http.StatusCreated, gin.H{"data": group})
}

func UpdateGroup(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var group model.AssetGroup
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	var update model.AssetGroup
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group.Name = update.Name
	group.ParentID = update.ParentID
	group.SortOrder = update.SortOrder
	database.DB.Save(&group)

	c.JSON(http.StatusOK, gin.H{"data": group})
}

func DeleteGroup(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	// Move assets in this group to ungrouped
	database.DB.Model(&model.Asset{}).Where("group_id = ? AND user_id = ?", id, userID).Update("group_id", 0)
	database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.AssetGroup{})

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Command Snippets
func ListSnippets(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var snippets []model.CommandSnippet
	database.DB.Where("user_id = ?", userID).Order("id DESC").Find(&snippets)
	c.JSON(http.StatusOK, gin.H{"data": snippets})
}

func CreateSnippet(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var snippet model.CommandSnippet
	if err := c.ShouldBindJSON(&snippet); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snippet.UserID = userID.(uint)
	database.DB.Create(&snippet)
	c.JSON(http.StatusCreated, gin.H{"data": snippet})
}

func DeleteSnippet(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id, _ := strconv.Atoi(c.Param("id"))
	database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.CommandSnippet{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Highlight Rules
func ListHighlightRules(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var rules []model.HighlightRule
	database.DB.Where("user_id = ?", userID).Order("id ASC").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

func CreateHighlightRule(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var rule model.HighlightRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule.UserID = userID.(uint)
	database.DB.Create(&rule)
	c.JSON(http.StatusCreated, gin.H{"data": rule})
}

func DeleteHighlightRule(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id, _ := strconv.Atoi(c.Param("id"))
	database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.HighlightRule{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Decrypt helpers for internal use
func DecryptAssetCredentials(asset *model.Asset) (password, privateKey, passphrase string) {
	if asset.Password != "" {
		password, _ = cryptoUtil.Decrypt(asset.Password)
	}
	if asset.PrivateKey != "" {
		privateKey, _ = cryptoUtil.Decrypt(asset.PrivateKey)
	}
	if asset.Passphrase != "" {
		passphrase, _ = cryptoUtil.Decrypt(asset.Passphrase)
	}
	return
}

// GetDecryptedAsset returns an asset with decrypted credentials (for internal use)
func GetDecryptedAsset(assetID, userID uint) (*model.Asset, error) {
	var asset model.Asset
	if err := database.DB.Where("id = ? AND user_id = ?", assetID, userID).First(&asset).Error; err != nil {
		return nil, err
	}

	password, privateKey, passphrase := DecryptAssetCredentials(&asset)
	asset.Password = password
	asset.PrivateKey = privateKey
	asset.Passphrase = passphrase

	return &asset, nil
}
