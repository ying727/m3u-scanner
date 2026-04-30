package ui

import (
	"context"
	"crypto/md5"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"m3u-scanner/internal/epg"
	"m3u-scanner/internal/ffprobe"
	"m3u-scanner/internal/parser"
	"m3u-scanner/internal/scanner"
)

//go:embed static/*
var staticFiles embed.FS

// Server represents the web UI server
type Server struct {
	playlist     *parser.M3UPlaylist
	results      []scanner.ScanResult
	resultIndex  map[string]int
	resultsMutex sync.RWMutex

	scanning   bool
	scanMutex  sync.Mutex
	cancelFunc context.CancelFunc

	progress scanner.ScanProgress

	clients      map[chan string]bool
	clientsMutex sync.Mutex

	settings      Settings
	settingsPath  string

	thumbnails map[string]string // URL -> base64 thumbnail
	thumbMutex sync.RWMutex
	thumbDir   string

	epgData *epg.EPG
}

// Settings holds scanner settings
type Settings struct {
	Concurrency int    `json:"concurrency"`
	Timeout     int    `json:"timeout"`
	QuickCheck  bool   `json:"quickCheck"`
	UserAgent   string `json:"userAgent"`
}

// NewServer creates a new web UI server
func NewServer() *Server {
	thumbDir := filepath.Join(os.TempDir(), "m3u-scanner-thumbnails")
	_ = os.MkdirAll(thumbDir, 0755)

	settingsPath := filepath.Join(configDir(), "settings.json")
	s := &Server{
		results:      make([]scanner.ScanResult, 0),
		resultIndex:  make(map[string]int),
		clients:      make(map[chan string]bool),
		settings:     Settings{Concurrency: 20, Timeout: 15, QuickCheck: false},
		settingsPath: settingsPath,
		thumbnails:   make(map[string]string),
		thumbDir:     thumbDir,
	}
	s.loadSettings()
	return s
}

func configDir() string {
	if exePath, err := os.Executable(); err == nil {
		return filepath.Dir(exePath)
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func (s *Server) loadSettings() {
	data, err := os.ReadFile(s.settingsPath)
	if err != nil {
		return
	}
	var loaded Settings
	if json.Unmarshal(data, &loaded) == nil {
		s.settings = loaded
	}
}

func (s *Server) saveSettings() {
	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.settingsPath), 0755)
	_ = os.WriteFile(s.settingsPath, data, 0644)
}

