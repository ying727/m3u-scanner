package ffprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var (
	ffprobePath     string
	ffprobePathOnce sync.Once
	// 全局信号量控制 ffprobe/ffmpeg 并发数
	globalSemaphore chan struct{}
	semaphoreMu     sync.Mutex
)

// InitConcurrency 初始化全局并发控制，可重复调用以更新并发数
func InitConcurrency(maxConcurrency int) {
	if maxConcurrency <= 0 {
		maxConcurrency = 20
	}
	semaphoreMu.Lock()
	if globalSemaphore == nil || cap(globalSemaphore) != maxConcurrency {
		globalSemaphore = make(chan struct{}, maxConcurrency)
	}
	semaphoreMu.Unlock()
}

// acquireSemaphore 获取信号量
func acquireSemaphore(ctx context.Context) bool {
	semaphoreMu.Lock()
	sem := globalSemaphore
	semaphoreMu.Unlock()
	if sem == nil {
		InitConcurrency(20)
		sem = globalSemaphore
	}
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// releaseSemaphore 释放信号量
func releaseSemaphore() {
	semaphoreMu.Lock()
	sem := globalSemaphore
	semaphoreMu.Unlock()
	if sem != nil {
		<-sem
	}
}

// AcquireSemaphore 导出的信号量获取函数，供server.go使用
func AcquireSemaphore(ctx context.Context) bool {
	return acquireSemaphore(ctx)
}

// ReleaseSemaphore 导出的信号量释放函数，供server.go使用
func ReleaseSemaphore() {
	releaseSemaphore()
}

// getFFprobePath finds ffprobe executable
// Priority: 1. Same directory as exe  2. PATH
func getFFprobePath() string {
	ffprobePathOnce.Do(func() {
		name := "ffprobe"
		if runtime.GOOS == "windows" {
			name = "ffprobe.exe"
		}

		// Check same directory as executable
		if exePath, err := os.Executable(); err == nil {
			localPath := filepath.Join(filepath.Dir(exePath), name)
			if _, err := os.Stat(localPath); err == nil {
				ffprobePath = localPath
				return
			}
		}

		// Check current working directory
		if cwd, err := os.Getwd(); err == nil {
			localPath := filepath.Join(cwd, name)
			if _, err := os.Stat(localPath); err == nil {
				ffprobePath = localPath
				return
			}
		}

		// Fall back to PATH
		if path, err := exec.LookPath(name); err == nil {
			ffprobePath = path
			return
		}

		// Default to just the name, let OS handle it
		ffprobePath = name
	})
	return ffprobePath
}

// StreamInfo contains detailed information about a media stream
type StreamInfo struct {
	Available    bool          `json:"available"`
	ResponseTime time.Duration `json:"response_time"`
	Error        string        `json:"error,omitempty"`
	Format       *FormatInfo   `json:"format,omitempty"`
	VideoStreams []VideoStream `json:"video_streams,omitempty"`
	AudioStreams []AudioStream `json:"audio_streams,omitempty"`
}

// FormatInfo contains container format information
type FormatInfo struct {
	Name       string  `json:"name"`
	LongName   string  `json:"long_name"`
	Duration   float64 `json:"duration"`
	Size       int64   `json:"size"`
	BitRate    int64   `json:"bit_rate"`
	ProbeScore int     `json:"probe_score"`
}

// VideoStream contains video stream information
type VideoStream struct {
	Index         int     `json:"index"`
	Codec         string  `json:"codec"`
	CodecLongName string  `json:"codec_long_name"`
	Profile       string  `json:"profile"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	FrameRate     float64 `json:"frame_rate"`
	BitRate       int64   `json:"bit_rate"`
	BitRateMode   string  `json:"bit_rate_mode,omitempty"` // CBR, VBR, or empty
	PixelFormat   string  `json:"pixel_format"`
	FieldOrder    string  `json:"field_order"` // progressive, tt, bb, tb, bt, unknown
	// HDR相关
	ColorTransfer  string `json:"color_transfer,omitempty"`
	ColorPrimaries string `json:"color_primaries,omitempty"`
	ColorSpace     string `json:"color_space,omitempty"`
}

// AudioStream contains audio stream information
type AudioStream struct {
	Index         int    `json:"index"`
	Codec         string `json:"codec"`
	CodecLongName string `json:"codec_long_name"`
	Profile       string `json:"profile"`
	SampleRate    int    `json:"sample_rate"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	BitRate       int64  `json:"bit_rate"`
	BitRateMode   string `json:"bit_rate_mode,omitempty"` // CBR, VBR, or empty
	Language      string `json:"language"`
}

// ffprobeOutput represents the JSON output from ffprobe
type ffprobeOutput struct {
	Format  ffprobeFormat   `json:"format"`
	Streams []ffprobeStream `json:"streams"`
	Packets []ffprobePacket `json:"packets,omitempty"`
}

type ffprobeFormat struct {
	Filename   string `json:"filename"`
	FormatName string `json:"format_name"`
	LongName   string `json:"format_long_name"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
	BitRate    string `json:"bit_rate"`
	ProbeScore int    `json:"probe_score"`
}

type ffprobeStream struct {
	Index          int               `json:"index"`
	CodecType      string            `json:"codec_type"`
	CodecName      string            `json:"codec_name"`
	CodecLongName  string            `json:"codec_long_name"`
	Profile        string            `json:"profile"`
	Width          int               `json:"width,omitempty"`
	Height         int               `json:"height,omitempty"`
	PixFmt         string            `json:"pix_fmt,omitempty"`
	FieldOrder     string            `json:"field_order,omitempty"`
	RFrameRate     string            `json:"r_frame_rate,omitempty"`
	SampleRate     string            `json:"sample_rate,omitempty"`
	Channels       int               `json:"channels,omitempty"`
	ChannelLayout  string            `json:"channel_layout,omitempty"`
	BitRate        string            `json:"bit_rate,omitempty"`
	ColorTransfer  string            `json:"color_transfer,omitempty"`
	ColorPrimaries string            `json:"color_primaries,omitempty"`
	ColorSpace     string            `json:"color_space,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
}

type ffprobePacket struct {
	StreamIndex  int    `json:"stream_index"`
	Size         string `json:"size"`
	Pts          string `json:"pts"`
	PtsTime      string `json:"pts_time"`
	Duration     string `json:"duration"`
	DurationTime string `json:"duration_time"`
}

// Probe analyzes a stream URL and returns detailed information
func Probe(url string, timeout time.Duration) *StreamInfo {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 控制 ffprobe 进程并发
	if !acquireSemaphore(ctx) {
		return &StreamInfo{Error: "cancelled"}
	}
	defer releaseSemaphore()

	start := time.Now()
	info := &StreamInfo{}

	// 第一步：获取基本流信息
	cmd := exec.CommandContext(ctx, getFFprobePath(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-analyzeduration", "3000000",
		"-probesize", "3000000",
		url,
	)

	output, err := cmd.Output()
	info.ResponseTime = time.Since(start)

	if err != nil {
		info.Available = false
		if ctx.Err() == context.DeadlineExceeded {
			info.Error = "timeout"
		} else {
			info.Error = err.Error()
		}
		return info
	}

	var probeOutput ffprobeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		info.Available = false
		info.Error = "failed to parse ffprobe output"
		return info
	}

	info.Available = true
	info.Format = parseFormat(probeOutput.Format)
	info.VideoStreams, info.AudioStreams = parseStreams(probeOutput.Streams, nil, info.Format)

	// 第二步：通过采样packets计算码率和判断CBR/VBR模式
	if ctx.Err() == nil {
		calcBitrateFromPackets(ctx, url, info)
	}

	return info
}

// calcBitrateFromPackets 通过采样packets计算码率
// 调用者 Probe() 已持有信号量，此处不再获取
func calcBitrateFromPackets(ctx context.Context, url string, info *StreamInfo) {
	// 使用 -show_entries 只获取必要字段，大幅减少输出量
	cmd := exec.CommandContext(ctx, getFFprobePath(),
		"-v", "quiet",
		"-select_streams", "v:0",
		"-show_entries", "packet=size,pts_time,stream_index",
		"-read_intervals", "%+2",
		"-of", "json",
		url,
	)

	output, err := cmd.Output()
	if err != nil {
		return
	}

	var packetData struct {
		Packets []ffprobePacket `json:"packets"`
	}
	if err := json.Unmarshal(output, &packetData); err != nil {
		return
	}

	if len(packetData.Packets) < 10 {
		return
	}

	// 计算码率和判断CBR/VBR
	var sizes []int64
	var minPts, maxPts float64
	first := true

	for _, p := range packetData.Packets {
		size, _ := strconv.ParseInt(p.Size, 10, 64)
		pts, _ := strconv.ParseFloat(p.PtsTime, 64)

		if size > 0 {
			sizes = append(sizes, size)
		}
		if pts > 0 {
			if first {
				minPts = pts
				maxPts = pts
				first = false
			} else {
				if pts < minPts {
					minPts = pts
				}
				if pts > maxPts {
					maxPts = pts
				}
			}
		}
	}

	duration := maxPts - minPts
	if duration <= 0 || len(sizes) < 5 {
		return
	}

	// 计算总大小和码率
	var totalSize int64
	for _, s := range sizes {
		totalSize += s
	}
	bitRate := int64(float64(totalSize*8) / duration)

	// 判断CBR/VBR
	mean := float64(totalSize) / float64(len(sizes))
	var sumSquares float64
	for _, s := range sizes {
		diff := float64(s) - mean
		sumSquares += diff * diff
	}
	stdDev := math.Sqrt(sumSquares / float64(len(sizes)))
	cv := stdDev / mean

	var mode string
	if cv < 0.15 {
		mode = "CBR"
	} else if cv > 0.4 {
		mode = "VBR"
	} else {
		mode = "ABR"
	}

	// 更新视频流信息：始终设置模式，仅在码率为0时覆盖
	for i := range info.VideoStreams {
		if info.VideoStreams[i].BitRate == 0 {
			info.VideoStreams[i].BitRate = bitRate
		}
		info.VideoStreams[i].BitRateMode = mode
	}
}

// CheckAvailability performs a quick availability check without full probing
func CheckAvailability(url string, timeout time.Duration) (bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, getFFprobePath(),
		"-v", "quiet",
		"-analyzeduration", "1000000", // 1 second
		"-probesize", "1000000",
		url,
	)

	err := cmd.Run()
	elapsed := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		return false, elapsed, ctx.Err()
	}

	return err == nil, elapsed, err
}

