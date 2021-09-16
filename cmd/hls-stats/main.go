package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	listener "github.com/filleokus/hls-stats"
	"github.com/google/uuid"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

type Config struct {
	PlaylistUrlStrings []string `json:"playlistUrls"`
	BufferSegments     int      `json:"bufferSegments"` // Number of segments to buffer
	InfluxEndpoint     string   `json:"influxEndpoint"`
	InfluxOrg          string   `json:"influxOrg"`
	InfluxBucket       string   `json:"influxBucket"`
	InfluxToken        string   `json:"influxToken"`
}

type Logger struct {
}

var writeApi api.WriteAPI
var sessionID uuid.UUID
var useInflux *bool
var quiet *bool

func main() {
	var config = parseConfig()
	var logger = Logger{}
	var client influxdb2.Client

	useInflux = flag.Bool("influx", false, "Use Influx for remote logging")
	quiet = flag.Bool("quiet", false, "Do not print successful downloads")
	flag.Parse()

	sessionID, _ = uuid.NewRandom()
	if *useInflux {
		client = influxdb2.NewClientWithOptions(config.InfluxEndpoint,
			config.InfluxToken,
			influxdb2.DefaultOptions().SetFlushInterval(1000*5))
		writeApi = client.WriteAPI(config.InfluxOrg, config.InfluxBucket)
	}

	fmt.Printf("Starting session with ID: %s\n", sessionID.String())
	for _, playlistUrlString := range config.PlaylistUrlStrings {
		go listener.StartListener(playlistUrlString, config.BufferSegments, logger)
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Exiting...")
		if *useInflux {
			writeApi.Flush()
			client.Close()
		}
		fmt.Println("Bye...")
		os.Exit(0)
	}()

	select {}
}

func parseConfig() Config {
	jsonFile, err := os.Open("config.json")
	if err != nil {
		fmt.Println(err)
	}

	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	var config Config

	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		fmt.Println(err)
	}
	return config
}

func (l Logger) SuccessfullyDownloaded(message listener.SuccessMessage) {
	if *useInflux {
		p := influxdb2.NewPointWithMeasurement("stats")
		p.AddField("X-Correlation-ID-HLS-Stats", message.CorrelationId.String())
		p.AddField("Duration", message.Duration.Milliseconds())
		p.AddField("File", message.File)
		p.AddField("URL", message.URL)

		p.AddTag("Host", message.Host)
		p.AddTag("Session", sessionID.String())

		p.SetTime(message.Time)
		writeApi.WritePoint(p)
	}
	if !*quiet {
		fmt.Printf("%-4d ms %s %s\n", message.Duration.Milliseconds(), message.Time.Format(time.RFC3339), message.URL)
	}
}

func (l Logger) ErrorWhileDownloading(playbackError listener.PlaybackError) {
	if *useInflux {
		p := influxdb2.NewPointWithMeasurement("errors")
		p.AddField("X-Correlation-ID-HLS-Stats", playbackError.CorrelationId.String())

		p.AddField("URL", playbackError.URL)
		p.AddTag("Host", playbackError.Host)
		p.AddTag("File", playbackError.File)
		p.AddTag("Session", sessionID.String())

		p.SetTime(playbackError.Time)
		writeApi.WritePoint(p)
	}
	if !*quiet {
		fmt.Printf("%+v\n", playbackError)
	}
}