// Run starts the web server
func (s *Server) Run(port int) error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/load-url", s.handleLoadURL)
	mux.HandleFunc("/api/scan/start", s.handleStartScan)
	mux.HandleFunc("/api/scan/stop", s.handleStopScan)
	mux.HandleFunc("/api/results", s.handleResults)
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/api/export/json", s.handleExportJSON)
	mux.HandleFunc("/api/export/csv", s.handleExportCSV)
	mux.HandleFunc("/api/play", s.handlePlay)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/events", s.handleSSE)
	mux.HandleFunc("/api/thumbnail", s.handleThumbnail)
	mux.HandleFunc("/api/stream", s.handleStream)
	mux.HandleFunc("/api/scan/single", s.handleScanSingle)
	mux.HandleFunc("/api/epg", s.handleEPG)

	// Static files
	mux.HandleFunc("/", s.handleStatic)

	addr := fmt.Sprintf(":%d", port)
	appURL := fmt.Sprintf("http://localhost:%d", port)

	fmt.Printf("M3U Scanner 启动: %s\n", appURL)
	fmt.Println("按 Ctrl+C 停止服务")

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(appURL)
	}()

	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	if strings.Contains(path, "..") {
		http.NotFound(w, r)
		return
	}

	content, err := staticFiles.ReadFile("static" + path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch {
	case strings.HasSuffix(path, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(path, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(path, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}

	_, _ = w.Write(content)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "读取文件失败", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "m3u-*.m3u")
	if err != nil {
		jsonError(w, "创建临时文件失败", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		jsonError(w, "保存上传文件失败", http.StatusBadRequest)
		return
	}
	if err := tmp.Close(); err != nil {
		jsonError(w, "保存上传文件失败", http.StatusBadRequest)
		return
	}

	playlist, err := parser.ParseFile(tmp.Name())
	if err != nil {
		jsonError(w, "解析M3U失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.loadPlaylist(playlist)
	jsonResponse(w, map[string]interface{}{
		"success":  true,
		"channels": len(playlist.Channels),
	})
}

func (s *Server) handleLoadURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "无效请求", http.StatusBadRequest)
		return
	}
	if !isAllowedURL(req.URL) {
		jsonError(w, "URL 不合法（仅支持公网 http/https）", http.StatusBadRequest)
		return
	}

	s.resultsMutex.RLock()
	userAgent := s.settings.UserAgent
	s.resultsMutex.RUnlock()

	playlist, err := parser.ParseURLWithUA(req.URL, userAgent)
	if err != nil {
		jsonError(w, "加载URL失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.loadPlaylist(playlist)
	jsonResponse(w, map[string]interface{}{
		"success":  true,
		"channels": len(playlist.Channels),
	})
}

func (s *Server) loadPlaylist(playlist *parser.M3UPlaylist) {
	s.resultsMutex.Lock()
	s.playlist = playlist
	s.results = make([]scanner.ScanResult, len(playlist.Channels))
	s.resultIndex = make(map[string]int, len(playlist.Channels))
	for i, ch := range playlist.Channels {
		s.results[i] = scanner.ScanResult{Channel: ch}
		s.resultIndex[ch.URL] = i
	}
	s.progress = scanner.ScanProgress{Total: len(playlist.Channels)}
	s.resultsMutex.Unlock()

	s.thumbMutex.Lock()
	s.thumbnails = make(map[string]string)
	s.thumbMutex.Unlock()

	// Load EPG asynchronously if URL is present
	if playlist.EPGUrl != "" {
		go func() {
			epgData, err := epg.ParseURL(playlist.EPGUrl)
			if err != nil {
				log.Printf("EPG加载失败: %v", err)
				return
			}
			s.resultsMutex.Lock()
			s.epgData = epgData
			s.resultsMutex.Unlock()
			s.broadcast("epg_loaded")
			log.Printf("EPG加载成功: %d个频道, %d个节目", len(epgData.Channels), len(epgData.Programmes))
		}()
	}
}

func (s *Server) handleEPG(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channel_id")
	if channelID == "" {
		jsonError(w, "需要channel_id参数", http.StatusBadRequest)
		return
	}

	s.resultsMutex.RLock()
	epgData := s.epgData
	s.resultsMutex.RUnlock()

	if epgData == nil {
		jsonResponse(w, map[string]interface{}{"available": false})
		return
	}

	channelEPG := epgData.GetChannelEPG(channelID)
	if channelEPG == nil {
		jsonResponse(w, map[string]interface{}{"available": false})
		return
	}

	current := epgData.GetCurrentProgramme(channelID)
	jsonResponse(w, map[string]interface{}{
		"available": true,
		"channel":   channelEPG,
		"current":   current,
	})
}

func (s *Server) handleStartScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.scanMutex.Lock()
	if s.scanning {
		s.scanMutex.Unlock()
		jsonError(w, "扫描正在进行中", http.StatusBadRequest)
		return
	}

	s.resultsMutex.RLock()
	playlist := s.playlist
	s.resultsMutex.RUnlock()
	if playlist == nil {
		s.scanMutex.Unlock()
		jsonError(w, "未加载播放列表", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	s.scanning = true
	s.scanMutex.Unlock()

	go s.runScan(ctx, playlist)
	jsonResponse(w, map[string]bool{"success": true})
}

func (s *Server) runScan(ctx context.Context, playlist *parser.M3UPlaylist) {
	defer func() {
		s.scanMutex.Lock()
		s.scanning = false
		s.cancelFunc = nil
		s.scanMutex.Unlock()
		s.broadcast("scan_complete")
	}()

	s.resultsMutex.RLock()
	concurrency := s.settings.Concurrency
	timeout := s.settings.Timeout
	quickCheck := s.settings.QuickCheck
	playlistLen := len(playlist.Channels)
	s.resultsMutex.RUnlock()

	if concurrency <= 0 {
		concurrency = 20
	}
	if concurrency > 100 {
		concurrency = 100
	}
	if timeout <= 0 {
		timeout = 15
	}

	sc := scanner.NewScanner(concurrency, time.Duration(timeout)*time.Second, quickCheck)
	progressCh := make(chan scanner.ScanProgress, 100)
	resultCh := make(chan scanner.ScanResult, playlistLen)
	errCh := make(chan error, 1)

	go func() {
		errCh <- sc.Scan(ctx, playlist, progressCh, resultCh)
	}()

	for progressCh != nil || resultCh != nil {
		select {
		case p, ok := <-progressCh:
			if !ok {
				progressCh = nil
				continue
			}
			s.resultsMutex.Lock()
			s.progress = p
			s.resultsMutex.Unlock()
			s.broadcast("progress")
		case result, ok := <-resultCh:
			if !ok {
				resultCh = nil
				continue
			}
			s.resultsMutex.Lock()
			if idx, found := s.resultIndex[result.Channel.URL]; found {
				s.results[idx] = result
			}
			s.resultsMutex.Unlock()
		}
	}

	if err := <-errCh; err != nil && err != context.Canceled {
		log.Printf("scan error: %v", err)
	}
}

func (s *Server) handleStopScan(w http.ResponseWriter, r *http.Request) {
	s.scanMutex.Lock()
	cancel := s.cancelFunc
	s.scanMutex.Unlock()

	if cancel != nil {
		cancel()
	}
	jsonResponse(w, map[string]bool{"success": true})
}

func (s *Server) handleScanSingle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "无效请求", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		jsonError(w, "需要URL参数", http.StatusBadRequest)
		return
	}

	s.resultsMutex.RLock()
	timeout := s.settings.Timeout
	quickCheck := s.settings.QuickCheck
	s.resultsMutex.RUnlock()

	if timeout <= 0 {
		timeout = 15
	}

	var streamInfo *ffprobe.StreamInfo
	if quickCheck {
		ok, responseTime, err := ffprobe.CheckAvailability(req.URL, time.Duration(timeout)*time.Second)
		streamInfo = &ffprobe.StreamInfo{
			Available:    ok,
			ResponseTime: responseTime,
		}
		if err != nil {
			streamInfo.Error = err.Error()
		}
	} else {
		streamInfo = ffprobe.Probe(req.URL, time.Duration(timeout)*time.Second)
	}

	s.resultsMutex.Lock()
	if i, ok := s.resultIndex[req.URL]; ok {
		s.results[i].StreamInfo = streamInfo
		s.results[i].ScannedAt = time.Now()
	}
	s.resultsMutex.Unlock()

	s.broadcast("progress")
	jsonResponse(w, map[string]interface{}{
		"success":     true,
		"stream_info": streamInfo,
	})
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	s.resultsMutex.RLock()
	results := append([]scanner.ScanResult(nil), s.results...)
	progress := s.progress
	s.resultsMutex.RUnlock()

	s.scanMutex.Lock()
	scanning := s.scanning
	s.scanMutex.Unlock()

	jsonResponse(w, map[string]interface{}{
		"results":  results,
		"progress": progress,
		"scanning": scanning,
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	s.resultsMutex.RLock()
	results := append([]scanner.ScanResult(nil), s.results...)
	s.resultsMutex.RUnlock()

	w.Header().Set("Content-Type", "application/x-mpegurl")
	w.Header().Set("Content-Disposition", "attachment; filename=available_channels.m3u")

	_, _ = w.Write([]byte("#EXTM3U\n"))
	for _, result := range results {
		if result.StreamInfo == nil || !result.StreamInfo.Available {
			continue
		}
		ch := result.Channel
		line := "#EXTINF:-1"
		if ch.TVGId != "" {
			line += fmt.Sprintf(` tvg-id="%s"`, ch.TVGId)
		}
		if ch.Logo != "" {
			line += fmt.Sprintf(` tvg-logo="%s"`, ch.Logo)
		}
		if ch.GroupTitle != "" {
			line += fmt.Sprintf(` group-title="%s"`, ch.GroupTitle)
		}
		line += fmt.Sprintf(",%s\n%s\n", ch.Name, ch.URL)
		_, _ = w.Write([]byte(line))
	}
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	s.resultsMutex.RLock()
	results := append([]scanner.ScanResult(nil), s.results...)
	s.resultsMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=scan_results.json")
	_ = json.NewEncoder(w).Encode(results)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	s.resultsMutex.RLock()
	results := append([]scanner.ScanResult(nil), s.results...)
	s.resultsMutex.RUnlock()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=scan_results.csv")

	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	_, _ = w.Write([]byte("name,url,available,resolution,codec,bitrate,response_time\n"))

	for _, result := range results {
		ch := result.Channel
		info := result.StreamInfo

		available := "false"
		resolution := ""
		codec := ""
		bitrate := "0"
		responseTime := "0"

		if info != nil {
			if info.Available {
				available = "true"
				responseTime = fmt.Sprintf("%d", info.ResponseTime/1000000)
			}

			if len(info.VideoStreams) > 0 {
				v := info.VideoStreams[0]
				fo := strings.ToLower(v.FieldOrder)
				isInterlaced := fo == "tt" || fo == "bb" || fo == "tb" || fo == "bt"
				if isInterlaced {
					resolution = fmt.Sprintf("%dx%di", v.Width, v.Height)
				} else {
					resolution = fmt.Sprintf("%dx%dp", v.Width, v.Height)
				}
				codec = v.Codec
				bitrate = fmt.Sprintf("%d", v.BitRate)
			}
		}

		line := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s\n",
			escapeCSV(ch.Name),
			escapeCSV(ch.URL),
			available,
			resolution,
			codec,
			bitrate,
			responseTime,
		)
		_, _ = w.Write([]byte(line))
	}
}

func escapeCSV(s string) string {
	if containsAny(s, ",\"\n\r") {
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(s, "\"", "\"\""))
	}
	return s
}

func containsAny(s, chars string) bool {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return true
			}
		}
	}
	return false
}

