package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type HomeFeedPlaylist struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Cover string `json:"cover"`
}

type HomeFeedSection struct {
	Title     string             `json:"title"`
	Playlists []HomeFeedPlaylist `json:"playlists"`
}

type homeFeedCache struct {
	sections  []HomeFeedSection
	fetchedAt time.Time
}

var (
	homeCacheMu sync.Mutex
	homeCache   *homeFeedCache
)

const homeCacheTTL = 2 * time.Hour

func FetchHomeFeed(ctx context.Context) ([]HomeFeedSection, error) {
	homeCacheMu.Lock()
	if homeCache != nil && time.Since(homeCache.fetchedAt) < homeCacheTTL {
		sections := homeCache.sections
		homeCacheMu.Unlock()
		return sections, nil
	}
	homeCacheMu.Unlock()

	client := NewSpotifyClient()
	if err := client.Initialize(); err != nil {
		return nil, fmt.Errorf("spotify init: %w", err)
	}
	token := client.accessToken

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		featured HomeFeedSection
		catSecs  []HomeFeedSection
		featErr  error
	)

	// Fetch featured playlists
	wg.Add(1)
	go func() {
		defer wg.Done()
		featured, featErr = fetchFeaturedPlaylists(ctx, token)
	}()

	// Fetch browse categories then their playlists in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		cats, err := fetchBrowseCategories(ctx, token)
		if err != nil || len(cats) == 0 {
			return
		}
		tmp := make([]HomeFeedSection, len(cats))
		var catWg sync.WaitGroup
		for i, cat := range cats {
			catWg.Add(1)
			go func(i int, id, name string) {
				defer catWg.Done()
				playlists, err := fetchCategoryPlaylists(ctx, token, id)
				if err == nil && len(playlists) > 0 {
					tmp[i] = HomeFeedSection{Title: name, Playlists: playlists}
				}
			}(i, cat.id, cat.name)
		}
		catWg.Wait()
		mu.Lock()
		for _, s := range tmp {
			if len(s.Playlists) > 0 {
				catSecs = append(catSecs, s)
			}
		}
		mu.Unlock()
	}()

	wg.Wait()

	var sections []HomeFeedSection
	if featErr == nil && len(featured.Playlists) > 0 {
		sections = append(sections, featured)
	}
	sections = append(sections, catSecs...)

	if len(sections) > 0 {
		homeCacheMu.Lock()
		homeCache = &homeFeedCache{sections: sections, fetchedAt: time.Now()}
		homeCacheMu.Unlock()
	}

	return sections, nil
}

// spotifyV1Get performs a GET request to the Spotify v1 REST API.
func spotifyV1Get(ctx context.Context, token, url string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		snip := string(body)
		if len(snip) > 200 {
			snip = snip[:200]
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snip)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func fetchFeaturedPlaylists(ctx context.Context, token string) (HomeFeedSection, error) {
	data, err := spotifyV1Get(ctx, token,
		"https://api.spotify.com/v1/browse/featured-playlists?limit=8&locale=en_US")
	if err != nil {
		return HomeFeedSection{}, err
	}

	msg := ""
	if m, ok := data["message"].(string); ok {
		msg = m
	}
	if msg == "" {
		msg = "Featured"
	}

	section := HomeFeedSection{Title: msg}
	pls, _ := data["playlists"].(map[string]interface{})
	items, _ := pls["items"].([]interface{})
	for _, raw := range items {
		p, ok := raw.(map[string]interface{})
		if !ok || p == nil {
			continue
		}
		id, _ := p["id"].(string)
		name, _ := p["name"].(string)
		cover := extractPlaylistCoverURL(p)
		if id != "" {
			section.Playlists = append(section.Playlists, HomeFeedPlaylist{ID: id, Title: name, Cover: cover})
		}
	}
	return section, nil
}

type browseCategory struct {
	id   string
	name string
}

func fetchBrowseCategories(ctx context.Context, token string) ([]browseCategory, error) {
	data, err := spotifyV1Get(ctx, token,
		"https://api.spotify.com/v1/browse/categories?limit=8&locale=en_US")
	if err != nil {
		return nil, err
	}

	cats, _ := data["categories"].(map[string]interface{})
	items, _ := cats["items"].([]interface{})

	var result []browseCategory
	for _, raw := range items {
		c, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := c["id"].(string)
		name, _ := c["name"].(string)
		if id != "" {
			result = append(result, browseCategory{id: id, name: name})
		}
	}
	return result, nil
}

func fetchCategoryPlaylists(ctx context.Context, token, categoryID string) ([]HomeFeedPlaylist, error) {
	data, err := spotifyV1Get(ctx, token,
		fmt.Sprintf("https://api.spotify.com/v1/browse/categories/%s/playlists?limit=8", categoryID))
	if err != nil {
		return nil, err
	}

	pls, _ := data["playlists"].(map[string]interface{})
	items, _ := pls["items"].([]interface{})

	var result []HomeFeedPlaylist
	for _, raw := range items {
		p, ok := raw.(map[string]interface{})
		if !ok || p == nil {
			continue
		}
		id, _ := p["id"].(string)
		name, _ := p["name"].(string)
		cover := extractPlaylistCoverURL(p)
		if id != "" {
			result = append(result, HomeFeedPlaylist{ID: id, Title: name, Cover: cover})
		}
	}
	return result, nil
}

func extractPlaylistCoverURL(p map[string]interface{}) string {
	images, _ := p["images"].([]interface{})
	if len(images) == 0 {
		return ""
	}
	img, _ := images[0].(map[string]interface{})
	url, _ := img["url"].(string)
	return url
}
