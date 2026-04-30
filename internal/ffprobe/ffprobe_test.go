package ffprobe

import (
	"testing"
)

func TestParseFrameRate(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"30/1", 30.0},
		{"24000/1001", 24000.0 / 1001.0},
		{"25/1", 25.0},
		{"0/0", 0},
		{"", 0},
		{"29.97", 29.97},
	}
	for _, tt := range tests {
		got := parseFrameRate(tt.input)
		diff := got - tt.want
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.001 {
			t.Errorf("parseFrameRate(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestParseFormat(t *testing.T) {
	f := ffprobeFormat{
		FormatName: "mpegts",
		LongName:   "MPEG-TS (MPEG-2 Transport Stream)",
		Duration:   "123.456",
		Size:       "1048576",
		BitRate:    "5000000",
		ProbeScore: 50,
	}

	info := parseFormat(f)

	if info.Name != "mpegts" {
		t.Errorf("Name = %q, want %q", info.Name, "mpegts")
	}
	if info.Duration != 123.456 {
		t.Errorf("Duration = %f, want 123.456", info.Duration)
	}
	if info.Size != 1048576 {
		t.Errorf("Size = %d, want 1048576", info.Size)
	}
	if info.BitRate != 5000000 {
		t.Errorf("BitRate = %d, want 5000000", info.BitRate)
	}
}

func TestParseStreams(t *testing.T) {
	streams := []ffprobeStream{
		{
			Index:         0,
			CodecType:     "video",
			CodecName:     "h264",
			CodecLongName: "H.264 / AVC",
			Profile:       "High",
			Width:         1920,
			Height:        1080,
			PixFmt:        "yuv420p",
			FieldOrder:    "progressive",
			RFrameRate:    "25/1",
			BitRate:       "5000000",
			ColorTransfer: "bt709",
		},
		{
			Index:         1,
			CodecType:     "audio",
			CodecName:     "aac",
			CodecLongName: "AAC (Advanced Audio Coding)",
			Profile:       "LC",
			SampleRate:    "48000",
			Channels:      2,
			ChannelLayout: "stereo",
			BitRate:       "128000",
			Tags:          map[string]string{"language": "eng"},
		},
	}

	formatInfo := &FormatInfo{BitRate: 6000000}
	video, audio := parseStreams(streams, nil, formatInfo)

	if len(video) != 1 {
		t.Fatalf("expected 1 video stream, got %d", len(video))
	}
	v := video[0]
	if v.Codec != "h264" {
		t.Errorf("video codec = %q, want %q", v.Codec, "h264")
	}
	if v.Width != 1920 || v.Height != 1080 {
		t.Errorf("resolution = %dx%d, want 1920x1080", v.Width, v.Height)
	}
	if v.FrameRate != 25.0 {
		t.Errorf("frame rate = %f, want 25.0", v.FrameRate)
	}
	if v.BitRate != 5000000 {
		t.Errorf("video bitrate = %d, want 5000000", v.BitRate)
	}
	if v.ColorTransfer != "bt709" {
		t.Errorf("color_transfer = %q, want %q", v.ColorTransfer, "bt709")
	}

	if len(audio) != 1 {
		t.Fatalf("expected 1 audio stream, got %d", len(audio))
	}
	a := audio[0]
	if a.Codec != "aac" {
		t.Errorf("audio codec = %q, want %q", a.Codec, "aac")
	}
	if a.Channels != 2 {
		t.Errorf("channels = %d, want 2", a.Channels)
	}
	if a.SampleRate != 48000 {
		t.Errorf("sample rate = %d, want 48000", a.SampleRate)
	}
	if a.Language != "eng" {
		t.Errorf("language = %q, want %q", a.Language, "eng")
	}
}

func TestParseStreamsBitrateFallback(t *testing.T) {
	streams := []ffprobeStream{
		{
			Index:     0,
			CodecType: "video",
			CodecName: "hevc",
			Width:     3840,
			Height:    2160,
			RFrameRate: "60/1",
			// No BitRate set
		},
	}
	formatInfo := &FormatInfo{BitRate: 10000000}
	video, _ := parseStreams(streams, nil, formatInfo)

	if len(video) != 1 {
		t.Fatalf("expected 1 video stream, got %d", len(video))
	}
	// Should estimate 90% of format bitrate
	if video[0].BitRate != 9000000 {
		t.Errorf("estimated bitrate = %d, want 9000000", video[0].BitRate)
	}
}
