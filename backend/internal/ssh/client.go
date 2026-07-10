package ssh

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/shelly-app/shelly/internal/model"
	"github.com/shelly-app/shelly/pkg/keepalive"
	gossh "golang.org/x/crypto/ssh"
)

// SSHClient wraps an SSH connection with a session
type SSHClient struct {
	Client   *gossh.Client
	Session  *gossh.Session
	Stdin    io.WriteCloser
	Stdout   io.Reader
	Stderr   io.Reader
	mu       sync.Mutex
	closed   bool
}

// ConnectSSH establishes an SSH connection based on asset config
func ConnectSSH(asset *model.Asset) (*SSHClient, error) {
	config := &gossh.ClientConfig{
		User:            asset.Username,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// Setup auth methods
	switch asset.AuthType {
	case model.AuthTypePassword:
		config.Auth = []gossh.AuthMethod{
			gossh.Password(asset.Password),
		}
	case model.AuthTypePrivateKey:
		signer, err := gossh.ParsePrivateKey([]byte(asset.PrivateKey))
		if err != nil {
			// Try with passphrase
			if asset.Passphrase != "" {
				signer, err = gossh.ParsePrivateKeyWithPassphrase([]byte(asset.PrivateKey), []byte(asset.Passphrase))
				if err != nil {
					return nil, fmt.Errorf("parse private key: %w", err)
				}
			} else {
				return nil, fmt.Errorf("parse private key: %w", err)
			}
		}
		config.Auth = []gossh.AuthMethod{
			gossh.PublicKeys(signer),
		}
	case model.AuthTypeKeyboardInteractive:
		config.Auth = []gossh.AuthMethod{
			gossh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = asset.Password
				}
				return answers, nil
			}),
		}
	}

	// Enable legacy algorithms if configured
	if asset.LegacyAlgorithms {
		config.SetDefaults()
		config.KeyExchanges = append(config.KeyExchanges,
			"diffie-hellman-group1-sha1",
			"diffie-hellman-group14-sha1",
		)
		config.Ciphers = append(config.Ciphers,
			"aes128-cbc",
			"aes256-cbc",
			"3des-cbc",
		)
		config.MACs = append(config.MACs,
			"hmac-sha1-96",
			"hmac-md5",
		)
	}

	addr := net.JoinHostPort(asset.Host, fmt.Sprintf("%d", asset.Port))
	client, err := gossh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("ssh session: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	// Request PTY
	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 115200,
		gossh.TTY_OP_OSPEED: 115200,
	}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("request pty: %w", err)
	}

	// Start shell
	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	sshClient := &SSHClient{
		Client:  client,
		Session: session,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}

	// Start keepalive
	if asset.KeepaliveInterval > 0 {
		ka := keepalive.New(client, time.Duration(asset.KeepaliveInterval)*time.Second, asset.KeepaliveCount)
		go ka.Run()
	}

	return sshClient, nil
}

// ConnectViaProxy establishes SSH connection through a jump host
func ConnectViaProxy(asset *model.Asset, proxyAsset *model.Asset) (*SSHClient, error) {
	// Connect to proxy first
	proxyClient, err := ConnectSSH(proxyAsset)
	if err != nil {
		return nil, fmt.Errorf("proxy connect: %w", err)
	}

	// Dial target through proxy
	addr := net.JoinHostPort(asset.Host, fmt.Sprintf("%d", asset.Port))
	conn, err := proxyClient.Client.Dial("tcp", addr)
	if err != nil {
		proxyClient.Close()
		return nil, fmt.Errorf("proxy dial: %w", err)
	}

	config := &gossh.ClientConfig{
		User:            asset.Username,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	switch asset.AuthType {
	case model.AuthTypePassword:
		config.Auth = []gossh.AuthMethod{gossh.Password(asset.Password)}
	case model.AuthTypePrivateKey:
		signer, err := gossh.ParsePrivateKey([]byte(asset.PrivateKey))
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("parse key: %w", err)
		}
		config.Auth = []gossh.AuthMethod{gossh.PublicKeys(signer)}
	}

	ncc, chans, reqs, err := gossh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("new client conn: %w", err)
	}

	client := gossh.NewClient(ncc, chans, reqs)
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("session: %w", err)
	}

	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()

	modes := gossh.TerminalModes{gossh.ECHO: 1, gossh.TTY_OP_ISPEED: 115200, gossh.TTY_OP_OSPEED: 115200}
	session.RequestPty("xterm-256color", 24, 80, modes)
	session.Shell()

	return &SSHClient{
		Client:  client,
		Session: session,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}, nil
}

func (s *SSHClient) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.EOF
	}
	return s.Stdin.Write(data)
}

func (s *SSHClient) Resize(cols, rows uint16) error {
	return s.Session.WindowChange(int(rows), int(cols))
}

func (s *SSHClient) Wait() error {
	return s.Session.Wait()
}

func (s *SSHClient) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.Session.Close()
	s.Client.Close()
}

// PortForward handles SSH port forwarding
type PortForward struct {
	Client   *gossh.Client
	Listener net.Listener
	Type     string // "local" or "remote"
	mu       sync.Mutex
	closed   bool

	// Traffic stats
	BytesSent     int64
	BytesReceived int64
	ActiveConns   int32
	TotalConns    int32
	StartedAt     time.Time
}

