package web

import (
	"fmt"
)

// AddInstance adds and persists a new runtime.
func (c *Console) AddInstance(rawURL string) error {
	url := normalizeURL(rawURL)
	if url == "" {
		return fmt.Errorf("invalid URL")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, existing := range c.urls {
		if existing == url {
			return fmt.Errorf("instance already exists")
		}
	}
	// Add to memory
	client := NewClient(url, c.config.RefreshInterval, c.config.LogLevel)
	c.instances[url] = &Instance{URL: url, Client: client}
	c.urls = append(c.urls, url)

	// Persist
	store := &RuntimeStorage{URLs: c.urls}
	if err := SaveRuntimeStorage(store); err != nil {
		Warnf("failed to save runtime list: %v", err)
	}
	// Fetch its Catalog in background
	go func() {
		if cat, err := client.FetchCatalog(); err == nil {
			c.mu.Lock()
			c.instances[url].Catalog = cat
			c.mu.Unlock()
		}
	}()
	return nil
}

// DeleteInstance removes a runtime by URL.
func (c *Console) DeleteInstance(rawURL string) error {
	url := normalizeURL(rawURL)
	if url == "" {
		return fmt.Errorf("invalid URL")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	newURLs := make([]string, 0, len(c.urls))
	found := false
	for _, u := range c.urls {
		if u == url {
			found = true
			continue
		}
		newURLs = append(newURLs, u)
	}
	if !found {
		return fmt.Errorf("instance not found")
	}
	delete(c.instances, url)
	c.urls = newURLs
	store := &RuntimeStorage{URLs: c.urls}
	if err := SaveRuntimeStorage(store); err != nil {
		Warnf("failed to save runtime list: %v", err)
	}
	return nil
}

// UpdateInstance replaces an old URL with a new one.
func (c *Console) UpdateInstance(oldURL, newURL string) error {
	if err := c.DeleteInstance(oldURL); err != nil {
		return err
	}
	return c.AddInstance(newURL)
}

// ListInstances returns all managed URLs.
func (c *Console) ListInstances() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, len(c.urls))
	copy(out, c.urls)
	return out
}