func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	streamURL := r.URL.Query().Get("url")
	player := r.URL.Query().Get("player")

	if streamURL == "" {
		jsonError(w, "需要URL参数", http.StatusBadRequest)
		return
	}

	var cmd *exec.Cmd
	var err error

	switch player {
	case "potplayer":
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", "start", "", "potplayer://"+streamURL)
		}
	case "vlc":
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", "start", "", "vlc://"+streamURL)
		} else if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", "-a", "VLC", streamURL)
		} else {
			cmd = exec.Command("vlc", streamURL)
		}
	case "iina":
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", "-a", "IINA", "--args", "--url="+streamURL)
		}
	case "mpv":
		cmd = exec.Command("mpv", streamURL)
	case "nplayer":
		cmd = exec.Command("open", "nplayer-"+streamURL)
	case "infuse":
		cmd = exec.Command("open", "infuse://x-callback-url/play?url="+streamURL)
	case "mxplayer":
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", "start", "", "intent://"+streamURL+"#Intent;package=com.mxtech.videoplayer.ad;end")
		}
	case "kodi":
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", "start", "", "kodi://"+streamURL)
		} else {
			cmd = exec.Command("kodi", streamURL)
		}
	default:
		switch runtime.GOOS {
		case "windows":
			for _, p := range []string{"vlc", "mpv", "ffplay"} {
				if _, err := exec.LookPath(p); err == nil {
					cmd = exec.Command(p, streamURL)
					break
				}
			}
			if cmd == nil {
				cmd = exec.Command("cmd", "/c", "start", "", streamURL)
			}
		case "darwin":
			cmd = exec.Command("open", streamURL)
		default:
			cmd = exec.Command("xdg-open", streamURL)
		}
	}

	if cmd == nil {
		jsonError(w, "不支持此播放器", http.StatusBadRequest)
		return
	}

	err = cmd.Start()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]bool{"success": true})
}

