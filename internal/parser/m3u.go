package parser

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// Channel represents a single channel/stream entry in M3U
type Channel struct {
	Name       string            `json:"name"`
	URL        string            `json:"url"`
	GroupTitle string            `json:"group_title"`
	Logo       string            `json:"logo"`
	TVGId      string            `json:"tvg_id"`
	TVGName    string            `json:"tvg_name"`
	Attributes map[string]string `json:"attributes"`
}

// M3UPlaylist represents a parsed M3U playlist
type M3UPlaylist struct {
	Channels []Channel `json:"channels"`
	EPGUrl   string    `json:"epg_url"`
}

var (
	extinfDurRegex = regexp.MustCompile(`#EXTINF:(-?\d+)\s*(.*)`)
	attrRegex      = regexp.MustCompile(`(\w+(?:-\w+)*)="([^"]*)"`)
)

// ParseFile parses an M3U file from a local path
func ParseFile(path string) (*M3UPlaylist, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parse(file)
}

// ParseURL parses an M3U file from a URL
func ParseURL(url string) (*M3UPlaylist, error) {
	return ParseURLWithUA(url, "")
}

// ParseURLWithUA parses an M3U file from a URL with custom User-Agent
func ParseURLWithUA(url, userAgent string) (*M3UPlaylist, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return parse(resp.Body)
}

func parse(reader io.Reader) (*M3UPlaylist, error) {
	playlist := &M3UPlaylist{
		Channels: make([]Channel, 0),
	}

	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var currentChannel *Channel

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for M3U header with EPG URL
		if strings.HasPrefix(line, "#EXTM3U") {
			attrs := parseAttributes(line)
			if epgUrl, ok := attrs["url-tvg"]; ok {
				playlist.EPGUrl = epgUrl
			} else if epgUrl, ok := attrs["x-tvg-url"]; ok {
				playlist.EPGUrl = epgUrl
			}
			continue
		}

		// Parse EXTINF line
		if strings.HasPrefix(line, "#EXTINF:") {
			channel := parseExtInf(line)
			currentChannel = &channel
			continue
		}

		// Skip other directives
		if strings.HasPrefix(line, "#") {
			continue
		}

		// This is a URL line
		if currentChannel != nil {
			currentChannel.URL = line
			playlist.Channels = append(playlist.Channels, *currentChannel)
			currentChannel = nil
		}
	}

	return playlist, scanner.Err()
}

func parseExtInf(line string) Channel {
	channel := Channel{
		Attributes: make(map[string]string),
	}

	matches := extinfDurRegex.FindStringSubmatch(line)
	if len(matches) < 3 {
		return channel
	}

	remainder := matches[2]
	attrStr, name := splitExtInfComma(remainder)
	channel.Name = strings.TrimSpace(name)

	attrs := parseAttributes(attrStr)
	channel.Attributes = attrs

	if v, ok := attrs["group-title"]; ok {
		channel.GroupTitle = v
	}
	if v, ok := attrs["tvg-logo"]; ok {
		channel.Logo = v
	}
	if v, ok := attrs["tvg-id"]; ok {
		channel.TVGId = v
	}
	if v, ok := attrs["tvg-name"]; ok {
		channel.TVGName = v
	}

	return channel
}

// splitExtInfComma finds the first comma outside of quotes to split attributes from channel name.
func splitExtInfComma(s string) (attrs, name string) {
	inQuotes := false
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			inQuotes = !inQuotes
		} else if s[i] == ',' && !inQuotes {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

func parseAttributes(line string) map[string]string {
	attrs := make(map[string]string)
	matches := attrRegex.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			attrs[strings.ToLower(match[1])] = match[2]
		}
	}
	return attrs
}
