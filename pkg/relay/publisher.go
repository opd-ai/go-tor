// Package relay implements Tor relay (bridge/non-exit) functionality.
package relay

import (
	"bytes"
	"context"
	"crypto/rsa"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// BridgeAuthority represents a bridge authority endpoint
type BridgeAuthority struct {
	Address string // IP:Port or hostname:port
	URL     string // HTTP URL for descriptor uploads (e.g., "http://authority/tor/")
}

// DefaultBridgeAuthorities returns the default Tor bridge authorities
// Reference: https://gitlab.torproject.org/tpo/core/tor/-/blob/HEAD/src/app/config/auth_dirs.inc
var DefaultBridgeAuthorities = []BridgeAuthority{
	{
		Address: "86.59.21.38:80",
		URL:     "http://86.59.21.38/tor/",
	},
}

// DescriptorPublisher handles publishing server descriptors to bridge authorities
type DescriptorPublisher struct {
	authorities  []BridgeAuthority
	httpClient   *http.Client
	logger       *logger.Logger
	publishMu    sync.Mutex
	lastPublish  time.Time
	publishCount int64
}

// PublisherConfig configures the descriptor publisher
type PublisherConfig struct {
	Authorities     []BridgeAuthority // Bridge authorities to publish to
	PublishInterval time.Duration     // Interval between automatic publishes (default: 18h)
	HTTPTimeout     time.Duration     // HTTP request timeout (default: 30s)
	RetryAttempts   int               // Number of retry attempts per authority (default: 3)
	RetryDelay      time.Duration     // Initial retry delay with exponential backoff (default: 5s)
	MaxRetryDelay   time.Duration     // Maximum retry delay (default: 60s)
}

// DefaultPublisherConfig returns the default publisher configuration
func DefaultPublisherConfig() PublisherConfig {
	return PublisherConfig{
		Authorities:     DefaultBridgeAuthorities,
		PublishInterval: 18 * time.Hour, // Tor default: refresh every 18 hours
		HTTPTimeout:     30 * time.Second,
		RetryAttempts:   3,
		RetryDelay:      5 * time.Second,
		MaxRetryDelay:   60 * time.Second,
	}
}

// NewDescriptorPublisher creates a new descriptor publisher
func NewDescriptorPublisher(config PublisherConfig, log *logger.Logger) *DescriptorPublisher {
	if len(config.Authorities) == 0 {
		config.Authorities = DefaultBridgeAuthorities
	}
	if config.PublishInterval == 0 {
		config.PublishInterval = 18 * time.Hour
	}
	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 30 * time.Second
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 5 * time.Second
	}
	if config.MaxRetryDelay == 0 {
		config.MaxRetryDelay = 60 * time.Second
	}

	return &DescriptorPublisher{
		authorities: config.Authorities,
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: log,
	}
}

// PublishDescriptor publishes a server descriptor to bridge authorities
// Returns the number of successful publications
func (p *DescriptorPublisher) PublishDescriptor(ctx context.Context, descriptor *ServerDescriptor) (int, error) {
	p.publishMu.Lock()
	defer p.publishMu.Unlock()

	if descriptor == nil {
		return 0, fmt.Errorf("descriptor is nil")
	}

	if len(descriptor.RawDescriptor) == 0 {
		return 0, fmt.Errorf("descriptor.RawDescriptor is empty")
	}

	p.logger.Info("publishing server descriptor to bridge authorities",
		"nickname", descriptor.Nickname,
		"address", descriptor.Address,
		"authorities", len(p.authorities))

	successCount := 0
	var lastErr error

	for _, auth := range p.authorities {
		err := p.publishToAuthority(ctx, auth, descriptor.RawDescriptor)
		if err != nil {
			p.logger.Warn("failed to publish to authority",
				"authority", auth.Address,
				"error", err)
			lastErr = err
			continue
		}
		successCount++
		p.logger.Info("successfully published descriptor",
			"authority", auth.Address)
	}

	p.lastPublish = time.Now()
	p.publishCount++

	if successCount == 0 {
		return 0, fmt.Errorf("failed to publish to any authority: %w", lastErr)
	}

	p.logger.Info("descriptor published",
		"successful", successCount,
		"total", len(p.authorities))

	return successCount, nil
}

// publishToAuthority publishes to a single authority with retries
func (p *DescriptorPublisher) publishToAuthority(ctx context.Context, auth BridgeAuthority, descriptorData []byte) error {
	var lastErr error
	retryDelay := 5 * time.Second

	for attempt := 0; attempt <= 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
				// Exponential backoff with cap
				retryDelay *= 2
				if retryDelay > 60*time.Second {
					retryDelay = 60 * time.Second
				}
			}
		}

		err := p.uploadDescriptor(ctx, auth, descriptorData)
		if err == nil {
			return nil
		}

		lastErr = err
		p.logger.Debug("upload attempt failed",
			"authority", auth.Address,
			"attempt", attempt+1,
			"error", err)
	}

	return fmt.Errorf("all upload attempts failed: %w", lastErr)
}