// handleThumbnail generates and returns a thumbnail for a stream
func (s *Server) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	streamURL := r.URL.Query().Get("url")
	if streamURL == "" {
		jsonError(w, "需要URL参数", http.StatusBadRequest)
		return
	}
	if !isAllowedURL(streamURL) {
		jsonError(w, "URL 不合法", http.StatusBadRequest)
		return
	}

	s.thumbMutex.RLock()
	if thumb, ok := s.thumbnails[streamURL]; ok {
		s.thumbMutex.RUnlock()
		jsonResponse(w, map[string]string{"thumbnail": thumb})
		return
	}
	s.thumbMutex.RUnlock()

	thumb, err := s.generateThumbnail(streamURL)
	if err != nil {
		jsonError(w, "生成缩略图失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.thumbMutex.Lock()
	s.thumbnails[streamURL] = thumb
	s.thumbMutex.Unlock()

	jsonResponse(w, map[string]string{"thumbnail": thumb})
}

func (s *Server) generateThumbnail(streamURL string) (string, error) {
	hash := md5.Sum([]byte(streamURL))
	filename := fmt.Sprintf("%x.jpg", hash)
	outputPath := filepath.Join(s.thumbDir, filename)

	if _, err := os.Stat(outputPath); err == nil {
		data, err := os.ReadFile(outputPath)
		if err == nil {
			return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
		}
	}

	ffmpegPath := getFFmpegPath()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if !ffprobe.AcquireSemaphore(ctx) {
		return "", ctx.Err()
	}
	defer ffprobe.ReleaseSemaphore()

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-i", streamURL,
		"-ss", "00:00:02",
		"-vframes", "1",
		"-vf", "scale=1280:-1",
		"-q:v", "2",
		"-y",
		outputPath,
	)

	if err := cmd.Run(); err != nil {
		return "", err
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return "", err
	}

	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
}

