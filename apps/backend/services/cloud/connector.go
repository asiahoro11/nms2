package cloud

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"management-server/config"
	"os"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Connector manages the MQTT connection between the edge agent and the cloud.
type Connector struct {
	config    config.CloudConfig
	db        *sql.DB
	client    mqtt.Client
	buffer    *Buffer
	stopCh    chan struct{}
	mu        sync.RWMutex
	running   bool
	connected bool

	// Command handler callback (set by the agent)
	OnCommand func(cmd CommandMessage)
}

// New creates a new edge connector.
func New(cfg config.CloudConfig, db *sql.DB) *Connector {
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 60
	}
	if cfg.MetricsInterval <= 0 {
		cfg.MetricsInterval = 300
	}
	if cfg.BufferMaxSize <= 0 {
		cfg.BufferMaxSize = 10000
	}

	return &Connector{
		config: cfg,
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// Start connects to the MQTT broker and begins the heartbeat loop.
func (c *Connector) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	// Initialize offline buffer
	c.buffer = NewBuffer(c.db, c.config.BufferMaxSize)
	if err := c.buffer.Init(); err != nil {
		return fmt.Errorf("init cloud buffer: %w", err)
	}

	// Configure MQTT client
	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.config.BrokerURL)
	opts.SetClientID(fmt.Sprintf("pronms-agent-%s", c.config.SiteID))
	opts.SetUsername(c.config.Username)
	opts.SetPassword(c.config.Password)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(5 * time.Minute)
	opts.SetCleanSession(false)
	opts.SetOrderMatters(false)

	// TLS configuration
	if c.config.CACert != "" {
		tlsConfig, err := c.newTLSConfig()
		if err != nil {
			return fmt.Errorf("TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsConfig)
	}

	// Last Will and Testament ??cloud knows if agent disconnects unexpectedly
	lwt := HeartbeatMessage{
		SiteID:    c.config.SiteID,
		SiteName:  c.config.SiteName,
		Timestamp: time.Now(),
	}
	lwtPayload, _ := json.Marshal(lwt)
	opts.SetWill(TopicHeartbeat(c.config.SiteID), string(lwtPayload), 1, true)

	// Connection handlers
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Printf("[Cloud] Connected to MQTT broker: %s", c.config.BrokerURL)
		c.mu.Lock()
		c.connected = true
		c.mu.Unlock()

		// Subscribe to command and config topics
		c.subscribe(client)

		// Drain offline buffer
		go c.drainBuffer()
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("[Cloud] MQTT connection lost: %v", err)
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	})

	c.client = mqtt.NewClient(opts)

	// Connect (non-blocking, will auto-reconnect)
	go func() {
		token := c.client.Connect()
		if token.WaitTimeout(15*time.Second) && token.Error() != nil {
			log.Printf("[Cloud] Initial MQTT connect failed (will retry): %v", token.Error())
		}
	}()

	c.running = true

	// Start background loops
	go c.heartbeatLoop()
	go c.metricsLoop()

	log.Printf("[Cloud] Edge Connector started (site: %s, broker: %s)", c.config.SiteID, c.config.BrokerURL)
	return nil
}

// Stop gracefully shuts down the connector.
func (c *Connector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}
	c.running = false
	close(c.stopCh)

	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(5000)
	}
	log.Println("[Cloud] Edge Connector stopped")
}

// IsConnected returns the current MQTT connection status.
func (c *Connector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// subscribe sets up cloud ??agent topic subscriptions.
func (c *Connector) subscribe(client mqtt.Client) {
	siteID := c.config.SiteID

	// Subscribe to commands
	client.Subscribe(TopicCommands(siteID), 1, func(_ mqtt.Client, msg mqtt.Message) {
		var cmd CommandMessage
		if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
			log.Printf("[Cloud] Failed to parse command: %v", err)
			return
		}
		log.Printf("[Cloud] Received command: %s (type: %s)", cmd.CommandID, cmd.CommandType)
		if c.OnCommand != nil {
			go c.OnCommand(cmd)
		}
	})

	// Subscribe to config updates
	client.Subscribe(TopicConfig(siteID), 1, func(_ mqtt.Client, msg mqtt.Message) {
		log.Printf("[Cloud] Received config update: %s", string(msg.Payload()))
		// Phase 2: Apply config changes
	})

	log.Printf("[Cloud] Subscribed to command/config topics for site %s", siteID)
}

// publish sends a message to the broker, buffering if offline.
func (c *Connector) publish(topic string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[Cloud] Marshal error for %s: %v", topic, err)
		return
	}

	c.mu.RLock()
	isConnected := c.connected
	c.mu.RUnlock()

	if isConnected && c.client != nil {
		token := c.client.Publish(topic, 1, false, data)
		if token.WaitTimeout(5*time.Second) && token.Error() != nil {
			log.Printf("[Cloud] Publish failed, buffering: %v", token.Error())
			c.buffer.Push(topic, data)
		}
	} else {
		c.buffer.Push(topic, data)
	}
}

// drainBuffer sends all buffered messages after reconnection.
func (c *Connector) drainBuffer() {
	entries, err := c.buffer.DrainAll()
	if err != nil {
		log.Printf("[Cloud] Failed to drain buffer: %v", err)
		return
	}
	if len(entries) == 0 {
		return
	}

	log.Printf("[Cloud] Draining %d buffered messages", len(entries))
	for _, e := range entries {
		c.mu.RLock()
		ok := c.connected
		c.mu.RUnlock()
		if !ok {
			log.Printf("[Cloud] Lost connection during drain, stopping")
			return
		}

		token := c.client.Publish(e.Topic, 1, false, e.Payload)
		token.WaitTimeout(5 * time.Second)
		if token.Error() != nil {
			log.Printf("[Cloud] Drain publish failed: %v", token.Error())
			return
		}
		c.buffer.MarkSent(e.ID)
		time.Sleep(50 * time.Millisecond) // Rate limit
	}
	log.Printf("[Cloud] Buffer drain complete")
}

// heartbeatLoop sends periodic heartbeats to the cloud.
func (c *Connector) heartbeatLoop() {
	// Wait for initial stabilization
	time.Sleep(5 * time.Second)

	ticker := time.NewTicker(time.Duration(c.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.PublishHeartbeat()
		}
	}
}

// metricsLoop sends periodic metric batches to the cloud.
func (c *Connector) metricsLoop() {
	time.Sleep(30 * time.Second)

	ticker := time.NewTicker(time.Duration(c.config.MetricsInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.PublishMetricBatch()
		}
	}
}

// newTLSConfig creates a TLS configuration with CA certificate.
func (c *Connector) newTLSConfig() (*tls.Config, error) {
	certPool := x509.NewCertPool()
	caCert, err := os.ReadFile(c.config.CACert)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}

	return &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}, nil
}