// uploadDescriptor performs a single HTTP POST upload
func (p *DescriptorPublisher) uploadDescriptor(ctx context.Context, auth BridgeAuthority, descriptorData []byte) error {
	// Bridge descriptors are posted to /tor/ endpoint
	url := auth.URL
	if url == "" {
		url = fmt.Sprintf("http://%s/tor/", auth.Address)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(descriptorData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set Content-Type per dir-spec.txt §4.3
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "go-tor/0.1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	// Accept 200 OK or 202 Accepted as success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("upload failed: status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// PublishExtraInfo publishes extra-info descriptor to bridge authorities
func (p *DescriptorPublisher) PublishExtraInfo(ctx context.Context, extraInfo *ExtraInfoDescriptor) (int, error) {
	p.publishMu.Lock()
	defer p.publishMu.Unlock()

	if extraInfo == nil {
		return 0, fmt.Errorf("extra-info descriptor is nil")
	}

	if len(extraInfo.RawDescriptor) == 0 {
		return 0, fmt.Errorf("extra-info descriptor.RawDescriptor is empty")
	}

	p.logger.Info("publishing extra-info descriptor to bridge authorities",
		"nickname", extraInfo.Nickname,
		"authorities", len(p.authorities))

	successCount := 0
	var lastErr error

	for _, auth := range p.authorities {
		err := p.publishToAuthority(ctx, auth, extraInfo.RawDescriptor)
		if err != nil {
			p.logger.Warn("failed to publish extra-info to authority",
				"authority", auth.Address,
				"error", err)
			lastErr = err
			continue
		}
		successCount++
	}

	if successCount == 0 {
		return 0, fmt.Errorf("failed to publish extra-info to any authority: %w", lastErr)
	}

	return successCount, nil
}

// GetStats returns publisher statistics
func (p *DescriptorPublisher) GetStats() (lastPublish time.Time, count int64) {
	p.publishMu.Lock()
	defer p.publishMu.Unlock()
	return p.lastPublish, p.publishCount
}

// ScheduledPublisher manages automatic descriptor publishing on a schedule
type ScheduledPublisher struct {
	publisher    *DescriptorPublisher
	interval     time.Duration
	generateFunc func() (*ServerDescriptor, *ExtraInfoDescriptor, error)
	logger       *logger.Logger
	stopCh       chan struct{}
	stoppedCh    chan struct{}
	running      bool
	mu           sync.Mutex
}

// NewScheduledPublisher creates a scheduled descriptor publisher
// generateFunc should return the current server descriptor and optional extra-info descriptor
func NewScheduledPublisher(
	publisher *DescriptorPublisher,
	interval time.Duration,
	generateFunc func() (*ServerDescriptor, *ExtraInfoDescriptor, error),
	log *logger.Logger,
) *ScheduledPublisher {
	return &ScheduledPublisher{
		publisher:    publisher,
		interval:     interval,
		generateFunc: generateFunc,
		logger:       log,
		stopCh:       make(chan struct{}),
		stoppedCh:    make(chan struct{}),
	}
}

// Start begins the scheduled publishing loop
func (s *ScheduledPublisher) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduled publisher already running")
	}
	s.running = true
	s.mu.Unlock()

	go s.publishLoop(ctx)
	return nil
}

// publishLoop runs the periodic publishing
func (s *ScheduledPublisher) publishLoop(ctx context.Context) {
	defer close(s.stoppedCh)

	// Publish immediately on startup
	s.publishOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.publishOnce(ctx)
		case <-s.stopCh:
			s.logger.Info("scheduled publisher stopping")
			return
		case <-ctx.Done():
			s.logger.Info("scheduled publisher context cancelled")
			return
		}
	}
}

// publishOnce performs a single publish operation
func (s *ScheduledPublisher) publishOnce(ctx context.Context) {
	descriptor, extraInfo, err := s.generateFunc()
	if err != nil {
		s.logger.Error("failed to generate descriptors", "error", err)
		return
	}

	// Publish server descriptor
	count, err := s.publisher.PublishDescriptor(ctx, descriptor)
	if err != nil {
		s.logger.Error("failed to publish descriptor", "error", err)
	} else {
		s.logger.Info("descriptor published successfully", "authorities", count)
	}

	// Publish extra-info if available
	if extraInfo != nil {
		count, err := s.publisher.PublishExtraInfo(ctx, extraInfo)
		if err != nil {
			s.logger.Warn("failed to publish extra-info", "error", err)
		} else {
			s.logger.Info("extra-info published successfully", "authorities", count)
		}
	}
}

// Stop halts the scheduled publishing
func (s *ScheduledPublisher) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	<-s.stoppedCh
}

// RelayIdentityKeys holds the relay's cryptographic identity
type RelayIdentityKeys struct {
	RSAIdentity     *rsa.PrivateKey
	Ed25519Identity []byte // 64-byte private key
	NtorOnionKey    []byte // 32-byte Curve25519 private key
}
