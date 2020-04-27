# `hls-stats`
Small package and example application (`/cmd/hls-stats`) for downloading segments in a `HLS` (`.m3u8`) playlist.

Logs failing downloads and duration of segment downloads.

## Example application
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


