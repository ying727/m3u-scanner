package epg

import (
	"compress/gzip"
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// EPG represents the parsed EPG data
type EPG struct {
	Channels   []EPGChannel   `xml:"channel"`
	Programmes []EPGProgramme `xml:"programme"`
}

// EPGChannel represents a channel in EPG
type EPGChannel struct {
	ID          string   `xml:"id,attr"`
	DisplayName []string `xml:"display-name"`
	Icon        *Icon    `xml:"icon"`
}

// Icon represents channel icon
type Icon struct {
	Src string `xml:"src,attr"`
}

// EPGProgramme represents a programme entry
type EPGProgramme struct {
	Start       string       `xml:"start,attr"`
	Stop        string       `xml:"stop,attr"`
	Channel     string       `xml:"channel,attr"`
	Title       []LangText   `xml:"title"`
	SubTitle    []LangText   `xml:"sub-title"`
	Description []LangText   `xml:"desc"`
	Category    []LangText   `xml:"category"`
	Credits     *Credits     `xml:"credits"`
	Date        string       `xml:"date"`
	EpisodeNum  []EpisodeNum `xml:"episode-num"`
	Icon        *Icon        `xml:"icon"`
}

// LangText represents text with language attribute
type LangText struct {
	Lang string `xml:"lang,attr"`
	Text string `xml:",chardata"`
}

// Credits represents programme credits
type Credits struct {
	Directors  []string `xml:"director"`
	Actors     []Actor  `xml:"actor"`
	Writers    []string `xml:"writer"`
	Presenters []string `xml:"presenter"`
}

// Actor represents an actor entry
type Actor struct {
	Name string `xml:",chardata"`
	Role string `xml:"role,attr"`
}

// EpisodeNum represents episode number in various formats
type EpisodeNum struct {
	System string `xml:"system,attr"`
	Value  string `xml:",chardata"`
}

// Programme represents a simplified programme for display
type Programme struct {
	Start       time.Time `json:"start"`
	Stop        time.Time `json:"stop"`
	Title       string    `json:"title"`
	SubTitle    string    `json:"subtitle"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
}

// ChannelEPG represents EPG data for a single channel
type ChannelEPG struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Icon       string      `json:"icon"`
	Programmes []Programme `json:"programmes"`
}

// ParseFile parses an EPG file from local path
func ParseFile(path string) (*EPG, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return parseEPG(file, path)
}

// ParseURL parses an EPG file from URL
func ParseURL(url string) (*EPG, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseEPG(resp.Body, url)
}

func parseEPG(reader io.Reader, source string) (*EPG, error) {
	var r io.Reader = reader

	// Check if gzipped
	if strings.HasSuffix(strings.ToLower(source), ".gz") {
		gr, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		r = gr
	}

	var epg EPG
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&epg); err != nil {
		return nil, err
	}

	return &epg, nil
}

// GetChannelEPG returns EPG data for a specific channel ID
func (e *EPG) GetChannelEPG(channelID string) *ChannelEPG {
	var channel *EPGChannel
	for _, ch := range e.Channels {
		if ch.ID == channelID {
			channel = &ch
			break
		}
	}

	if channel == nil {
		return nil
	}

	result := &ChannelEPG{
		ID:         channel.ID,
		Programmes: make([]Programme, 0),
	}

	if len(channel.DisplayName) > 0 {
		result.Name = channel.DisplayName[0]
	}
	if channel.Icon != nil {
		result.Icon = channel.Icon.Src
	}

	for _, prog := range e.Programmes {
		if prog.Channel == channelID {
			p := Programme{}
			p.Start = parseEPGTime(prog.Start)
			p.Stop = parseEPGTime(prog.Stop)
			if len(prog.Title) > 0 {
				p.Title = prog.Title[0].Text
			}
			if len(prog.SubTitle) > 0 {
				p.SubTitle = prog.SubTitle[0].Text
			}
			if len(prog.Description) > 0 {
				p.Description = prog.Description[0].Text
			}
			if len(prog.Category) > 0 {
				p.Category = prog.Category[0].Text
			}
			result.Programmes = append(result.Programmes, p)
		}
	}

	return result
}

// GetCurrentProgramme returns the current programme for a channel
func (e *EPG) GetCurrentProgramme(channelID string) *Programme {
	now := time.Now()
	for _, prog := range e.Programmes {
		if prog.Channel != channelID {
			continue
		}
		start := parseEPGTime(prog.Start)
		stop := parseEPGTime(prog.Stop)
		if now.After(start) && now.Before(stop) {
			p := &Programme{
				Start: start,
				Stop:  stop,
			}
			if len(prog.Title) > 0 {
				p.Title = prog.Title[0].Text
			}
			if len(prog.Description) > 0 {
				p.Description = prog.Description[0].Text
			}
			return p
		}
	}
	return nil
}

func parseEPGTime(s string) time.Time {
	// EPG time format: 20210101120000 +0000
	layouts := []string{
		"20060102150405 -0700",
		"20060102150405",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
