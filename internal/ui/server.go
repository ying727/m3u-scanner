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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

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
	resultsMutex sync.RWMutex
	scanning     bool
	scanMutex    sync.Mutex
	cancelFunc   context.CancelFunc
	progress     scanner.ScanProgress
	clients      map[chan string]bool
	clientsMutex sync.Mutex
	settings     Settings
	thumbnails   map[string]string // URL -> base64 thumbnail
	thumbMutex   sync.RWMutex
	thumbDir     string
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
	os.MkdirAll(thumbDir, 0755)

	return &Server{
		results:    make([]scanner.ScanResult, 0),
		clients:    make(map[chan string]bool),
		settings:   Settings{Concurrency: 20, Timeout: 15, QuickCheck: false},
		thumbnails: make(map[string]string),
		thumbDir:   thumbDir,
	}
}

// Run starts the web server
func (s *Server) Run(port int) error {
	// API routes
	http.HandleFunc("/api/upload", s.handleUpload)
	http.HandleFunc("/api/load-url", s.handleLoadURL)
	http.HandleFunc("/api/scan/start", s.handleStartScan)
	http.HandleFunc("/api/scan/stop", s.handleStopScan)
	http.HandleFunc("/api/results", s.handleResults)
	http.HandleFunc("/api/export", s.handleExport)
	http.HandleFunc("/api/export/json", s.handleExportJSON)
	http.HandleFunc("/api/export/csv", s.handleExportCSV)
	http.HandleFunc("/api/play", s.handlePlay)
	http.HandleFunc("/api/settings", s.handleSettings)
	http.HandleFunc("/api/status", s.handleStatus)
	http.HandleFunc("/api/events", s.handleSSE)
	http.HandleFunc("/api/thumbnail", s.handleThumbnail)
	http.HandleFunc("/api/stream", s.handleStream)
	http.HandleFunc("/api/scan/single", s.handleScanSingle)

	// Static files
	http.HandleFunc("/", s.handleStatic)

	addr := fmt.Sprintf(":%d", port)
	url := fmt.Sprintf("http://localhost:%d", port)

	fmt.Printf("M3U Scanner 已启动: %s\n", url)
	fmt.Println("按 Ctrl+C 停止服务")

	// Open browser
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	content, err := staticFiles.ReadFile("static" + path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type
	switch {
	case len(path) > 5 && path[len(path)-5:] == ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case len(path) > 4 && path[len(path)-4:] == ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case len(path) > 3 && path[len(path)-3:] == ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}

	w.Write(content)
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

	// Create temp file
	tmp, err := os.CreateTemp("", "m3u-*.m3u")
	if err != nil {
		jsonError(w, "创建临时文件失败", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())

	io.Copy(tmp, file)
	tmp.Close()

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
	defer s.resultsMutex.Unlock()

	s.playlist = playlist
	s.results = make([]scanner.ScanResult, len(playlist.Channels))
	for i, ch := range playlist.Channels {
		s.results[i] = scanner.ScanResult{Channel: ch}
	}
	s.progress = scanner.ScanProgress{Total: len(playlist.Channels)}

	// Clear old thumbnails
	s.thumbMutex.Lock()
	s.thumbnails = make(map[string]string)
	s.thumbMutex.Unlock()
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
	s.scanning = true
	s.scanMutex.Unlock()

	s.resultsMutex.RLock()
	if s.playlist == nil {
		s.resultsMutex.RUnlock()
		s.scanMutex.Lock()
		s.scanning = false
		s.scanMutex.Unlock()
		jsonError(w, "未加载播放列表", http.StatusBadRequest)
		return
	}
	s.resultsMutex.RUnlock()

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel

	go s.runScan(ctx)

	jsonResponse(w, map[string]bool{"success": true})
}

func (s *Server) runScan(ctx context.Context) {
	defer func() {
		s.scanMutex.Lock()
		s.scanning = false
		s.scanMutex.Unlock()
		s.broadcast("scan_complete")
	}()

	s.resultsMutex.RLock()
	concurrency := s.settings.Concurrency
	timeout := s.settings.Timeout
	quickCheck := s.settings.QuickCheck
	playlistLen := len(s.playlist.Channels)
	s.resultsMutex.RUnlock()

	// Validate concurrency
	if concurrency <= 0 {
		concurrency = 20
	}
	if concurrency > 100 {
		concurrency = 100
	}

	sc := scanner.NewScanner(
		concurrency,
		time.Duration(timeout)*time.Second,
		quickCheck,
	)

	progressCh := make(chan scanner.ScanProgress, 100)
	resultCh := make(chan scanner.ScanResult, playlistLen)

	// Handle progress updates
	go func() {
		for p := range progressCh {
			s.resultsMutex.Lock()
			s.progress = p
			s.resultsMutex.Unlock()
			s.broadcast("progress")
		}
	}()

	// Handle results
	go func() {
		for result := range resultCh {
			s.resultsMutex.Lock()
			for i := range s.results {
				if s.results[i].Channel.URL == result.Channel.URL {
					s.results[i] = result
					break
				}
			}
			s.resultsMutex.Unlock()
		}
	}()

	s.resultsMutex.RLock()
	playlist := s.playlist
	s.resultsMutex.RUnlock()

	sc.Scan(ctx, playlist, progressCh, resultCh)
}

func (s *Server) handleStopScan(w http.ResponseWriter, r *http.Request) {
	if s.cancelFunc != nil {
		s.cancelFunc()
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

	// Get current settings
	s.resultsMutex.RLock()
	timeout := s.settings.Timeout
	quickCheck := s.settings.QuickCheck
	s.resultsMutex.RUnlock()

	// Scan the single URL
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

	// Update the result in our results slice
	s.resultsMutex.Lock()
	for i := range s.results {
		if s.results[i].Channel.URL == req.URL {
			s.results[i].StreamInfo = streamInfo
			s.results[i].ScannedAt = time.Now()
			break
		}
	}
	s.resultsMutex.Unlock()

	// Broadcast update
	s.broadcast("progress")

	jsonResponse(w, map[string]interface{}{
		"success":     true,
		"stream_info": streamInfo,
	})
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	s.resultsMutex.RLock()
	defer s.resultsMutex.RUnlock()

	s.scanMutex.Lock()
	scanning := s.scanning
	s.scanMutex.Unlock()

	jsonResponse(w, map[string]interface{}{
		"results":  s.results,
		"progress": s.progress,
		"scanning": scanning,
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	s.resultsMutex.RLock()
	defer s.resultsMutex.RUnlock()

	w.Header().Set("Content-Type", "application/x-mpegurl")
	w.Header().Set("Content-Disposition", "attachment; filename=available_channels.m3u")

	w.Write([]byte("#EXTM3U\n"))
	for _, result := range s.results {
		if result.StreamInfo == nil || !result.StreamInfo.Available {
			continue
		}
		ch := result.Channel
		line := fmt.Sprintf("#EXTINF:-1")
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
		w.Write([]byte(line))
	}
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	s.resultsMutex.RLock()
	defer s.resultsMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=scan_results.json")
	json.NewEncoder(w).Encode(s.results)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	s.resultsMutex.RLock()
	defer s.resultsMutex.RUnlock()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=scan_results.csv")

	// Write BOM for Excel compatibility
	w.Write([]byte("\xEF\xBB\xBF"))

	w.Write([]byte("name,url,available,resolution,codec,bitrate,response_time\n"))

	for _, result := range s.results {
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
				responseTime = fmt.Sprintf("%d", info.ResponseTime/1000000) // ms
			}

			if len(info.VideoStreams) > 0 {
				v := info.VideoStreams[0]
				resolution = fmt.Sprintf("%dx%d", v.Width, v.Height)
				codec = v.Codec
				bitrate = fmt.Sprintf("%d", v.BitRate)
			}
		}

		// Escape CSV fields
		line := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s\n",
			escapeCSV(ch.Name),
			escapeCSV(ch.URL),
			available,
			resolution,
			codec,
			bitrate,
			responseTime,
		)
		w.Write([]byte(line))
	}
}

func escapeCSV(s string) string {
	if containsAny(s, ",\"\n\r") {
		return fmt.Sprintf("\"%s\"", replaceAll(s, "\"", "\"\""))
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

func replaceAll(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
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
		// PotPlayer URL scheme
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
		// nPlayer URL scheme (iOS/macOS)
		cmd = exec.Command("open", "nplayer-"+streamURL)
	case "infuse":
		// Infuse URL scheme
		cmd = exec.Command("open", "infuse://x-callback-url/play?url="+streamURL)
	case "mxplayer":
		// MX Player (Android)
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", "start", "", "intent://"+streamURL+"#Intent;package=com.mxtech.videoplayer.ad;end")
		}
	case "kodi":
		// Kodi JSON-RPC
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", "start", "", "kodi://"+streamURL)
		} else {
			cmd = exec.Command("kodi", streamURL)
		}
	default:
		// Default: try common players
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

	// Check cache
	s.thumbMutex.RLock()
	if thumb, ok := s.thumbnails[streamURL]; ok {
		s.thumbMutex.RUnlock()
		jsonResponse(w, map[string]string{"thumbnail": thumb})
		return
	}
	s.thumbMutex.RUnlock()

	// Generate thumbnail using ffmpeg
	thumb, err := s.generateThumbnail(streamURL)
	if err != nil {
		jsonError(w, "生成缩略图失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Cache it
	s.thumbMutex.Lock()
	s.thumbnails[streamURL] = thumb
	s.thumbMutex.Unlock()

	jsonResponse(w, map[string]string{"thumbnail": thumb})
}

func (s *Server) generateThumbnail(streamURL string) (string, error) {
	// Create unique filename
	hash := md5.Sum([]byte(streamURL))
	filename := fmt.Sprintf("%x.jpg", hash)
	outputPath := filepath.Join(s.thumbDir, filename)

	// Check if already exists
	if _, err := os.Stat(outputPath); err == nil {
		data, err := os.ReadFile(outputPath)
		if err == nil {
			return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
		}
	}

	// Get ffmpeg path (same directory logic as ffprobe)
	ffmpegPath := getFFmpegPath()

	// Generate thumbnail
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 使用全局信号量控制ffmpeg并发
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

	// Read and encode
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

	// Proxy the request
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	// Enable CORS for video playback
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
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
			jsonError(w, "无效的设置", http.StatusBadRequest)
			return
		}

		// Validate
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
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
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
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
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

	// Check same directory as executable
	if exePath, err := os.Executable(); err == nil {
		localPath := filepath.Join(filepath.Dir(exePath), name)
		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}

	// Check current working directory
	if cwd, err := os.Getwd(); err == nil {
		localPath := filepath.Join(cwd, name)
		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}

	// Fall back to PATH
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	return name
}
