package gui

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
)

const (
	imageWorkers        = 4                // images fetched at once
	imageRequestTimeout = 15 * time.Second // one request to the image server
	imageFetchCap       = 45 * time.Second // all of a report's images, past which the search carries on with what it has
	imageMaxBytes       = 2 << 20          // an image bigger than this is not used
)

// imageCache is the part of the cache the images use
type imageCache interface {
	GetImage(url string) ([]byte, bool)
	SaveImage(url string, data []byte)
}

// newImageClient is a connection to the image server. It has the retries of every api.APIConnection but not the ESI's RateGuard, as the image server is not under the ESI's limits.
func newImageClient() api.APIClient {
	return api.APIConnection{
		BaseURL:    api.IMAGE_SERVER_URL,
		HTTPClient: &http.Client{Timeout: imageRequestTimeout},
		Headers:    map[string]string{"User-Agent": api.USER_AGENT},
		MaxRetries: 2,
		Backoff:    time.Second,
	}
}

/*
	fetchReportImages gets the PDF report's images from the EVE image server, each from the cache when it has it, the rest in parallel, and caches what it fetched.
	A failed image is logged and left out; it never fails the search.

Parameters:
  - urls: the images to get, see pdfImageURLs

Returns:
  - map[string][]byte: each image got, by its url
*/
func (a *App) fetchReportImages(urls []string) map[string][]byte {
	return fetchImages(newImageClient(), a.cache, urls, imageFetchCap)
}

/*
fetchImages is fetchReportImages with its client, cache, and time cap given. When the cap passes it returns what it has, and requests still running are left to finish on their own.
*/
func fetchImages(client api.APIClient, cache imageCache, urls []string, limit time.Duration) map[string][]byte {
	var mu sync.Mutex
	images := make(map[string][]byte, len(urls))
	var missing []string
	seen := make(map[string]bool, len(urls))
	for _, url := range urls {
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		if cache != nil {
			if data, found := cache.GetImage(url); found {
				images[url] = data
				continue
			}
		}
		missing = append(missing, url)
	}
	if len(missing) == 0 {
		return images
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	closed := false // set once the cap passed, so late images are not added to the map being returned
	for range min(imageWorkers, len(missing)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer comm.RecoverGoroutine("fetching report images", nil)
			for url := range jobs {
				data, err := downloadImage(client, url)
				if err != nil {
					comm.Logger.WithError(err).Warnf("Unable to get report image %s, it will be left out", url)
					continue
				}
				if cache != nil {
					cache.SaveImage(url, data)
				}
				mu.Lock()
				if !closed {
					images[url] = data
				}
				mu.Unlock()
			}
		}()
	}

	expired := make(chan struct{}) // closed when the cap passes
	timer := time.AfterFunc(limit, func() { close(expired) })
	defer timer.Stop()
	done := make(chan struct{})
	go func() {
		defer comm.RecoverGoroutine("feeding report image fetches", nil)
		defer close(done)
		for _, url := range missing {
			select {
			case jobs <- url:
			case <-expired:
				close(jobs)
				return
			}
		}
		close(jobs)
	}()
	<-done

	finished := make(chan struct{})
	go func() {
		defer comm.RecoverGoroutine("waiting for report images", nil)
		defer close(finished)
		wg.Wait()
	}()
	select {
	case <-finished:
	case <-expired:
		comm.Logger.Warn("Report images are taking too long, carrying on without the rest")
	}

	mu.Lock()
	defer mu.Unlock()
	closed = true
	result := make(map[string][]byte, len(images))
	for url, data := range images {
		result[url] = data
	}
	return result
}

// downloadImage gets one image, accepting only a 200 with a PNG or JPEG body of at most imageMaxBytes
func downloadImage(client api.APIClient, url string) ([]byte, error) {
	resp, err := client.Request(http.MethodGet, strings.TrimPrefix(strings.TrimPrefix(url, api.IMAGE_SERVER_URL), "/"), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errBadImage("status " + http.StatusText(resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, imageMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > imageMaxBytes {
		return nil, errBadImage("larger than 2 MB")
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) && !bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return nil, errBadImage("not a PNG or JPEG")
	}
	return data, nil
}

type errBadImage string

func (e errBadImage) Error() string { return "bad image: " + string(e) }
