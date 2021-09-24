package listener

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/grafov/m3u8"
)

var client http.Client
var logger Logger

type Logger interface {
	SuccessfullyDownloaded(message SuccessMessage)
	ErrorWhileDownloading(playbackError PlaybackError)
}

type SuccessMessage struct {
	CorrelationId uuid.UUID
	Time          time.Time

	URL   string
	Host  string
	File  string
	Bytes int

	Duration time.Duration
}

type PlaybackError struct {
	CorrelationId uuid.UUID
	Time          time.Time

	URL  string
	Host string
	File string

	HTTPStatusCode int
	Message        string
}

func StartListener(playlistUrlString string, bufferSegments int, l Logger, httpClient *http.Client, userAgent string) {
	logger = l
	client = *httpClient
	startStreamingPlaylist(playlistUrlString, bufferSegments, userAgent)
}

func downloadURL(url string, userAgent string) (response *http.Response, didFailAndShouldRetry bool) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		log.Fatal(fmt.Sprintf("Could not make correlation ID: %s\n", err))
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(fmt.Sprintf("Could not make request for %s: %s X-Correlation-ID-HLS-Stats: %s\n", url, err, uuid.String()))
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Correlation-ID-HLS-Stats", uuid.String())

	start := time.Now()
	resp, err := client.Do(req)

	host, file := splitUrlString(url)
	t := time.Now()
	elapsed := t.Sub(start)

	if err != nil {
		// Here we can get network errors, like timeouts, we should report and try again

		playbackError := PlaybackError{
			CorrelationId:  uuid,
			Time:           time.Now(),
			URL:            url,
			Host:           host,
			File:           file,
			HTTPStatusCode: 0,
			Message:        fmt.Sprintf("No connection made: %s", err.Error()),
		}
		logger.ErrorWhileDownloading(playbackError)
		return nil, true
	}

	if resp.StatusCode != 200 {
		playbackError := PlaybackError{
			CorrelationId:  uuid,
			Time:           time.Now(),
			URL:            url,
			Host:           host,
			File:           file,
			HTTPStatusCode: resp.StatusCode,
			Message:        fmt.Sprintf("Connection made with HTTP error %d", resp.StatusCode),
		}
		logger.ErrorWhileDownloading(playbackError)
		return nil, true
	}
	var reader io.ReadCloser
	var transferedBytes int
	var body []byte
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, _ = gzip.NewReader(resp.Body)
		transferedBytes = int(resp.ContentLength)
		body, _ = io.ReadAll(reader)
		reader.Close()
		resp.Header.Del("Content-Encoding")
	default:
		reader = resp.Body
		body, _ = io.ReadAll(reader)
		transferedBytes = len(body)
		reader.Close()
	}
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	message := SuccessMessage{
		CorrelationId: uuid,
		Time:          time.Now(),
		URL:           url,
		Host:          host,
		File:          file,
		Duration:      elapsed,
		Bytes:         transferedBytes,
	}
	logger.SuccessfullyDownloaded(message)
	return resp, false

}

func startStreamingPlaylist(playlistUrl string, bufferSegments int, userAgent string) {
	// Download the playlist once, then the first segment, then enter
	// the infinite loop to continously download the new playlist and
	// segments

	playlist, playlistFetchingFailed := fetchPlaylist(playlistUrl, userAgent)
	if playlistFetchingFailed {
		log.Fatalf("Could not fetch playlist %s, check if it's correct and try again", playlistUrl)
	}

	var targetDuration = time.Duration(int64(playlist.TargetDuration * 1000000000))

	latestSegment := getLatestSegment(playlist, bufferSegments)

	currentSequenceId := latestSegment.SeqId

	fetchSegment(playlistUrl, playlist, &currentSequenceId, userAgent)
	time.Sleep(targetDuration)

	for {
		playlist, playlistFetchingFailed = fetchPlaylist(playlistUrl, userAgent)
		if playlistFetchingFailed {
			time.Sleep(targetDuration)
			break
		}

		segmentFetchingFailed := fetchSegment(playlistUrl, playlist, &currentSequenceId, userAgent)
		if segmentFetchingFailed {
			time.Sleep(targetDuration)
			break
		}

		time.Sleep(targetDuration)
	}

}

func fetchPlaylist(playlistUrl string, userAgent string) (*m3u8.MediaPlaylist, bool) {
	resp, didFail := downloadURL(playlistUrl, userAgent)
	if didFail {
		return nil, true
	}

	defer resp.Body.Close()
	playlist, _, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		log.Fatalf("Could not decode provided m3u8 for %s:\n%s\nIs the URL a variant playlist?", playlistUrl, err)
	}
	return playlist.(*m3u8.MediaPlaylist), false
}

func fetchSegment(playlistUrl string, playlist *m3u8.MediaPlaylist, sequenceId *uint64, userAgent string) (didFailAndShouldRetry bool) {
	var segment *m3u8.MediaSegment
	for _, s := range playlist.Segments {
		if s == nil {
			// We are looking for sequenceId not yet available -> Stall playback
			host, file := splitUrlString(playlistUrl)
			playbackError := PlaybackError{
				CorrelationId:  uuid.UUID{},
				Time:           time.Now(),
				URL:            playlistUrl,
				Host:           host,
				File:           file,
				HTTPStatusCode: 0,
				Message:        fmt.Sprintf("Sequence id: %d not available, stalling playback", sequenceId),
			}
			logger.ErrorWhileDownloading(playbackError)
			return true
		}
		if s.SeqId == *sequenceId {
			segment = s
			break
		}
	}

	if segment == nil {
		log.Fatal("Segment nil")
	}

	segmentUrl := segment.URI
	if !strings.HasPrefix(segmentUrl, "http") {
		// Segment URL is relative
		components := strings.Split(playlistUrl, "/")
		components = components[:len(components)-1]
		components = append(components, segmentUrl)
		segmentUrl = strings.Join(components, "/")
	}
	resp, didFail := downloadURL(segmentUrl, userAgent)
	if didFail {
		return true
	}

	resp.Body.Close()
	*sequenceId += 1
	return false

}

func getLatestSegment(playlist *m3u8.MediaPlaylist, bufferSegments int) *m3u8.MediaSegment {

	// Need to access the latest segment, playlist.Segments.tail is private...
	var latestSegmentIndex int
	for index := range playlist.Segments {
		if playlist.Segments[index+1] == nil {
			latestSegmentIndex = index
			break
		}
	}
	return playlist.Segments[latestSegmentIndex-bufferSegments]
}

func splitUrlString(urlString string) (host string, file string) {
	url, err := url.Parse(urlString)
	if err != nil {
		log.Fatalf("Could not parse URL: %s, %s \n", urlString, err)
	}
	host = url.Host
	components := strings.Split(urlString, "/")
	file = components[len(components)-1]
	return
}
