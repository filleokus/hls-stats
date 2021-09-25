# `hls-stats`
Small package and two example applications (`/cmd`) for downloading segments in a `HLS` (`.m3u8`) playlist.

Logs failing downloads and duration of segment downloads.

## `load-gen`
Perform paralell download of segments in playlist to simulate load from real clients

### Usage
```
load-gen [options] [url]
```
*URL must include protocol and point to a variant-playlist*

**Options:**
- `buffer`
  - Number of segments away from live edge to start playback (default 1)
- `instances`
   - Number of paralell clients (default 10)
- `proxy`
  - HTTP(S) proxy [http://URL:port]
- `quiet`
  - Do not print successful downloads
- `useragent`
  - Provide custom value for User-Agent header (default "hls-stats-0.02")

## `hls-stats`
### Config

Currently you have to run the binary in the same directory as a `config.json` file with the follwoing structure:

```json
{
  "playlistUrls": [
    "https://example.com/audiom3u8"
  ],
  "bufferSegments": 0,
  "influxEndpoint": "https://example.com",
  "influxOrg": "Example Org",
  "influxBucket": "Example Bucket",
  "influxToken": "{{token }}"
}
```

*Influx parameters are optional*

## Arguments
- `-influx`: Use Influx for remote logging
- `-quiet`: Do not print successful downloads

