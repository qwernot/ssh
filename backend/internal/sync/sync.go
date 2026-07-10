package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/shelly-app/shelly/internal/database"
	"github.com/shelly-app/shelly/internal/model"
	pkgcrypto "github.com/shelly-app/shelly/pkg/crypto"
)

// Syncer is the interface for cloud sync providers
type Syncer interface {
	Upload(ctx context.Context, key string, data []byte) error
	Download(ctx context.Context, key string) ([]byte, error)
	List(ctx context.Context, prefix string) ([]string, error)
	Delete(ctx context.Context, key string) error
}

// SyncEngine manages data synchronization
type SyncEngine struct {
	provider Syncer
	crypto   *pkgcrypto.AESCrypto
	interval time.Duration
	userID   uint
}

func NewSyncEngine(provider Syncer, cryptoKey string, interval time.Duration, userID uint) (*SyncEngine, error) {
	crypto, err := pkgcrypto.NewAESCrypto(cryptoKey)
	if err != nil {
		return nil, err
	}
	return &SyncEngine{provider: provider, crypto: crypto, interval: interval, userID: userID}, nil
}

// SyncData exports and syncs data to cloud
func (e *SyncEngine) SyncData(ctx context.Context) error {
	var assets []model.Asset
	database.DB.Where("user_id = ?", e.userID).Find(&assets)
	var snippets []model.CommandSnippet
	database.DB.Where("user_id = ?", e.userID).Find(&snippets)
	var rules []model.HighlightRule
	database.DB.Where("user_id = ?", e.userID).Find(&rules)
	var settings model.AppSettings
	database.DB.Where("user_id = ?", e.userID).First(&settings)

	data := map[string]interface{}{
		"version": 1, "timestamp": time.Now().Unix(),
		"assets": assets, "snippets": snippets, "highlights": rules, "settings": settings.Settings,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal sync data: %w", err)
	}
	encrypted, err := e.crypto.Encrypt(string(jsonData))
	if err != nil {
		return fmt.Errorf("encrypt sync data: %w", err)
	}
	return e.provider.Upload(ctx, "shelly_backup.enc", []byte(encrypted))
}