// ActiveForwards tracks all active port forwards (ruleID -> PortForward)
var ActiveForwards = NewForwardRegistry()

// ForwardRegistry manages active port forwards
type ForwardRegistry struct {
	sync.RWMutex
	m map[uint]*PortForward
}

func NewForwardRegistry() *ForwardRegistry {
	return &ForwardRegistry{m: make(map[uint]*PortForward)}
}

func (r *ForwardRegistry) Get(id uint) (*PortForward, bool) {
	r.RLock()
	defer r.RUnlock()
	pf, ok := r.m[id]
	return pf, ok
}

func (r *ForwardRegistry) Set(id uint, pf *PortForward) {
	r.Lock()
	defer r.Unlock()
	r.m[id] = pf
}

func (r *ForwardRegistry) Delete(id uint) {
	r.Lock()
	defer r.Unlock()
	delete(r.m, id)
}

func (r *ForwardRegistry) Keys() []uint {
	r.RLock()
	defer r.RUnlock()
	keys := make([]uint, 0, len(r.m))
	for k := range r.m {
		keys = append(keys, k)
	}
	return keys
}

// countingWriter wraps an io.Writer and counts bytes
type countingWriter struct {
	w       io.Writer
	counter *int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	*cw.counter += int64(n)
	return n, err
}

// GetForwardStatus returns the status of all active port forwards
func GetForwardStatus() map[uint]map[string]interface{} {
	ActiveForwards.RLock()
	defer ActiveForwards.RUnlock()

	result := make(map[uint]map[string]interface{})
	for id, pf := range ActiveForwards.m {
		result[id] = map[string]interface{}{
			"active_conns":   pf.ActiveConns,
			"total_conns":    pf.TotalConns,
			"bytes_sent":     pf.BytesSent,
			"bytes_received": pf.BytesReceived,
			"started_at":     pf.StartedAt,
			"type":           pf.Type,
		}
	}
	return result
}

// SetupLocalForward creates a local port forward
func SetupLocalForward(client *gossh.Client, bindAddr string, bindPort int, remoteHost string, remotePort int) (*PortForward, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(bindAddr, fmt.Sprintf("%d", bindPort)))
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	pf := &PortForward{
		Client:    client,
		Listener:  listener,
		Type:      "local",
		StartedAt: time.Now(),
	}

	go func() {
		for {
			localConn, err := listener.Accept()
			if err != nil {
				return
			}

			pf.ActiveConns++
			pf.TotalConns++

			remoteConn, err := client.Dial("tcp", net.JoinHostPort(remoteHost, fmt.Sprintf("%d", remotePort)))
			if err != nil {
				localConn.Close()
				pf.ActiveConns--
				continue
			}

			go func() {
				defer localConn.Close()
				defer remoteConn.Close()
				defer func() { pf.ActiveConns-- }()

				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					cw := &countingWriter{w: remoteConn, counter: &pf.BytesSent}
					io.Copy(cw, localConn)
				}()
				go func() {
					defer wg.Done()
					cw := &countingWriter{w: localConn, counter: &pf.BytesReceived}
					io.Copy(cw, remoteConn)
				}()
				wg.Wait()
			}()
		}
	}()

	return pf, nil
}

// SetupRemoteForward creates a remote port forward
func SetupRemoteForward(client *gossh.Client, remoteAddr string, remotePort int, localHost string, localPort int) (*PortForward, error) {
	listener, err := client.Listen("tcp", net.JoinHostPort(remoteAddr, fmt.Sprintf("%d", remotePort)))
	if err != nil {
		return nil, fmt.Errorf("remote listen: %w", err)
	}

	pf := &PortForward{
		Client:    client,
		Listener:  listener,
		Type:      "remote",
		StartedAt: time.Now(),
	}

	go func() {
		for {
			remoteConn, err := listener.Accept()
			if err != nil {
				return
			}

			pf.ActiveConns++
			pf.TotalConns++

			localConn, err := net.Dial("tcp", net.JoinHostPort(localHost, fmt.Sprintf("%d", localPort)))
			if err != nil {
				remoteConn.Close()
				pf.ActiveConns--
				continue
			}

			go func() {
				defer localConn.Close()
				defer remoteConn.Close()
				defer func() { pf.ActiveConns-- }()

				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					cw := &countingWriter{w: remoteConn, counter: &pf.BytesReceived}
					io.Copy(cw, localConn)
				}()
				go func() {
					defer wg.Done()
					cw := &countingWriter{w: localConn, counter: &pf.BytesSent}
					io.Copy(cw, remoteConn)
				}()
				wg.Wait()
			}()
		}
	}()

	return pf, nil
}

func (pf *PortForward) Close() {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	if pf.closed {
		return
	}
	pf.closed = true
	pf.Listener.Close()
}

// DetectEncoding tries to detect if output is GBK or UTF-8
func DetectEncoding(data []byte) string {
	// Simple heuristic: if there are invalid UTF-8 sequences, likely GBK
	s := string(data)
	if strings.ContainsRune(s, '\ufffd') {
		return "gbk"
	}
	return "utf-8"
}
