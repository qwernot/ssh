package keepalive

import (
	"log"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

// Keepalive manages SSH connection keepalive
type Keepalive struct {
	client   *gossh.Client
	interval time.Duration
	maxCount int
	stopCh   chan struct{}
}

func New(client *gossh.Client, interval time.Duration, maxCount int) *Keepalive {
	return &Keepalive{
		client:   client,
		interval: interval,
		maxCount: maxCount,
		stopCh:   make(chan struct{}),
	}
}

func (k *Keepalive) Run() {
	ticker := time.NewTicker(k.interval)
	defer ticker.Stop()

	failCount := 0
	for {
		select {
		case <-ticker.C:
			_, _, err := k.client.SendRequest("keepalive@shelly", true, nil)
			if err != nil {
				failCount++
				if failCount >= k.maxCount {
					log.Printf("SSH keepalive failed %d times, connection likely dead", failCount)
					k.client.Close()
					return
				}
			} else {
				failCount = 0
			}
		case <-k.stopCh:
			return
		}
	}
}

func (k *Keepalive) Stop() {
	close(k.stopCh)
}