func parseFormat(f ffprobeFormat) *FormatInfo {
	format := &FormatInfo{
		Name:       f.FormatName,
		LongName:   f.LongName,
		ProbeScore: f.ProbeScore,
	}

	if d, err := strconv.ParseFloat(f.Duration, 64); err == nil {
		format.Duration = d
	}
	if s, err := strconv.ParseInt(f.Size, 10, 64); err == nil {
		format.Size = s
	}
	if b, err := strconv.ParseInt(f.BitRate, 10, 64); err == nil {
		format.BitRate = b
	}

	return format
}

func parseStreams(streams []ffprobeStream, packets []ffprobePacket, formatInfo *FormatInfo) ([]VideoStream, []AudioStream) {
	var videoStreams []VideoStream
	var audioStreams []AudioStream

	for _, s := range streams {
		switch s.CodecType {
		case "video":
			video := VideoStream{
				Index:          s.Index,
				Codec:          s.CodecName,
				CodecLongName:  s.CodecLongName,
				Profile:        s.Profile,
				Width:          s.Width,
				Height:         s.Height,
				PixelFormat:    s.PixFmt,
				FieldOrder:     s.FieldOrder,
				ColorTransfer:  s.ColorTransfer,
				ColorPrimaries: s.ColorPrimaries,
				ColorSpace:     s.ColorSpace,
			}

			// 尝试从流的bit_rate字段获取
			if b, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil && b > 0 {
				video.BitRate = b
			}

			// 如果没有，尝试从format总码率估算（视频通常占90%）
			if video.BitRate == 0 && formatInfo != nil && formatInfo.BitRate > 0 {
				video.BitRate = formatInfo.BitRate * 90 / 100
				video.BitRateMode = "估算"
			}

			video.FrameRate = parseFrameRate(s.RFrameRate)
			videoStreams = append(videoStreams, video)

		case "audio":
			audio := AudioStream{
				Index:         s.Index,
				Codec:         s.CodecName,
				CodecLongName: s.CodecLongName,
				Profile:       s.Profile,
				Channels:      s.Channels,
				ChannelLayout: s.ChannelLayout,
			}
			if sr, err := strconv.Atoi(s.SampleRate); err == nil {
				audio.SampleRate = sr
			}

			// 尝试从流的bit_rate字段获取
			if b, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil && b > 0 {
				audio.BitRate = b
			}

			if lang, ok := s.Tags["language"]; ok {
				audio.Language = lang
			}
			audioStreams = append(audioStreams, audio)
		}
	}

	return videoStreams, audioStreams
}

func parseFrameRate(rate string) float64 {
	if rate == "" || rate == "0/0" {
		return 0
	}
	var num, den int
	if n, _ := fmt.Sscanf(rate, "%d/%d", &num, &den); n == 2 && den != 0 {
		return float64(num) / float64(den)
	}
	if f, err := strconv.ParseFloat(rate, 64); err == nil {
		return f
	}
	return 0
}

// IsFFprobeAvailable checks if ffprobe is installed and accessible
func IsFFprobeAvailable() bool {
	cmd := exec.Command(getFFprobePath(), "-version")
	return cmd.Run() == nil
}

// GetFFprobePath returns the path being used for ffprobe (for debugging)
func GetFFprobePath() string {
	return getFFprobePath()
}
