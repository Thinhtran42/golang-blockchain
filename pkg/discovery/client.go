package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// NodeInfo represents node information for registration
type NodeInfo struct {
	ID         string `json:"id"`
	Address    string `json:"address"`
	Port       string `json:"port"`
	IsLeader   bool   `json:"is_leader"`
	Height     int32  `json:"height"`
	TxPoolSize int32  `json:"tx_pool_size"`
}

// Client handles communication with the registry service
type Client struct {
	registryURL string
	httpClient  *http.Client
	nodeInfo    *NodeInfo
}

// NewClient creates a new discovery client
func NewClient(registryURL string, nodeInfo *NodeInfo) *Client {
	return &Client{
		registryURL: registryURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		nodeInfo: nodeInfo,
	}
}

// Register registers this node with the registry
func (c *Client) Register() error {
	data, err := json.Marshal(c.nodeInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal node info: %w", err)
	}

	url := fmt.Sprintf("%s/register", c.registryURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to register with registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully registered node %s with registry", c.nodeInfo.ID)
	return nil
}

// DiscoverPeers discovers healthy peer nodes from the registry
func (c *Client) DiscoverPeers() ([]string, error) {
	url := fmt.Sprintf("%s/discover", c.registryURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to discover peers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery failed with status %d", resp.StatusCode)
	}

	var result struct {
		Nodes []NodeInfo `json:"nodes"`
		Count int        `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode discovery response: %w", err)
	}

	// Convert to peer addresses, excluding self
	var peers []string
	for _, node := range result.Nodes {
		if node.ID != c.nodeInfo.ID {
			peerAddr := fmt.Sprintf("%s:%s", node.Address, node.Port)
			peers = append(peers, peerAddr)
		}
	}

	log.Printf("Discovered %d peers: %v", len(peers), peers)
	return peers, nil
}

// SendHeartbeat sends a heartbeat to the registry
func (c *Client) SendHeartbeat() error {
	// Create heartbeat payload with current node info
	heartbeatData := map[string]interface{}{
		"node_id":      c.nodeInfo.ID,
		"height":       c.nodeInfo.Height,
		"tx_pool_size": c.nodeInfo.TxPoolSize,
	}
	
	jsonData, err := json.Marshal(heartbeatData)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat data: %w", err)
	}
	
	url := fmt.Sprintf("%s/health", c.registryURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status %d", resp.StatusCode)
	}

	return nil
}

// UpdateNodeInfo updates the node information
func (c *Client) UpdateNodeInfo(height int32, txPoolSize int32) {
	c.nodeInfo.Height = height
	c.nodeInfo.TxPoolSize = txPoolSize
}

// StartHeartbeat starts sending periodic heartbeats to the registry
func (c *Client) StartHeartbeat(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			if err := c.SendHeartbeat(); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
			}
		}
	}()
}

// ReportConsensusFailure reports a consensus failure to the registry
func (c *Client) ReportConsensusFailure(reason string) error {
	data := map[string]string{
		"node_id": c.nodeInfo.ID,
		"reason":  reason,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal consensus failure report: %w", err)
	}

	url := fmt.Sprintf("%s/consensus-failure", c.registryURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to report consensus failure: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("consensus failure report failed with status %d", resp.StatusCode)
	}

	log.Printf("Reported consensus failure to registry: %s", reason)
	return nil
}

// GetRegistryStatus gets the overall registry status
func (c *Client) GetRegistryStatus() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/status", c.registryURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status request failed with status %d", resp.StatusCode)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	return status, nil
}

// AutoDiscoveryLoop runs the auto-discovery loop for new nodes
func (c *Client) AutoDiscoveryLoop(interval time.Duration, connectCallback func([]string) error) {
	// Initial registration
	for {
		if err := c.Register(); err != nil {
			log.Printf("Failed to register, retrying in 5s: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}

	// Start heartbeat
	c.StartHeartbeat(5 * time.Second)

	// Discovery loop
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			peers, err := c.DiscoverPeers()
			if err != nil {
				log.Printf("Failed to discover peers: %v", err)
				continue
			}

			if len(peers) > 0 && connectCallback != nil {
				if err := connectCallback(peers); err != nil {
					log.Printf("Failed to connect to discovered peers: %v", err)
				}
			}
		}
	}()
}

// WaitForMinimumPeers waits until a minimum number of peers are discovered
func (c *Client) WaitForMinimumPeers(minPeers int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		peers, err := c.DiscoverPeers()
		if err != nil {
			log.Printf("Discovery attempt failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if len(peers) >= minPeers {
			log.Printf("Minimum peer threshold met: %d/%d peers discovered", len(peers), minPeers)
			return nil
		}

		log.Printf("Waiting for more peers: %d/%d discovered", len(peers), minPeers)
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for minimum peers (%d)", minPeers)
}