package gui

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type fakeImageClient struct {
	mu     sync.Mutex
	asked  []string
	bodies map[string]string // by endpoint
	status map[string]int
	delay  time.Duration
}

func (f *fakeImageClient) Request(method, endpoint string, body interface{}) (*http.Response, error) {
	f.mu.Lock()
	f.asked = append(f.asked, endpoint)
	f.mu.Unlock()
	time.Sleep(f.delay)
	data, ok := f.bodies[endpoint]
	if !ok {
		return nil, api.ErrRequestFailedToComplete
	}
	status := http.StatusOK
	if s, ok := f.status[endpoint]; ok {
		status = s
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(data))}, nil
}
func (f *fakeImageClient) UpdateHeaders(map[string]string) {}

type fakeImageCache struct {
	mu     sync.Mutex
	images map[string][]byte
}

func (c *fakeImageCache) GetImage(url string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.images[url]
	return d, ok
}
func (c *fakeImageCache) SaveImage(url string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.images[url] = data
}

const pngBody = "\x89PNG\r\n\x1a\nrest"

func TestFetchImages(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	base := api.IMAGE_SERVER_URL
	client := &fakeImageClient{
		bodies: map[string]string{"types/1/icon": pngBody, "types/2/icon": "<html>", "types/3/icon": "\xFF\xD8\xFFjpeg", "types/5/icon": pngBody},
		status: map[string]int{"types/5/icon": 204},
	}
	cache := &fakeImageCache{images: map[string][]byte{base + "/types/9/icon": []byte("cached")}}
	urls := []string{base + "/types/1/icon", base + "/types/2/icon", base + "/types/3/icon", base + "/types/4/icon", base + "/types/5/icon", base + "/types/9/icon", base + "/types/1/icon"}

	got := fetchImages(client, cache, urls, 10*time.Second)

	assert.Equal(t, map[string][]byte{
		base + "/types/1/icon": []byte(pngBody),
		base + "/types/3/icon": []byte("\xFF\xD8\xFFjpeg"),
		base + "/types/9/icon": []byte("cached"),
	}, got)
	assert.NotContains(t, client.asked, "types/9/icon")
	assert.Len(t, client.asked, 5, "the repeated url is fetched once")
	_, saved := cache.GetImage(base + "/types/1/icon")
	assert.True(t, saved)
	_, saved = cache.GetImage(base + "/types/2/icon")
	assert.False(t, saved)

	// a second run fetches nothing
	client.asked = nil
	fetchImages(client, cache, []string{base + "/types/1/icon"}, time.Second)
	assert.Empty(t, client.asked)
}

func TestFetchImagesCap(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	client := &fakeImageClient{bodies: map[string]string{"types/1/icon": pngBody}, delay: 3 * time.Second}
	started := time.Now()
	got := fetchImages(client, &fakeImageCache{images: map[string][]byte{}}, []string{api.IMAGE_SERVER_URL + "/types/1/icon"}, 50*time.Millisecond)
	assert.Less(t, time.Since(started), time.Second)
	assert.Empty(t, got)
}
