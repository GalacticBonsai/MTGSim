// Package combo provides a Commander Spellbook API client for discovering
// combo variants contained in (or near-contained in) a given decklist.
package combo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const baseURL = "https://backend.commanderspellbook.com"

// Client wraps the Commander Spellbook API with local file caching.
type Client struct {
	httpClient *http.Client
	cacheDir   string
}

// NewClient creates a new Commander Spellbook API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		cacheDir:   ".cache/combos",
	}
}

// DeckRequest is the POST body sent to /find-my-combos.
type DeckRequest struct {
	Main       []CardInDeck `json:"main"`
	Commanders []CardInDeck `json:"commanders,omitempty"`
}

// CardInDeck is a single card entry inside a DeckRequest.
type CardInDeck struct {
	Card     string `json:"card"`
	Quantity int    `json:"quantity"`
}

// FindMyCombosResult holds the flattened variants returned by the API.
type FindMyCombosResult struct {
	Included       []Variant `json:"included"`
	AlmostIncluded []Variant `json:"almostIncluded"`
}

// Variant represents a combo variant from Commander Spellbook.
type Variant struct {
	ID          string          `json:"id"`
	Uses        []CardInVariant `json:"uses"`
	Requires    []TemplateInVariant `json:"requires"`
	Produces    []FeatureInVariant  `json:"produces"`
	Identity    string          `json:"identity"`
	ManaNeeded  string          `json:"manaNeeded"`
	Description string          `json:"description"`
	Notes       string          `json:"notes"`
	Popularity  *int            `json:"popularity"`
	Spoiler     bool            `json:"spoiler"`
}

// CardInVariant represents a card used in a variant.
type CardInVariant struct {
	Card                 ComboCard `json:"card"`
	ZoneLocations        []string  `json:"zoneLocations"`
	BattlefieldCardState string    `json:"battlefieldCardState"`
	ExileCardState       string    `json:"exileCardState"`
	LibraryCardState     string    `json:"libraryCardState"`
	GraveyardCardState   string    `json:"graveyardCardState"`
}

// ComboCard is the minimal card object inside a variant.
type ComboCard struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TemplateInVariant represents a template requirement in a variant.
type TemplateInVariant struct {
	Template        Template `json:"template"`
	Quantity        int      `json:"quantity"`
	ZoneLocations   []string `json:"zoneLocations"`
	MustBeCommander bool     `json:"mustBeCommander"`
}

// Template is a Scryfall query template.
type Template struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	ScryfallQuery string `json:"scryfallQuery"`
	ScryfallAPI   string `json:"scryfallApi"`
}

// FeatureInVariant represents a feature produced by a variant.
type FeatureInVariant struct {
	Feature Feature `json:"feature"`
}

// Feature is a combo outcome (e.g., "Infinite mana", "Win the game").
type Feature struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Combo is a minimal combo reference inside a variant.
type Combo struct {
	ID int `json:"id"`
}

// paginatedResponse is the raw API envelope.
// The API may return results as either a single object or an array depending
// on query parameters, so we unmarshal lazily via RawMessage.
type paginatedResponse struct {
	Count    int             `json:"count"`
	Next     *string         `json:"next"`
	Previous *string         `json:"previous"`
	Results  json.RawMessage `json:"results"`
}

type findMyCombosResponse struct {
	Identity                                           string    `json:"identity"`
	Included                                           []Variant `json:"included"`
	IncludedByChangingCommanders                       []Variant `json:"includedByChangingCommanders"`
	AlmostIncluded                                     []Variant `json:"almostIncluded"`
	AlmostIncludedByAddingColors                       []Variant `json:"almostIncludedByAddingColors"`
	AlmostIncludedByChangingCommanders                 []Variant `json:"almostIncludedByChangingCommanders"`
	AlmostIncludedByAddingColorsAndChangingCommanders []Variant `json:"almostIncludedByAddingColorsAndChangingCommanders"`
}

// unmarshalResults extracts one or more findMyCombosResponse values from the raw JSON.
func (p *paginatedResponse) unmarshalResults() ([]findMyCombosResponse, error) {
	if len(p.Results) == 0 {
		return nil, nil
	}
	// Try array first.
	var arr []findMyCombosResponse
	if err := json.Unmarshal(p.Results, &arr); err == nil {
		return arr, nil
	}
	// Fallback to single object.
	var single findMyCombosResponse
	if err := json.Unmarshal(p.Results, &single); err != nil {
		return nil, err
	}
	return []findMyCombosResponse{single}, nil
}

// FindMyCombos queries the API for combos contained in the given decklist.
// Results are cached by a hash of the sorted card names.
func (c *Client) FindMyCombos(deckCards, commanders []string) (*FindMyCombosResult, error) {
	cacheKey := deckHash(deckCards, commanders)
	if cached := c.loadCache(cacheKey); cached != nil {
		return cached, nil
	}

	reqBody := buildDeckRequest(deckCards, commanders)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := baseURL + "/find-my-combos/"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("commander spellbook API returned %d: %s", resp.StatusCode, string(b))
	}

	var page paginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, err
	}

	pageResults, err := page.unmarshalResults()
	if err != nil {
		return nil, err
	}

	result := &FindMyCombosResult{}
	for _, r := range pageResults {
		result.Included = append(result.Included, r.Included...)
		result.AlmostIncluded = append(result.AlmostIncluded, r.AlmostIncluded...)
	}

	// Follow pagination if needed.
	for page.Next != nil && *page.Next != "" {
		nextReq, err := http.NewRequest("GET", *page.Next, nil)
		if err != nil {
			break
		}
		nextReq.Header.Set("Accept", "application/json")
		nextResp, err := c.httpClient.Do(nextReq)
		if err != nil {
			break
		}
		var nextPage paginatedResponse
		if err := json.NewDecoder(nextResp.Body).Decode(&nextPage); err != nil {
			nextResp.Body.Close()
			break
		}
		nextResp.Body.Close()
		nextResults, err := nextPage.unmarshalResults()
		if err != nil {
			break
		}
		for _, r := range nextResults {
			result.Included = append(result.Included, r.Included...)
			result.AlmostIncluded = append(result.AlmostIncluded, r.AlmostIncluded...)
		}
		if nextPage.Next == nil || *nextPage.Next == "" {
			break
		}
		page = nextPage
	}

	c.saveCache(cacheKey, result)
	return result, nil
}

func buildDeckRequest(cards, commanders []string) DeckRequest {
	req := DeckRequest{Main: make([]CardInDeck, 0, len(cards))}
	for _, c := range cards {
		req.Main = append(req.Main, CardInDeck{Card: c, Quantity: 1})
	}
	for _, c := range commanders {
		req.Commanders = append(req.Commanders, CardInDeck{Card: c, Quantity: 1})
	}
	return req
}

func deckHash(cards, commanders []string) string {
	all := append([]string{}, cards...)
	all = append(all, commanders...)
	sort.Strings(all)
	h := sha256.New()
	for _, s := range all {
		h.Write([]byte(s))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Client) cacheFile(key string) string {
	return filepath.Join(c.cacheDir, key+".json")
}

func (c *Client) loadCache(key string) *FindMyCombosResult {
	data, err := os.ReadFile(c.cacheFile(key))
	if err != nil {
		return nil
	}
	var result FindMyCombosResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return &result
}

func (c *Client) saveCache(key string, result *FindMyCombosResult) {
	_ = os.MkdirAll(c.cacheDir, 0o755)
	data, _ := json.MarshalIndent(result, "", "  ")
	_ = os.WriteFile(c.cacheFile(key), data, 0o644)
}
