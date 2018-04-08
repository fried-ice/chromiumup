package downloadextract

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/krolaw/zipstream"
)

// DownloadExtractor is a stateful utility to download zip archives via http(s) and extract them.
// Because of the the use of go pipes and routines, zip files are streamed right at the beginning of the download, so there is no need to buffer the complete archive first.
type DownloadExtractor struct {
	url               string
	outPath           string
	omittedParentDirs int
	removeOnFail      bool
}

// NewDownloadExtractor creates a new DownloadExtractor.
// A http GET request will be sent to url and the contents of the archive extracted to a folder at outPath.
func NewDownloadExtractor(url string, outPath string) *DownloadExtractor {
	return &DownloadExtractor{
		url:               url,
		outPath:           outPath,
		omittedParentDirs: 0,
		removeOnFail:      false,
	}
}

// OmitTopDirs sets the number of top hierarchy directories to be omitted on extraction time.
// This is useful, if your directory of interest is included in a wrapper directory you do not actually need.
func (d *DownloadExtractor) OmitTopDirs(count int) {
	d.omittedParentDirs = count
}

// RemoveOnFail enables, when set to true, the removal of any created files and directories if any error occurs.
func (d *DownloadExtractor) RemoveOnFail(b bool) {
	d.removeOnFail = b
}

// Run initiates the process for downloading and extracting the file.
func (d *DownloadExtractor) Run() {
	pR, pW := io.Pipe()
	go d.fetch(pW)
	d.extract(pR)

}

func (d *DownloadExtractor) fetch(pW *io.PipeWriter) {
	defer pW.Close()

	resp, err := http.Get(d.url)
	if err != nil {
		panic(err)
	}
	if resp.Body == nil {
		panic(errors.New("HTTP response body is nil"))
	}

	defer resp.Body.Close()
	_, err = io.Copy(pW, resp.Body)
	if err != nil {
		panic(err)
	}
}

func (d *DownloadExtractor) extract(pR *io.PipeReader) {
	defer pR.Close()

	// Delete extracted files on panic if this behavior is enabled via RemoveOnFail
	if d.removeOnFail {
		defer func() {
			if err := recover(); err != nil {
				e := os.RemoveAll(d.outPath)
				if e == nil {
					println("Removed already extracted files of partially downloaded archive")
				}
				panic(err)
			}
		}()
	}

	zR := zipstream.NewReader(pR)

	fHdr, err := zR.Next()
	for ; err != io.EOF; fHdr, err = zR.Next() {
		if err != nil {
			panic(err)
		}

		// Remove top folders if necessary
		shortenedPath := ""
		if d.omittedParentDirs != 0 {
			shortenedPath = strings.Join(strings.SplitAfterN(fHdr.Name, "/", d.omittedParentDirs+1)[d.omittedParentDirs:], "")
		} else {
			shortenedPath = fHdr.Name
		}

		fPath := filepath.Join(d.outPath, shortenedPath)

		if fHdr.FileInfo().IsDir() { // Create directory ...
			err := os.MkdirAll(fPath, os.ModePerm)
			if err != nil {
				panic(err)
			}
		} else { // ... or regular file

			err := os.MkdirAll(filepath.Dir(fPath), os.ModePerm)
			if err != nil {
				panic(err)
			}

			outFile, err := os.OpenFile(fPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fHdr.Mode())
			if err != nil {
				panic(err)
			}
			defer outFile.Close()

			fSize, err := io.Copy(outFile, zR)
			if err != nil {
				panic(err)
			}

			absPath, err := filepath.Abs(fPath)
			if err == nil {
				fmt.Printf("Wrote %v bytes to file \"%s\"\n", fSize, absPath)
			} else {
				fmt.Printf("Wrote %v bytes to file \"%s\"\n", fSize, fPath)
			}
		}
	}

}
