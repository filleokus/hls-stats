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
	quiet        bool
	statsChannel chan stat
}

type stat struct {
	time  time.Time
	bytes int64
}

func main() {
	quiet := flag.Bool("quiet", false, "Do not print successful downloads")
	instances := flag.Int("instances", 10, "Number of paralell clients")
	buffer := flag.Int("buffer", 1, "Number of segments away from live edge to start playback")
	proxyURLString := flag.String("proxy", "", "HTTP(S) proxy [http://URL:port]")
	userAgentString := flag.String("useragent", "hls-stats-0.02", "Provide custom value for User-Agent header")
	flag.Parse()

	playlistURL := flag.Arg(0)
	var logger = Logger{quiet: *quiet, statsChannel: make(chan stat)}

	if playlistURL == "" {
		fmt.Println("Usage: load-gen [options] [url]")
		fmt.Println("URL must include protocol and point to a variant-playlist")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	var httpClient *http.Client
	httpClient = &http.Client{
		Timeout:   time.Second * 3,
		Transport: &http.Transport{DisableCompression: true},
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

	fmt.Printf("Starting %d session(s) with User-Agent %s \n", *instances, *userAgentString)
	if *quiet {
		fmt.Println("Will not print successful downloads, only statistics")
	}
	for i := 0; i < *instances; i++ {
		time.Sleep(time.Millisecond * 100)
		go listener.StartListener(playlistURL, *buffer, logger, httpClient, *userAgentString)
	}

	go printStats(logger, 10)

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
		fmt.Printf("%-4d ms %s %s %s\n", message.Duration.Milliseconds(), ByteCountSI(int64(message.Bytes)), message.Time.Format(time.RFC3339), message.URL)
	}
	l.statsChannel <- stat{time.Now(), int64(message.Bytes)}
}

func (l Logger) ErrorWhileDownloading(playbackError listener.PlaybackError) {
	fmt.Printf("%+v\n", playbackError)
}

func printStats(l Logger, printInterval int) {
	var total int64
	var lastStat stat
	firstStat := <-l.statsChannel
	total = firstStat.bytes
	deadline := time.Now().Add(time.Second * time.Duration(printInterval))
	for stat := range l.statsChannel {
		total += stat.bytes
		lastStat = stat
		if time.Now().After(deadline) {
			duration := lastStat.time.Sub(firstStat.time)
			rate := int64(float64(total) / duration.Seconds())
			fmt.Printf("Last %.2f seconds: Transfered data: %-8s Avg: %s/s\n", duration.Seconds(), ByteCountSI(total), ByteCountSI(rate))
			firstStat = stat
			total = 0
			deadline = time.Now().Add(time.Second * time.Duration(printInterval))
		}
	}
}

func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
