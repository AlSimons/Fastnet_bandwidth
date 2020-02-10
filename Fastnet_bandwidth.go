package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

var urls = [3]string{ //MAKE SURE DIM MATCHES NUM STRINGS!
	"http://simonshome.org/tenk_random.txt",
	"http://simonshome.org/megabyte_random.txt",
	"http://simonshome.org/two_meg_random.txt",
	//"http://simonshome.org/four_meg_random.txt",
	//"http://simonshome.org/six_meg_random.txt",
	//"http://simonshome.org/ten_meg_random.txt",
}

var outFilePath = "bandwidth_monitor_log.txt"
var firstTime = true
var timeout = time.Duration(90)  // Seconds to wait for a complete response
var interval = time.Duration(10) // Interval between runs

func main() {
	outputHeaderIfNeeded(outFilePath)

	// Create the ticker with a very short time. We'll replace it
	// with the desired time in the go routine on first execution.
	// This allows us to get our
	ticker := time.NewTicker(1 * time.Second)
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
				runOverAllSizes()
			case <-quit:
				ticker.Stop()
				allExit <- 1
				return
			}
		}
	}()

	// Wait forever.
	<-allExit
}

func runOverAllSizes() {
	for _, url := range urls {
		doTest(url)
	}
}

func doTest(url string) {
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
	// are not transferred until the ReadAll() call.  Therefore, we
	// need to encapsulate both in our timings.
	startGet := time.Now()
	response, err := client.Get(url)
	endGet := time.Now()
	getElapsed := endGet.Sub(startGet).Seconds()

	if err != nil {
		msg := fmt.Sprintf("%s\t%s\t\t\t\t0.0\tget failed with error %s\n",
			startGet.Format("2006-01-02"),
			startGet.Format("15:04:05"),
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
		msg := fmt.Sprintf("%s\t%s\t\t%6.4f\t\t0.0\treading contents failed with: %s\n",
			startGet.Format("2006-01-02"),
			startGet.Format("15:04:05"),
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
		startGet.Format("2006-01-02"),
		startGet.Format("15:04:05"),
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
