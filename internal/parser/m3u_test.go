package parser

import (
	"strings"
	"testing"
)

func TestParseBasicM3U(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 group-title="News" tvg-id="cnn" tvg-logo="http://example.com/cnn.png",CNN HD
http://example.com/cnn.m3u8
#EXTINF:-1 group-title="Sports",ESPN
http://example.com/espn.m3u8
`
	playlist, err := parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(playlist.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(playlist.Channels))
	}

	ch := playlist.Channels[0]
	if ch.Name != "CNN HD" {
		t.Errorf("name = %q, want %q", ch.Name, "CNN HD")
	}
	if ch.GroupTitle != "News" {
		t.Errorf("group_title = %q, want %q", ch.GroupTitle, "News")
	}
	if ch.TVGId != "cnn" {
		t.Errorf("tvg_id = %q, want %q", ch.TVGId, "cnn")
	}
	if ch.Logo != "http://example.com/cnn.png" {
		t.Errorf("logo = %q, want %q", ch.Logo, "http://example.com/cnn.png")
	}
	if ch.URL != "http://example.com/cnn.m3u8" {
		t.Errorf("url = %q, want %q", ch.URL, "http://example.com/cnn.m3u8")
	}

	ch2 := playlist.Channels[1]
	if ch2.Name != "ESPN" {
		t.Errorf("name = %q, want %q", ch2.Name, "ESPN")
	}
}

func TestParseChannelNameWithComma(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 group-title="Movies",Action, Comedy & More
http://example.com/channel.m3u8
`
	playlist, err := parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(playlist.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(playlist.Channels))
	}

	if playlist.Channels[0].Name != "Action, Comedy & More" {
		t.Errorf("name = %q, want %q", playlist.Channels[0].Name, "Action, Comedy & More")
	}
}

func TestParseEPGUrl(t *testing.T) {
	input := `#EXTM3U url-tvg="http://example.com/epg.xml"
#EXTINF:-1,Test
http://example.com/test.m3u8
`
	playlist, err := parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if playlist.EPGUrl != "http://example.com/epg.xml" {
		t.Errorf("epg_url = %q, want %q", playlist.EPGUrl, "http://example.com/epg.xml")
	}
}

func TestParseEPGUrlX(t *testing.T) {
	input := `#EXTM3U x-tvg-url="http://example.com/epg2.xml"
#EXTINF:-1,Test
http://example.com/test.m3u8
`
	playlist, err := parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if playlist.EPGUrl != "http://example.com/epg2.xml" {
		t.Errorf("epg_url = %q, want %q", playlist.EPGUrl, "http://example.com/epg2.xml")
	}
}

func TestParseEmptyPlaylist(t *testing.T) {
	input := `#EXTM3U
`
	playlist, err := parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(playlist.Channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(playlist.Channels))
	}
}

func TestParseSkipsUnknownDirectives(t *testing.T) {
	input := `#EXTM3U
#EXTVLCOPT:http-user-agent=MyAgent
#EXTINF:-1,Test Channel
http://example.com/test.m3u8
`
	playlist, err := parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(playlist.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(playlist.Channels))
	}
	if playlist.Channels[0].Name != "Test Channel" {
		t.Errorf("name = %q, want %q", playlist.Channels[0].Name, "Test Channel")
	}
}

func TestParseAttributes(t *testing.T) {
	input := `group-title="News" tvg-id="cnn" tvg-name="CNN" tvg-logo="http://logo.png"`
	attrs := parseAttributes(input)

	tests := map[string]string{
		"group-title": "News",
		"tvg-id":      "cnn",
		"tvg-name":    "CNN",
		"tvg-logo":    "http://logo.png",
	}

	for key, want := range tests {
		if got := attrs[key]; got != want {
			t.Errorf("attrs[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestParseNoDuration(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 tvg-id="test",Test
http://example.com/test.m3u8
#EXTINF:120,WithDuration
http://example.com/duration.m3u8
`
	playlist, err := parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(playlist.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(playlist.Channels))
	}
}

func TestParseBlankLines(t *testing.T) {
	input := `#EXTM3U

#EXTINF:-1,Channel A

http://example.com/a.m3u8

#EXTINF:-1,Channel B
http://example.com/b.m3u8

`
	playlist, err := parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(playlist.Channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(playlist.Channels))
	}
}
