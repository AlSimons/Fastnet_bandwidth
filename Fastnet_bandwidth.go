package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

// Write the output to the current directory.
var outFilePath = "bandwidth_monitor_log.txt"

// Keeping track of firstTime lets us get immediate output instead
// of waiting for the interval timer.
var firstTime = true

// Seconds to wait for a complete response
var timeout = time.Duration(45)

// Interval between runs
var interval = time.Duration(2)

// Keep track of the file size as well as the URL. This lets us
// display the size in the log in the event the request times out
// or otherwise fails.
type fileInfo struct {
	url  string
	size int
}

func main() {
	// Make a slice of length 0 with a capacity of 10 of fileInfo.
	// This is just a convenience. If we were to declare the array
	// statically instead of using a slice, we would need to be sure
	// to change the dimension every time we added or commented out
	// a file.
	urls := make([]fileInfo, 0, 10)

	//PLEASE CHANGE these URLs if you want to use this and you're not
	// Al.  These hit on his hosting site.
	urls = append(urls, fileInfo{
		url:  "http://simonshome.org/tenk_random.txt",
		size: 10240})

	urls = append(urls, fileInfo{
		url:  "http://simonshome.org/megabyte_random.txt",
		size: 1000000})

	urls = append(urls, fileInfo{
		url:  "http://simonshome.org/two_meg_random.txt",
		size: 2000000})

	/**

	Don't use these larger sizes for now, while we lower the interval
	to 2 minutes.

	urls = append(urls, fileInfo{
		url:  "http://simonshome.org/four_meg_random.txt",
		size: 4000000})

	urls = append(urls, fileInfo{
		url:  "http://simonshome.org/six_meg_random.txt",
		size: 6000000})

	urls = append(urls, fileInfo{
		url:  "http://simonshome.org/ten_meg_random.txt",
		size: 10000000})
	*/

	outputHeaderIfNeeded(outFilePath)

	// Create the ticker with a very short time. We'll replace it
	// with the desired time in the go routine on first execution.
	// This allows us to get our first results without waiting
	// for an interval to elapse.
	ticker := time.NewTicker(1 * time.Second)

	// These next two are never used, since we don't include a way
	// to cleanly tear down.  We just Ctrl-C it out of existence.
	// Left them in to remind me ouf how to use channels to coordinate.
	quit := make(chan struct{})
	allExit := make(chan int)

	go func() {
		for {
			select {
			case <-ticker.C:
				if firstTime {
					ticker = time.NewTicker(interval * time.Minute)
					firstTime = false
				}
				runOverAllSizes(urls)
			case <-quit:
				ticker.Stop()
				allExit <- 1
				return
			}
		}
	}()

	// Wait forever. If we let main() complete, the go routine exits.
	<-allExit
}

func runOverAllSizes(urls []fileInfo) {
	for _, url := range urls {
		doTest(url)
	}
}

func doTest(url fileInfo) {
	// Include a Transport to force no compression.  This is probably
	// not needed.
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    timeout * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   timeout * time.Second,
	}

	// About timings. It SEEMS from experimentation, that the
	// Initial Get call only opens the connection, and that the data
	// are not transferred until the ReadAll() call.  For small files,
	// the time to set up the connection swamps the transfer time,
	// resulting in artificially low bandwidths. Track and report the
	// two times separately.
	startGet := time.Now()
	startDate := startGet.Format("2006-01-02")
	startTime := startGet.Format("15:04:05")

	response, err := client.Get(url.url)
	endGet := time.Now()
	getElapsed := endGet.Sub(startGet).Seconds()

	if err != nil {
		msg := fmt.Sprintf(
			"%s\t%s\t%d\t\t\t0.0\tget failed with error %s\n",
			startDate,
			startTime,
			url.size,
			err)
		doLog(msg)
		return
	}

	defer response.Body.Close()
	startReadAll := time.Now()
	contents, err := ioutil.ReadAll(response.Body)
	endReadAll := time.Now()
	readElapsed := endReadAll.Sub(startReadAll).Seconds()
	if err != nil {
		msg := fmt.Sprintf(
			"%s\t%s\t%d\t%6.4f\t\t0.0\treading contents failed with: %s\n",
			startDate,
			startTime,
			url.size,
			getElapsed,
			err)
		doLog(msg)
		return
	}

	// Just doing a simple len(string) works because what we really
	// care about is the number of bytes transferred; we don't care
	// about the number of characters. (The test file is ASCII anyway
	// so they are the same--but the interesting part is the length
	// in bytes.)
	bodyLength := len(string(contents))

	bytesPerSec := float64(bodyLength) / readElapsed
	bitsPerSec := 8.0 * bytesPerSec
	megaBitsPerSec := bitsPerSec / 1000000.0

	msg := fmt.Sprintf("%s\t%s\t%d\t%6.4f\t%6.4f\t%3.1f\n",
		startDate,
		startTime,
		bodyLength,
		getElapsed,
		readElapsed,
		megaBitsPerSec,
	)
	doLog(msg)
}

func doLog(msg string) {
	f, err := os.OpenFile(outFilePath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Printf("File open failed with error %s", err)
		//Keep on trucking
		return
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, msg)
	w.Flush()
}

func outputHeaderIfNeeded(path string) {
	_, err := os.Stat(path)
	if err == nil {
		// File exists, no need to do anything.
		return
	}
	if !os.IsNotExist(err) {
		// This is completely unexpected, don't know how to recover.
		// Bail out.
		fmt.Printf("File stat failed with error %s", err)
		os.Exit(1)
	}
	//Log doesn't exist. Create it and put out the header.
	f, err := os.Create(path)
	if err != nil {
		// Again, completely unexpected. Bail.
		fmt.Printf("File create failed with error %s", err)
		os.Exit(1)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprint(w,
		"Date\tTime\tSize (Bytes)\tGet Elapsed Sec\tRead Elapsed Sec\tMb/s\n")
	w.Flush()
}
