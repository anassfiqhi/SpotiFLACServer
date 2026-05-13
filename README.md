# SpotiFLAC Server

A self-hosted Go HTTP server that powers [SpotiFLAC](https://github.com/spotbye/SpotiFLAC) — resolves Spotify tracks to lossless audio from Tidal, Qobuz, and Amazon Music, then downloads and tags them as FLAC/MP3/AAC with full metadata.

## Requirements

- Go 1.26+
- `ffmpeg` (for audio processing and resampling)

## Build

```sh
go build -o spotflac-server ./cmd/server/
```

## Docker

```sh
docker build -t spotflac-server .
docker run -p 8001:8001 -v /your/music:/data/music spotflac-server
```

## Configuration

| Environment variable  | Default        | Description                        |
|-----------------------|----------------|------------------------------------|
| `PORT`                | `8001`         | HTTP port to listen on             |
| `SPOTFLAC_DATA_DIR`   | `/data/cache`  | Directory for cache databases      |

## API

All responses are JSON. CORS is enabled for all origins.

### `GET /health`
Returns server status.

### `GET /search?q=<query>&limit=<n>&type=<track|album|artist|playlist>`
Searches Spotify. Omit `type` (or use `all`) for a combined result.

### `GET /metadata?url=<spotify_url>&separator=<sep>&batch=<true|false>&timeout=<s>&delay=<s>`
Fetches full track/album/playlist metadata from Spotify.

### `GET /stream/<spotify_track_id>?region=<cc>`
Returns all streaming service URLs for a track via Songlink/Odesli.

### `GET /availability/<spotify_track_id>`
Checks which services carry a track.

### `GET /isrc/<spotify_track_id>`
Resolves a Spotify track ID to its ISRC code.

### `GET /lyrics/<spotify_track_id>?track=&artist=&album=&duration=&format=<lrc|json>`
Fetches synced lyrics from LRCLIB and other sources.

### `GET /audio/<spotify_id>?quality=<HIGH|LOSSLESS|HI_RES>`
Resolves a Spotify track to a direct CDN audio URL (via Tidal) and redirects to it.

### `POST /download`
Downloads a track to the server filesystem and embeds metadata. Body is JSON:

```jsonc
{
  "service": "tidal",        // "tidal" | "qobuz" | "amazon"
  "spotify_id": "...",
  "service_url": "...",      // optional: direct service URL
  "track_name": "...",
  "artist_name": "...",
  "album_name": "...",
  "output_dir": "/data/music",
  "audio_format": "LOSSLESS", // LOSSLESS | HI_RES | HIGH etc.
  "filename_format": "title-artist",
  "embed_lyrics": true,
  "allow_fallback": true
}
```

Returns `{ "success": true, "file": "/path/to/file.flac", "item_id": "..." }`.

### `GET /queue`
Returns the current download queue.

### `GET /history`
Returns past downloads stored in the local BoltDB history database.

## Supported Sources

| Service      | Formats                        |
|--------------|--------------------------------|
| Tidal        | FLAC (Lossless, Hi-Res)        |
| Qobuz        | FLAC (up to 24-bit/192 kHz)    |
| Amazon Music | FLAC / AAC (HD, Ultra HD)      |

## API Credits

[MusicBrainz](https://musicbrainz.org) · [LRCLIB](https://lrclib.net) · [Songlink/Odesli](https://song.link) · [hifi-api](https://github.com/binimum/hifi-api) · [dabmusic.xyz](https://dabmusic.xyz) · [musicdl.me](https://musicdl.me)

## Disclaimer

This project is for **educational and private use only**. It is not affiliated with, endorsed by, or connected to Spotify, Tidal, Qobuz, Amazon Music, or any other streaming service. You are solely responsible for ensuring your use complies with local laws and the Terms of Service of the respective platforms.