// handleStream proxies the stream for web playback
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	streamURL := r.URL.Query().Get("url")
	if streamURL == "" {
		http.Error(w, "需要URL参数", http.StatusBadRequest)
		return
	}
	if !isAllowedURL(streamURL) {
		http.Error(w, "URL 不合法", http.StatusBadRequest)
		return
	}

	// Use configured User-Agent, fallback to a common one
	ua := s.settings.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	}

	// Retry logic for transient failures
	var resp *http.Response
	var err error
	maxRetries := 2
	for attempt := 0; attempt <= maxRetries; attempt++ {
		client := &http.Client{Timeout: 30 * time.Second}
		req, reqErr := http.NewRequest("GET", streamURL, nil)
		if reqErr != nil {
			http.Error(w, reqErr.Error(), http.StatusInternalServerError)
			return
		}

		req.Header.Set("User-Agent", ua)
		// Forward Referer to satisfy CDN restrictions
		if ref := r.Header.Get("Referer"); ref != "" {
			req.Header.Set("Referer", ref)
		} else {
			req.Header.Set("Referer", streamURL)
		}

		resp, err = client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break
		}
		if resp != nil {
			resp.Body.Close()
			resp = nil
		}
		if attempt < maxRetries {
			time.Sleep(time.Duration(500*(attempt+1)) * time.Millisecond)
		}
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if resp == nil {
		http.Error(w, "upstream returned error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	isM3U8 := strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") ||
		strings.HasSuffix(strings.ToLower(streamURL), ".m3u8")

	if isM3U8 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return
		}
		rewritten := rewriteM3U8Segments(string(body), streamURL)
		
		for k, v := range resp.Header {
			if strings.ToLower(k) == "content-length" {
				continue
			}
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(rewritten)))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write([]byte(rewritten))
	} else {
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

// rewriteM3U8Segments rewrites relative segment URLs in an m3u8 playlist
// to use the /api/stream proxy, so the browser can fetch them without CORS issues.
func rewriteM3U8Segments(content string, baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return content
	}
	baseDir := parsed.Scheme + "://" + parsed.Host
	if idx := strings.LastIndex(parsed.Path, "/"); idx >= 0 {
		baseDir += parsed.Path[:idx+1]
	}

	resolveURL := func(raw string) string {
		raw = strings.TrimSpace(raw)
		raw = strings.Trim(raw, "\"'")
		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
			return "/api/stream?url=" + url.QueryEscape(raw)
		}
		return "/api/stream?url=" + url.QueryEscape(baseDir+raw)
	}

	var result strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Handle #EXT-X-KEY and similar directives with URI attributes
		if strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, "URI=") {
			rewritten := rewriteURIAttribute(trimmed, resolveURL)
			result.WriteString(rewritten)
			result.WriteString("\n")
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// This is a URL line (segment or sub-playlist)
		result.WriteString(resolveURL(trimmed))
		result.WriteString("\n")
	}
	return result.String()
}