// RestoreData downloads and restores data from cloud
func (e *SyncEngine) RestoreData(ctx context.Context) error {
	encrypted, err := e.provider.Download(ctx, "shelly_backup.enc")
	if err != nil {
		return fmt.Errorf("download sync data: %w", err)
	}
	decrypted, err := e.crypto.Decrypt(string(encrypted))
	if err != nil {
		return fmt.Errorf("decrypt sync data: %w", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(decrypted), &data); err != nil {
		return fmt.Errorf("unmarshal sync data: %w", err)
	}
	// TODO: Import assets, snippets, highlights, settings
	return nil
}

// StartAutoSync begins periodic sync
func (e *SyncEngine) StartAutoSync(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.SyncData(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// --- Local Syncer ---
type LocalSyncer struct{ dir string }

func NewLocalSyncer(dir string) *LocalSyncer {
	os.MkdirAll(dir, 0755)
	return &LocalSyncer{dir: dir}
}
func (l *LocalSyncer) Upload(ctx context.Context, key string, data []byte) error {
	return os.WriteFile(filepath.Join(l.dir, key), data, 0644)
}
func (l *LocalSyncer) Download(ctx context.Context, key string) ([]byte, error) {
	return os.ReadFile(filepath.Join(l.dir, key))
}
func (l *LocalSyncer) List(ctx context.Context, prefix string) ([]string, error) {
	var files []string
	filepath.Walk(l.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(l.dir, path)
		files = append(files, rel)
		return nil
	})
	return files, nil
}
func (l *LocalSyncer) Delete(ctx context.Context, key string) error {
	return os.Remove(filepath.Join(l.dir, key))
}

// --- WebDAV Syncer ---
type WebDAVSyncer struct {
	url, username, password string
	client                  *http.Client
}

func NewWebDAVSyncer(url, username, password string) *WebDAVSyncer {
	return &WebDAVSyncer{url: url, username: username, password: password, client: &http.Client{Timeout: 60 * time.Second}}
}
func (w *WebDAVSyncer) Upload(ctx context.Context, key string, data []byte) error {
	req, _ := http.NewRequestWithContext(ctx, "PUT", w.url+"/"+key, bytesReader(data))
	req.SetBasicAuth(w.username, w.password)
	req.ContentLength = int64(len(data))
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webdav upload: status %d", resp.StatusCode)
	}
	return nil
}
func (w *WebDAVSyncer) Download(ctx context.Context, key string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", w.url+"/"+key, nil)
	req.SetBasicAuth(w.username, w.password)
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
func (w *WebDAVSyncer) List(ctx context.Context, prefix string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, "PROPFIND", w.url+"/", nil)
	req.SetBasicAuth(w.username, w.password)
	req.Header.Set("Depth", "1")
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return []string{}, nil
}
func (w *WebDAVSyncer) Delete(ctx context.Context, key string) error {
	req, _ := http.NewRequestWithContext(ctx, "DELETE", w.url+"/"+key, nil)
	req.SetBasicAuth(w.username, w.password)
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// --- S3 Syncer ---
type S3Syncer struct {
	endpoint, bucket, accessKey, secretKey, region string
	client                                         *http.Client
}

func NewS3Syncer(endpoint, bucket, accessKey, secretKey, region string) *S3Syncer {
	return &S3Syncer{endpoint: endpoint, bucket: bucket, accessKey: accessKey, secretKey: secretKey, region: region, client: &http.Client{Timeout: 120 * time.Second}}
}
func (s *S3Syncer) Upload(ctx context.Context, key string, data []byte) error {
	req, _ := http.NewRequestWithContext(ctx, "PUT", fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key), bytesReader(data))
	req.SetBasicAuth(s.accessKey, s.secretKey)
	req.ContentLength = int64(len(data))
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 upload: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
func (s *S3Syncer) Download(ctx context.Context, key string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key), nil)
	req.SetBasicAuth(s.accessKey, s.secretKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("s3 download: status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
func (s *S3Syncer) List(ctx context.Context, prefix string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/%s?prefix=%s", s.endpoint, s.bucket, prefix), nil)
	req.SetBasicAuth(s.accessKey, s.secretKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return []string{}, nil
}
func (s *S3Syncer) Delete(ctx context.Context, key string) error {
	req, _ := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key), nil)
	req.SetBasicAuth(s.accessKey, s.secretKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// --- iCloud Syncer ---
type ICloudSyncer struct{ localPath string }

func NewICloudSyncer() *ICloudSyncer {
	homeDir, _ := os.UserHomeDir()
	p := filepath.Join(homeDir, "Library", "Mobile Documents", "com~apple~CloudDocs", "shelly")
	os.MkdirAll(p, 0755)
	return &ICloudSyncer{localPath: p}
}
func (i *ICloudSyncer) Upload(ctx context.Context, key string, data []byte) error {
	return os.WriteFile(filepath.Join(i.localPath, key), data, 0644)
}
func (i *ICloudSyncer) Download(ctx context.Context, key string) ([]byte, error) {
	return os.ReadFile(filepath.Join(i.localPath, key))
}
func (i *ICloudSyncer) List(ctx context.Context, prefix string) ([]string, error) {
	var files []string
	filepath.Walk(i.localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(i.localPath, path)
		files = append(files, rel)
		return nil
	})
	return files, nil
}
func (i *ICloudSyncer) Delete(ctx context.Context, key string) error {
	return os.Remove(filepath.Join(i.localPath, key))
}

// --- Helpers ---
func bytesReader(data []byte) io.Reader {
	return &bytesReaderImpl{data: data, pos: 0}
}

type bytesReaderImpl struct {
	data []byte
	pos  int
}

func (r *bytesReaderImpl) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// NewSyncerFromConfig creates a Syncer from provider name and JSON config
func NewSyncerFromConfig(provider, configJSON string) (Syncer, error) {
	var cfg map[string]string
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("parse sync config: %w", err)
	}
	switch provider {
	case "local":
		return NewLocalSyncer(cfg["path"]), nil
	case "webdav":
		return NewWebDAVSyncer(cfg["url"], cfg["username"], cfg["password"]), nil
	case "s3":
		return NewS3Syncer(cfg["endpoint"], cfg["bucket"], cfg["access_key"], cfg["secret_key"], cfg["region"]), nil
	case "icloud":
		return NewICloudSyncer(), nil
	default:
		return nil, fmt.Errorf("unknown sync provider: %s", provider)
	}
}
