package retrieval

import "time"

// CachedImage is an image from the image server, as the cache holds it
type CachedImage struct {
	Data []byte `json:"data"`
}

/*
	GetImage returns an image cached by SaveImage.

Parameters:
  - url: the image's url

Returns:
  - []byte: the image
  - bool: whether it was cached
*/
func (c *CachedEveIDs) GetImage(url string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	image := cachedAs[CachedImage](c, sectionImages, c.Images, url)
	if image == nil || len(image.Data) == 0 {
		return nil, false
	}
	return image.Data, true
}

/*
	SaveImage caches an image by its url. It is not Permanent, so a newer SDE drops it (renders change with the game's data), but it has no time limit.

Parameters:
  - url: the image's url
  - data: the image
*/
func (c *CachedEveIDs) SaveImage(url string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	put(c, sectionImages, c.Images, url, &CacheEntry{Value: &CachedImage{Data: data}, CachedAt: time.Now().UTC()})
}