// rewriteURIAttribute rewrites URI="..." values in HLS directives like #EXT-X-KEY
func rewriteURIAttribute(line string, resolveURL func(string) string) string {
	idx := strings.Index(line, "URI=")
	if idx < 0 {
		return line
	}

	// Find the URI value (quoted or unquoted)
	uriStart := idx + 4
	if uriStart >= len(line) {
		return line
	}

	quote := byte(0)
	if line[uriStart] == '"' || line[uriStart] == '\'' {
		quote = line[uriStart]
		uriStart++
	}

	// Find end of URI value
	uriEnd := uriStart
	for uriEnd < len(line) {
		if quote != 0 && line[uriEnd] == quote {
			break
		}
		if quote == 0 && (line[uriEnd] == ',' || line[uriEnd] == ' ' || line[uriEnd] == '\n') {
			break
		}
		uriEnd++
	}

	uriValue := line[uriStart:uriEnd]
	proxied := resolveURL(uriValue)

	// Reconstruct the line
	var b strings.Builder
	b.WriteString(line[:idx+4])
	if quote != 0 {
		b.WriteByte(quote)
	}
	b.WriteString(proxied)
	if quote != 0 {
		b.WriteByte(quote)
	}
	b.WriteString(line[uriEnd:])
	return b.String()
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.resultsMutex.RLock()
		settings := s.settings
		s.resultsMutex.RUnlock()
		jsonResponse(w, settings)
		return
	}

	if r.Method == http.MethodPost {
		var newSettings Settings
		if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
			jsonError(w, "无效设置", http.StatusBadRequest)
			return
		}

		if newSettings.Concurrency <= 0 {
			newSettings.Concurrency = 20
		}
		if newSettings.Concurrency > 100 {
			newSettings.Concurrency = 100
		}
		if newSettings.Timeout <= 0 {
			newSettings.Timeout = 15
		}
		if newSettings.Timeout > 120 {
			newSettings.Timeout = 120
		}

		s.resultsMutex.Lock()
		s.settings = newSettings
		s.resultsMutex.Unlock()
		s.saveSettings()

		jsonResponse(w, newSettings)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]interface{}{
		"ffprobe":     ffprobe.IsFFprobeAvailable(),
		"ffprobePath": ffprobe.GetFFprobePath(),
	})
}

// SSE for real-time updates
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 10)
	s.clientsMutex.Lock()
	s.clients[ch] = true
	s.clientsMutex.Unlock()

	defer func() {
		s.clientsMutex.Lock()
		delete(s.clients, ch)
		s.clientsMutex.Unlock()
		close(ch)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case msg := <-ch:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) broadcast(event string) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()
	for ch := range s.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func openBrowser(appURL string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", appURL).Start()
	case "darwin":
		err = exec.Command("open", appURL).Start()
	default:
		err = exec.Command("xdg-open", appURL).Start()
	}
	if err != nil {
		log.Printf("打开浏览器失败: %v", err)
	}
}

// getFFmpegPath finds ffmpeg executable
func getFFmpegPath() string {
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name = "ffmpeg.exe"
	}

	if exePath, err := os.Executable(); err == nil {
		localPath := filepath.Join(filepath.Dir(exePath), name)
		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		localPath := filepath.Join(cwd, name)
		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}

	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	return name
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// isAllowedURL checks that the URL is http(s) and does not point to a private/internal IP
func isAllowedURL(raw string) bool {
	if !isHTTPURL(raw) {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	// Check if it's an IP literal
	if ip := net.ParseIP(host); ip != nil {
		return !isPrivateIP(ip)
	}
	// For hostnames, resolve and check all IPs
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return false
		}
	}
	return true
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
