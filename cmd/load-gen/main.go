package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	listener "github.com/filleokus/hls-stats"
)

type Logger struct {
}

var quiet *bool
var instances *int
var buffer *int

func main() {
	var logger = Logger{}
	quiet = flag.Bool("quiet", false, "Do not print successful downloads")
	instances = flag.Int("instances", 10, "Number of paralell clients")
	buffer = flag.Int("buffer", 1, "Number of segments away from live edge to start playback")
	flag.Parse()
	playlistURL := flag.Arg(0)
	if playlistURL == "" {
		fmt.Println("Usage: load-gen [options] [url]")
		fmt.Println("URL must include protocol and point to a variant-playlist")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}
	fmt.Printf("Starting %d session(s)\n", instances)
	for i := 0; i < *instances; i++ {
		time.Sleep(time.Millisecond * 100)
		go listener.StartListener(playlistURL, 1, logger)
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
	if !*quiet {
		fmt.Printf("%-4d ms %s %s\n", message.Duration.Milliseconds(), message.Time.Format(time.RFC3339), message.URL)
	}
}

func (l Logger) ErrorWhileDownloading(playbackError listener.PlaybackError) {
	fmt.Printf("%+v\n", playbackError)
}
