package model

import (
	"time"

	"gorm.io/gorm"
)

// User represents a system user
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Username  string         `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Password  string         `gorm:"size:128;not null" json:"-"`
	Role      string         `gorm:"size:16;default:viewer" json:"role"` // admin, editor, viewer
}

// AssetType defines the connection type
type AssetType string

const (
	AssetTypeSSH          AssetType = "ssh"
	AssetTypeTelnet       AssetType = "telnet"
	AssetTypeSerial       AssetType = "serial"
	AssetTypeLocal        AssetType = "local"
	AssetTypeNextTerminal AssetType = "nextterminal"
)

// AuthType defines the authentication method
type AuthType string

const (
	AuthTypePassword            AuthType = "password"
	AuthTypePrivateKey          AuthType = "private_key"
	AuthTypeKeyboardInteractive AuthType = "keyboard_interactive"
)

// Asset represents a managed connection target
type Asset struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`

	Name     string    `gorm:"size:128;not null" json:"name"`
	Type     AssetType `gorm:"size:32;not null" json:"type"`
	Host     string    `gorm:"size:255" json:"host"`
	Port     int       `gorm:"default:22" json:"port"`
	Username string    `gorm:"size:64" json:"username"`

	AuthType    AuthType `gorm:"size:32;default:password" json:"auth_type"`
	Password    string   `gorm:"size:512" json:"-"`    // encrypted
	PrivateKey  string   `gorm:"type:text" json:"-"`   // encrypted
	Passphrase  string   `gorm:"size:512" json:"-"`    // encrypted, for private key

	// Serial specific
	SerialPort string `gorm:"size:64" json:"serial_port,omitempty"` // e.g. COM3, /dev/ttyS0
	BaudRate   int    `gorm:"default:9600" json:"baud_rate,omitempty"`

	// SSH options
	KeepaliveInterval int  `gorm:"default:30" json:"keepalive_interval"`
	KeepaliveCount    int  `gorm:"default:3" json:"keepalive_count"`
	LegacyAlgorithms  bool `gorm:"default:false" json:"legacy_algorithms"`
	Encoding          string `gorm:"size:16;default:utf-8" json:"encoding"` // utf-8, gbk

	// Organization
	GroupID   *uint  `gorm:"default:null" json:"group_id"`
	Tags      string `gorm:"size:512" json:"tags"`       // comma separated
	Note      string `gorm:"type:text" json:"note"`
	SortOrder int    `gorm:"default:0" json:"sort_order"`

	// Proxy / jump host
	ProxyAssetID *uint `gorm:"default:null" json:"proxy_asset_id"`

	Group *AssetGroup `gorm:"foreignKey:GroupID" json:"group,omitempty"`
}

// AssetGroup represents a group/folder for assets
type AssetGroup struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	Name      string         `gorm:"size:128;not null" json:"name"`
	ParentID  uint           `gorm:"default:0" json:"parent_id"`
	SortOrder int            `gorm:"default:0" json:"sort_order"`
}

// CommandSnippet stores frequently used commands
type CommandSnippet struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	Title     string         `gorm:"size:128;not null" json:"title"`
	Command   string         `gorm:"type:text;not null" json:"command"`
	Tags      string         `gorm:"size:256" json:"tags"`
}

// HighlightRule defines keyword highlighting rules
type HighlightRule struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	Keyword   string         `gorm:"size:256;not null" json:"keyword"`
	Color     string         `gorm:"size:16;default:#ff0000" json:"color"`
	IsRegex   bool           `gorm:"default:false" json:"is_regex"`
	Enabled   bool           `gorm:"default:true" json:"enabled"`
}

// SessionRecord stores terminal session recordings
type SessionRecord struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	AssetID   uint           `gorm:"index" json:"asset_id"`
	AssetName string         `gorm:"size:128" json:"asset_name"`
	FilePath  string         `gorm:"size:512" json:"file_path"`
	Duration  int            `json:"duration"` // seconds
	FileSize  int64          `json:"file_size"` // bytes
	Title     string         `gorm:"size:256" json:"title"`
}

// PortForwardRule stores SSH port forwarding rules
type PortForwardRule struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	AssetID   uint           `gorm:"index;not null" json:"asset_id"`
	Name      string         `gorm:"size:128" json:"name"`
	Type      string         `gorm:"size:16;not null" json:"type"` // local, remote
	BindHost  string         `gorm:"size:255;default:127.0.0.1" json:"bind_host"`
	BindPort  int            `gorm:"not null" json:"bind_port"`
	RemoteHost string        `gorm:"size:255;default:127.0.0.1" json:"remote_host"`
	RemotePort int           `gorm:"not null" json:"remote_port"`
	Enabled   bool           `gorm:"default:true" json:"enabled"`
}

// AIChatSession stores AI conversation sessions
type AIChatSession struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	TerminalID string        `gorm:"size:64" json:"terminal_id"` // associated terminal
	Title     string         `gorm:"size:256" json:"title"`
	Model     string         `gorm:"size:64" json:"model"`
}

// AIChatMessage stores individual AI chat messages
type AIChatMessage struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	SessionID uint           `gorm:"index;not null" json:"session_id"`
	Role      string         `gorm:"size:16;not null" json:"role"` // user, assistant, system
	Content   string         `gorm:"type:text;not null" json:"content"`
}

// SyncConfig stores cloud sync configuration
type SyncConfig struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	UserID    uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	Provider  string         `gorm:"size:32;not null" json:"provider"` // local, webdav, s3, icloud
	Config    string         `gorm:"type:text" json:"config"`          // JSON encrypted
	Enabled   bool           `gorm:"default:false" json:"enabled"`
	Interval  int            `gorm:"default:300" json:"interval"`
	LastSync  *time.Time     `json:"last_sync"`
}

// APIToken for external CLI access
type APIToken struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	Name      string         `gorm:"size:128;not null" json:"name"`
	Token     string         `gorm:"uniqueIndex;size:64;not null" json:"token"`
	ExpiresAt *time.Time     `json:"expires_at"`
	LastUsed  *time.Time     `json:"last_used"`
}

// AppSettings stores user application settings
type AppSettings struct {
	ID        uint   `gorm:"primarykey" json:"id"`
	UserID    uint   `gorm:"uniqueIndex;not null" json:"user_id"`
	Settings  string `gorm:"type:text" json:"settings"` // JSON
}
