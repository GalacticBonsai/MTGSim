// Package scryfall provides a minimal Scryfall API client for on-demand card
// lookups and image URI enrichment. When using images, callers must follow
// Scryfall's Fan Content Policy (https://scryfall.com/docs/api): accurate
// User-Agent, no cropping copyright/artist, no watermarking, no distorting.
package scryfall

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	baseURL   = "https://api.scryfall.com"
	userAgent = "MTGSim/1.0 (github.com/mtgsim/mtgsim)"
)

// ImageURIs holds available image sizes for a Scryfall card.
// When displaying images, do not crop the copyright or artist name,
// and do not add watermarks or distort the image per Scryfall's policy.
type ImageURIs struct {
	Small      string `json:"small,omitempty"`
	Normal     string `json:"normal,omitempty"`
	Large      string `json:"large,omitempty"`
	PNG        string `json:"png,omitempty"`
	ArtCrop    string `json:"art_crop,omitempty"`
	BorderCrop string `json:"border_crop,omitempty"`
}

// CardData is a minimal Scryfall card object for image lookup.
type CardData struct {
	Name       string     `json:"name"`
	ScryfallID string     `json:"id"`
	ImageURIs  *ImageURIs `json:"image_uris,omitempty"`
}

// RulingData represents a single ruling from Scryfall.
type RulingData struct {
	Source    string `json:"source"`
	Published string `json:"published_at"`
	Comment   string `json:"comment"`
}

// RulingsResponse wraps the list of rulings returned by Scryfall.
type RulingsResponse struct {
	Data []RulingData `json:"data"`
}

// Client wraps the Scryfall API with a local file cache.
type Client struct {
	httpClient *http.Client
	cacheDir   string
}

// NewClient creates a Scryfall client with the default cache directory.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cacheDir:   ".cache/scryfall",
	}
}

// GetCardByName fetches a card's data by exact name, checking the local cache first.
func (c *Client) GetCardByName(name string) (*CardData, error) {
	if cached := c.loadCache(name); cached != nil {
		return cached, nil
	}
	url := fmt.Sprintf("%s/cards/named?exact=%s", baseURL, strings.ReplaceAll(name, " ", "+"))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scryfall API returned %d: %s", resp.StatusCode, string(body))
	}

	var card CardData
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, err
	}
	c.saveCache(name, &card)
	return &card, nil
}

// GetRulingsByName fetches rulings for a card by exact name.
func (c *Client) GetRulingsByName(name string) ([]RulingData, error) {
	// First get the card to obtain its Scryfall ID
	card, err := c.GetCardByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch card for rulings: %w", err)
	}
	if card.ScryfallID == "" {
		return nil, fmt.Errorf("card %s has no Scryfall ID", name)
	}

	// Check rulings cache
	cachePath := filepath.Join(c.cacheDir, "rulings", card.ScryfallID+".json")
	if data, err := os.ReadFile(cachePath); err == nil {
		var resp RulingsResponse
		if err := json.Unmarshal(data, &resp); err == nil {
			return resp.Data, nil
		}
	}

	url := fmt.Sprintf("%s/cards/%s/rulings", baseURL, card.ScryfallID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scryfall rulings API returned %d: %s", resp.StatusCode, string(body))
	}

	var rulingsResp RulingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&rulingsResp); err != nil {
		return nil, err
	}

	// Cache rulings
	_ = os.MkdirAll(filepath.Join(c.cacheDir, "rulings"), 0o755)
	if data, err := json.Marshal(rulingsResp); err == nil {
		_ = os.WriteFile(cachePath, data, 0o644)
	}

	return rulingsResp.Data, nil
}

func (c *Client) cacheFile(name string) string {
	safe := strings.ReplaceAll(name, "/", "-")
	safe = strings.ReplaceAll(safe, "\\", "-")
	return filepath.Join(c.cacheDir, safe+".json")
}

func (c *Client) loadCache(name string) *CardData {
	path := c.cacheFile(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var card CardData
	if err := json.Unmarshal(data, &card); err != nil {
		return nil
	}
	return &card
}

func (c *Client) saveCache(name string, card *CardData) {
	_ = os.MkdirAll(c.cacheDir, 0o755)
	data, _ := json.Marshal(card)
	_ = os.WriteFile(c.cacheFile(name), data, 0o644)
}
