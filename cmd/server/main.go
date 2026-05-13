package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/afkarxyz/SpotiFLAC/backend"
)

// ── Request / Response types ──────────────────────────────────────────────────

type DownloadRequest struct {
	Service              string `json:"service"`
	SpotifyID            string `json:"spotify_id,omitempty"`
	ServiceURL           string `json:"service_url,omitempty"`
	TrackName            string `json:"track_name,omitempty"`
	ArtistName           string `json:"artist_name,omitempty"`
	AlbumName            string `json:"album_name,omitempty"`
	AlbumArtist          string `json:"album_artist,omitempty"`
	ReleaseDate          string `json:"release_date,omitempty"`
	CoverURL             string `json:"cover_url,omitempty"`
	TidalAPIURL          string `json:"tidal_api_url,omitempty"`
	OutputDir            string `json:"output_dir,omitempty"`
	AudioFormat          string `json:"audio_format,omitempty"`
	FilenameFormat       string `json:"filename_format,omitempty"`
	TrackNumber          bool   `json:"track_number,omitempty"`
	Position             int    `json:"position,omitempty"`
	UseAlbumTrackNumber  bool   `json:"use_album_track_number,omitempty"`
	EmbedLyrics          bool   `json:"embed_lyrics,omitempty"`
	EmbedMaxQualityCover bool   `json:"embed_max_quality_cover,omitempty"`
	Duration             int    `json:"duration,omitempty"`
	ItemID               string `json:"item_id,omitempty"`
	SpotifyTrackNumber   int    `json:"spotify_track_number,omitempty"`
	SpotifyDiscNumber    int    `json:"spotify_disc_number,omitempty"`
	SpotifyTotalTracks   int    `json:"spotify_total_tracks,omitempty"`
	SpotifyTotalDiscs    int    `json:"spotify_total_discs,omitempty"`
	ISRC                 string `json:"isrc,omitempty"`
	Copyright            string `json:"copyright,omitempty"`
	Publisher            string `json:"publisher,omitempty"`
	Composer             string `json:"composer,omitempty"`
	PlaylistName         string `json:"playlist_name,omitempty"`
	PlaylistOwner        string `json:"playlist_owner,omitempty"`
	AllowFallback        bool   `json:"allow_fallback"`
	UseFirstArtistOnly   bool   `json:"use_first_artist_only,omitempty"`
	UseSingleGenre       bool   `json:"use_single_genre,omitempty"`
	EmbedGenre           bool   `json:"embed_genre,omitempty"`
	Separator            string `json:"separator,omitempty"`
}

type DownloadResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message,omitempty"`
	File          string `json:"file,omitempty"`
	Error         string `json:"error,omitempty"`
	AlreadyExists bool   `json:"already_exists,omitempty"`
	ItemID        string `json:"item_id,omitempty"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func pathSuffix(r *http.Request, prefix string) string {
	return strings.TrimPrefix(r.URL.Path, prefix)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GET /health
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "SpotiFLAC-Server",
	})
}

// GET /search?q=...&limit=...&type=track|album|artist|playlist
func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	searchType := r.URL.Query().Get("type")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if searchType == "" || searchType == "all" {
		resp, err := backend.SearchSpotify(ctx, q, limit)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
	} else {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		resp, err := backend.SearchSpotifyByType(ctx, q, searchType, limit, offset)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": resp, "total": len(resp)})
	}
}

// GET /metadata?url=...&separator=...&batch=true
func handleMetadata(w http.ResponseWriter, r *http.Request) {
	spotifyURL := r.URL.Query().Get("url")
	if spotifyURL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	separator := r.URL.Query().Get("separator")
	if separator == "" {
		separator = ", "
	}

	batch := r.URL.Query().Get("batch") == "true"

	timeoutSec, _ := strconv.ParseFloat(r.URL.Query().Get("timeout"), 64)
	if timeoutSec <= 0 {
		timeoutSec = 300
	}
	delaySec, _ := strconv.ParseFloat(r.URL.Query().Get("delay"), 64)
	if delaySec <= 0 {
		delaySec = 1
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSec*float64(time.Second)))
	defer cancel()

	data, err := backend.GetFilteredSpotifyData(
		ctx, spotifyURL, batch,
		time.Duration(delaySec*float64(time.Second)),
		separator, nil,
	)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// GET /stream/{track_id}?region=...
func handleStream(w http.ResponseWriter, r *http.Request) {
	trackID := pathSuffix(r, "/stream/")
	if trackID == "" {
		writeError(w, http.StatusBadRequest, "track_id is required")
		return
	}

	region := r.URL.Query().Get("region")
	client := backend.NewSongLinkClient()
	urls, err := client.GetAllURLsFromSpotify(trackID, region)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, urls)
}

// GET /availability/{track_id}
func handleAvailability(w http.ResponseWriter, r *http.Request) {
	trackID := pathSuffix(r, "/availability/")
	if trackID == "" {
		writeError(w, http.StatusBadRequest, "track_id is required")
		return
	}

	client := backend.NewSongLinkClient()
	avail, err := client.CheckTrackAvailability(trackID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, avail)
}

// GET /isrc/{track_id}
func handleISRC(w http.ResponseWriter, r *http.Request) {
	trackID := pathSuffix(r, "/isrc/")
	if trackID == "" {
		writeError(w, http.StatusBadRequest, "track_id is required")
		return
	}

	isrc := backend.ResolveTrackISRC(trackID)
	if isrc == "" {
		writeError(w, http.StatusNotFound, "ISRC not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"track_id": trackID, "isrc": isrc})
}

// GET /lyrics/{track_id}?track=...&artist=...&album=...&duration=...&format=lrc|json
func handleLyrics(w http.ResponseWriter, r *http.Request) {
	trackID := pathSuffix(r, "/lyrics/")
	if trackID == "" {
		writeError(w, http.StatusBadRequest, "track_id is required")
		return
	}

	trackName := r.URL.Query().Get("track")
	artistName := r.URL.Query().Get("artist")
	albumName := r.URL.Query().Get("album")
	duration, _ := strconv.Atoi(r.URL.Query().Get("duration"))
	format := r.URL.Query().Get("format")

	client := backend.NewLyricsClient()
	resp, source, err := client.FetchLyricsAllSources(trackID, trackName, artistName, albumName, duration)
	if err != nil || resp == nil || len(resp.Lines) == 0 {
		msg := "lyrics not found"
		if err != nil {
			msg = err.Error()
		}
		writeError(w, http.StatusNotFound, msg)
		return
	}

	if format == "lrc" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(client.ConvertToLRC(resp, trackName, artistName)))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"lyrics": resp,
		"source": source,
	})
}

// POST /download
// Downloads a track to the server filesystem. Body: DownloadRequest JSON.
func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Service == "" {
		req.Service = "tidal"
	}
	if req.OutputDir == "" {
		req.OutputDir = "."
	}
	if req.AudioFormat == "" {
		req.AudioFormat = "LOSSLESS"
	}
	if req.FilenameFormat == "" {
		req.FilenameFormat = "title-artist"
	}
	separator := req.Separator
	if separator == "" {
		separator = ", "
	}

	itemID := req.ItemID
	if itemID == "" {
		itemID = fmt.Sprintf("%s-%d", req.SpotifyID, time.Now().UnixNano())
	}

	backend.AddToQueue(itemID, req.TrackName, req.ArtistName, req.AlbumName, req.SpotifyID)
	backend.SetDownloading(true)
	backend.StartDownloadItem(itemID)
	defer backend.SetDownloading(false)

	spotifyURL := ""
	if req.SpotifyID != "" {
		spotifyURL = fmt.Sprintf("https://open.spotify.com/track/%s", req.SpotifyID)
	}

	// Resolve ISRC in parallel for Qobuz
	isrcChan := make(chan string, 1)
	if req.Service == "qobuz" && req.ISRC == "" && req.SpotifyID != "" {
		go func() {
			c := backend.NewSongLinkClient()
			isrc, _ := c.GetISRCDirect(req.SpotifyID)
			isrcChan <- isrc
		}()
	} else {
		isrcChan <- req.ISRC
	}

	// Fetch lyrics in parallel
	lyricsChan := make(chan string, 1)
	if req.EmbedLyrics && req.SpotifyID != "" {
		go func() {
			lc := backend.NewLyricsClient()
			resp, _, err := lc.FetchLyricsAllSources(req.SpotifyID, req.TrackName, req.ArtistName, req.AlbumName, req.Duration)
			if err == nil && resp != nil && len(resp.Lines) > 0 {
				lyricsChan <- lc.ConvertToLRC(resp, req.TrackName, req.ArtistName)
			} else {
				lyricsChan <- ""
			}
		}()
	} else {
		lyricsChan <- ""
	}

	var filename string
	var dlErr error

	switch req.Service {
	case "tidal":
		dl := backend.NewTidalDownloader(req.TidalAPIURL)
		if req.ServiceURL != "" {
			filename, dlErr = dl.DownloadByURLWithFallback(
				req.ServiceURL, req.OutputDir, req.AudioFormat, req.FilenameFormat,
				req.TrackNumber, req.Position,
				req.TrackName, req.ArtistName, req.AlbumName, req.AlbumArtist, req.ReleaseDate,
				req.UseAlbumTrackNumber, req.CoverURL, req.EmbedMaxQualityCover,
				req.SpotifyTrackNumber, req.SpotifyDiscNumber, req.SpotifyTotalTracks, req.SpotifyTotalDiscs,
				req.Copyright, req.Publisher, req.Composer, separator,
				req.ISRC, spotifyURL, req.AllowFallback,
				req.UseFirstArtistOnly, req.UseSingleGenre, req.EmbedGenre,
			)
		} else {
			filename, dlErr = dl.Download(
				req.SpotifyID, req.OutputDir, req.AudioFormat, req.FilenameFormat,
				req.TrackNumber, req.Position,
				req.TrackName, req.ArtistName, req.AlbumName, req.AlbumArtist, req.ReleaseDate,
				req.UseAlbumTrackNumber, req.CoverURL, req.EmbedMaxQualityCover,
				req.SpotifyTrackNumber, req.SpotifyDiscNumber, req.SpotifyTotalTracks, req.SpotifyTotalDiscs,
				req.Copyright, req.Publisher, req.Composer, separator,
				req.ISRC, spotifyURL, req.AllowFallback,
				req.UseFirstArtistOnly, req.UseSingleGenre, req.EmbedGenre,
			)
		}

	case "qobuz":
		isrc := strings.TrimSpace(<-isrcChan)
		dl := backend.NewQobuzDownloader()
		quality := req.AudioFormat
		if quality == "" {
			quality = "6"
		}
		filename, dlErr = dl.DownloadTrackWithISRC(
			isrc, req.OutputDir, quality, req.FilenameFormat,
			req.TrackNumber, req.Position,
			req.TrackName, req.ArtistName, req.AlbumName, req.AlbumArtist, req.ReleaseDate,
			req.UseAlbumTrackNumber, req.CoverURL, req.EmbedMaxQualityCover,
			req.SpotifyTrackNumber, req.SpotifyDiscNumber, req.SpotifyTotalTracks, req.SpotifyTotalDiscs,
			req.Copyright, req.Publisher, req.Composer, separator,
			spotifyURL, req.AllowFallback,
			req.UseFirstArtistOnly, req.UseSingleGenre, req.EmbedGenre,
		)

	case "amazon":
		dl := backend.NewAmazonDownloader()
		if req.ServiceURL != "" {
			filename, dlErr = dl.DownloadByURL(
				req.ServiceURL, req.OutputDir, req.AudioFormat, req.FilenameFormat,
				req.PlaylistName, req.PlaylistOwner,
				req.TrackNumber, req.Position,
				req.TrackName, req.ArtistName, req.AlbumName, req.AlbumArtist, req.ReleaseDate,
				req.CoverURL,
				req.SpotifyTrackNumber, req.SpotifyDiscNumber, req.SpotifyTotalTracks,
				req.EmbedMaxQualityCover, req.SpotifyTotalDiscs,
				req.Copyright, req.Publisher, req.Composer, separator,
				req.ISRC, spotifyURL,
				req.UseFirstArtistOnly, req.UseSingleGenre, req.EmbedGenre,
			)
		} else {
			filename, dlErr = dl.DownloadBySpotifyID(
				req.SpotifyID, req.OutputDir, req.AudioFormat, req.FilenameFormat,
				req.PlaylistName, req.PlaylistOwner,
				req.TrackNumber, req.Position,
				req.TrackName, req.ArtistName, req.AlbumName, req.AlbumArtist, req.ReleaseDate,
				req.CoverURL,
				req.SpotifyTrackNumber, req.SpotifyDiscNumber, req.SpotifyTotalTracks,
				req.EmbedMaxQualityCover, req.SpotifyTotalDiscs,
				req.Copyright, req.Publisher, req.Composer, separator,
				req.ISRC, spotifyURL,
				req.UseFirstArtistOnly, req.UseSingleGenre, req.EmbedGenre,
			)
		}

	default:
		backend.FailDownloadItem(itemID, "unknown service: "+req.Service)
		writeError(w, http.StatusBadRequest, "unknown service: "+req.Service)
		return
	}

	if dlErr != nil {
		backend.FailDownloadItem(itemID, dlErr.Error())
		writeJSON(w, http.StatusBadGateway, DownloadResponse{
			Success: false,
			Error:   dlErr.Error(),
			ItemID:  itemID,
		})
		return
	}

	alreadyExists := strings.HasPrefix(filename, "EXISTS:")
	if alreadyExists {
		filename = strings.TrimPrefix(filename, "EXISTS:")
		backend.SkipDownloadItem(itemID, filename)
	} else {
		// Embed lyrics if requested
		if req.EmbedLyrics {
			if lyrics := <-lyricsChan; lyrics != "" {
				if err := backend.EmbedLyricsOnlyUniversal(filename, lyrics); err != nil {
					fmt.Printf("Warning: failed to embed lyrics: %v\n", err)
				}
			}
		}

		var fileSizeMB float64
		if fi, err := os.Stat(filename); err == nil {
			fileSizeMB = float64(fi.Size()) / (1024 * 1024)
		}
		backend.CompleteDownloadItem(itemID, filename, fileSizeMB)

		go func(fPath, track, artist, album, sID, cover, service string) {
			time.Sleep(2 * time.Second)
			item := backend.HistoryItem{
				SpotifyID: sID,
				Title:     track,
				Artists:   artist,
				Album:     album,
				CoverURL:  cover,
				Path:      fPath,
				Source:    service,
			}
			if ext := filepath.Ext(fPath); len(ext) > 1 {
				item.Format = strings.ToUpper(ext[1:])
			}
			backend.AddHistoryItem(item, "SpotiFLAC")
		}(filename, req.TrackName, req.ArtistName, req.AlbumName, req.SpotifyID, req.CoverURL, req.Service)
	}

	msg := "download completed"
	if alreadyExists {
		msg = "file already exists"
	}
	writeJSON(w, http.StatusOK, DownloadResponse{
		Success:       true,
		Message:       msg,
		File:          filename,
		AlreadyExists: alreadyExists,
		ItemID:        itemID,
	})
}

// GET /queue
func handleQueue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, backend.GetDownloadQueue())
}

// GET /history
func handleHistory(w http.ResponseWriter, r *http.Request) {
	items, err := backend.GetHistoryItems("SpotiFLAC")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	if err := backend.InitHistoryDB("SpotiFLAC"); err != nil {
		fmt.Printf("Warning: history DB: %v\n", err)
	}
	if err := backend.InitISRCCacheDB(); err != nil {
		fmt.Printf("Warning: ISRC cache DB: %v\n", err)
	}
	if err := backend.InitProviderPriorityDB(); err != nil {
		fmt.Printf("Warning: provider priority DB: %v\n", err)
	}
	go func() {
		if err := backend.PrimeTidalAPIList(); err != nil {
			fmt.Printf("Warning: Tidal API list: %v\n", err)
		}
	}()

	defer backend.CloseHistoryDB()
	defer backend.CloseISRCCacheDB()
	defer backend.CloseProviderPriorityDB()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/search", handleSearch)
	mux.HandleFunc("/metadata", handleMetadata)
	mux.HandleFunc("/stream/", handleStream)
	mux.HandleFunc("/availability/", handleAvailability)
	mux.HandleFunc("/isrc/", handleISRC)
	mux.HandleFunc("/lyrics/", handleLyrics)
	mux.HandleFunc("/download", handleDownload)
	mux.HandleFunc("/queue", handleQueue)
	mux.HandleFunc("/history", handleHistory)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      cors(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 600 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("SpotiFLAC Server listening on :%s\n", port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	fmt.Println("Server stopped")
}
