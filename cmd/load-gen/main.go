package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	listener "github.com/filleokus/hls-stats"
)

type Logger struct {
	quiet bool
}

func main() {
	quiet := flag.Bool("quiet", false, "Do not print successful downloads")
	instances := flag.Int("instances", 10, "Number of paralell clients")
	buffer := flag.Int("buffer", 1, "Number of segments away from live edge to start playback")
	proxyURLString := flag.String("proxy", "", "HTTP(S) proxy [http://URL:port]")
	userAgentString := flag.String("useragent", "hls-stats-0.01", "Provide custom value for User-Agent header")
	flag.Parse()

	playlistURL := flag.Arg(0)
	var logger = Logger{quiet: *quiet}

	if playlistURL == "" {
		fmt.Println("Usage: load-gen [options] [url]")
		fmt.Println("URL must include protocol and point to a variant-playlist")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	var httpClient *http.Client
	httpClient = &http.Client{
		Timeout: time.Second * 3,
	}
	if *proxyURLString != "" {
		proxyUrl, err := url.Parse(*proxyURLString)
		if err != nil {
			log.Fatal("Invalid proxy URL")
		}
		httpClient = &http.Client{
			Timeout:   time.Second * 3,
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)},
		}
	}

	fmt.Printf("Starting %d session(s)\n", *instances)
	fmt.Printf("Starting %d session(s) with User-Agent %s \n", *instances, *userAgentString)
	for i := 0; i < *instances; i++ {
		time.Sleep(time.Millisecond * 100)
		go listener.StartListener(playlistURL, *buffer, logger, httpClient)
		go listener.StartListener(playlistURL, *buffer, logger, httpClient, *userAgentString)
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Exiting...")
		fmt.Println("Bye...")
		os.Exit(0)
	}()

	select {}
}

func (l Logger) SuccessfullyDownloaded(message listener.SuccessMessage) {
	if !l.quiet {
		fmt.Printf("%-4d ms %s %s\n", message.Duration.Milliseconds(), message.Time.Format(time.RFC3339), message.URL)
	}
}

func (l Logger) ErrorWhileDownloading(playbackError listener.PlaybackError) {
	fmt.Printf("%+v\n", playbackError)
}
