package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/fried-ice/chromiumup/downloadextract"
)

const (
	tmpExt = ".tmp"
	oldExt = "~"

	upstreamBase       = "https://www.googleapis.com/download/storage/v1/b/chromium-browser-snapshots/o/"
	upstreamSep        = "%2F"
	upstreamLastChange = "LAST_CHANGE"
	upstreamParams     = "?alt=media"
)

func main() {

	targetPath := "chromium"
	flag.Parse()
	if strings.TrimSpace(flag.Arg(0)) != "" {
		targetPath = flag.Arg(0)
	}

	// Listen for SIGTERM and register handling.
	// Remove temporary folder of downloaded files.
	sigtermChannel := make(chan os.Signal, 2)
	signal.Notify(sigtermChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigtermChannel
		println("Received SIGTERM signal\nDeleting temporary folder " + targetPath + tmpExt)
		os.RemoveAll(targetPath + tmpExt)
		os.Exit(1)
	}()

	platform, file := platformStrings()
	url := upstreamBase + platform + upstreamSep + latestBuild(platform) + upstreamSep + file + upstreamParams
	fmt.Printf("Downloading archive file from \"%s\"\n\n", url)
	dE := downloadextract.NewDownloadExtractor(url, targetPath+tmpExt)
	dE.OmitTopDirs(1)
	dE.RemoveOnFail(true)
	dE.Run()

	// If there is no such directory, we will simply rename the downloaded folder to its target path.
	// If there is, rename existing directory and rename downloaded directory to target path.
	// If this succeeds, delete original directory, else try to restore original directory and delete downloaded files.
	pathExisted := pathExists(targetPath)
	if pathExisted {
		err := os.Rename(targetPath, targetPath+oldExt)
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(targetPath + oldExt)
		defer fmt.Printf("\nDeleted old directory \"%s\"\n", targetPath+oldExt)
	}
	err := os.Rename(targetPath+tmpExt, targetPath)
	if err != nil {
		if pathExisted {
			// Restore previous state and remove downloaded files
			os.Rename(targetPath+oldExt, targetPath)
			os.RemoveAll(targetPath + tmpExt)
		}
		panic(err)
	}
}

func latestBuild(platform string) string {
	resp, err := http.Get(upstreamBase + platform + upstreamSep + upstreamLastChange + upstreamParams)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		panic(errors.New("Http Status not 200"))
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func platformStrings() (platform string, file string) {
	platform = ""
	file = ""
	switch runtime.GOOS {
	case "linux":
		platform += "Linux"
		file += "chrome-linux.zip"
	case "windows":
		platform += "Win"
		file += "chrome-win.zip"
	case "darwin":
		// There is no distinction between architectures here
		return "chrome-mac.zip", "Mac"
	default:
		panic(errors.New("Current GOOS not supported"))
	}

	switch runtime.GOARCH {
	case "amd64":
		platform += "_x64"
	case "386":
		platform += ""
	default:
		panic(errors.New("Current GOARCH not supported"))
	}

	return
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
