package scanner

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"m3u-scanner/internal/ffprobe"
	"m3u-scanner/internal/parser"
)

// ScanResult represents the result of scanning a single channel
type ScanResult struct {
	Channel    parser.Channel      `json:"channel"`
	StreamInfo *ffprobe.StreamInfo `json:"stream_info"`
	ScannedAt  time.Time           `json:"scanned_at"`
}

// ScanProgress represents the current scanning progress
type ScanProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Available int `json:"available"`
	Failed    int `json:"failed"`
}

// Scanner handles concurrent stream scanning
type Scanner struct {
	concurrency int
	timeout     time.Duration
	quickCheck  bool
}

// NewScanner creates a new scanner instance
func NewScanner(concurrency int, timeout time.Duration, quickCheck bool) *Scanner {
	if concurrency <= 0 {
		concurrency = 10
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Scanner{
		concurrency: concurrency,
		timeout:     timeout,
		quickCheck:  quickCheck,
	}
}

// Scan scans all channels in the playlist
func (s *Scanner) Scan(ctx context.Context, playlist *parser.M3UPlaylist,
	progressCh chan<- ScanProgress, resultCh chan<- ScanResult) error {

	total := len(playlist.Channels)
	if total == 0 {
		if progressCh != nil {
			close(progressCh)
		}
		if resultCh != nil {
			close(resultCh)
		}
		return nil
	}

	ffprobe.InitConcurrency(s.concurrency)

	var completed, available, failed int32
	var wg sync.WaitGroup
	// 本地信号量控制扫描 goroutine 并发数
	semaphore := make(chan struct{}, s.concurrency)

	// 使用 cancelled 标记，而不是直接 return
	cancelled := false

	for _, channel := range playlist.Channels {
		// 检查是否取消，但不立即返回
		select {
		case <-ctx.Done():
			cancelled = true
		default:
		}

		if cancelled {
			break // 退出循环，但继续等待已启动的goroutine完成
		}

		// 获取信号量（限制并发启动数）
		select {
		case <-ctx.Done():
			cancelled = true
		case semaphore <- struct{}{}:
		}

		if cancelled {
			break
		}

		wg.Add(1)
		go func(ch parser.Channel) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// 检查是否已取消
			select {
			case <-ctx.Done():
				return
			default:
			}

			var streamInfo *ffprobe.StreamInfo
			if s.quickCheck {
				ok, responseTime, err := ffprobe.CheckAvailability(ch.URL, s.timeout)
				streamInfo = &ffprobe.StreamInfo{
					Available:    ok,
					ResponseTime: responseTime,
				}
				if err != nil {
					streamInfo.Error = err.Error()
				}
			} else {
				streamInfo = ffprobe.Probe(ch.URL, s.timeout)
			}

			// 再次检查是否取消，避免向已关闭的channel发送
			select {
			case <-ctx.Done():
				return
			default:
			}

			result := ScanResult{
				Channel:    ch,
				StreamInfo: streamInfo,
				ScannedAt:  time.Now(),
			}

			if resultCh != nil {
				select {
				case resultCh <- result:
				case <-ctx.Done():
					return
				}
			}

			atomic.AddInt32(&completed, 1)
			if streamInfo.Available {
				atomic.AddInt32(&available, 1)
			} else {
				atomic.AddInt32(&failed, 1)
			}

			if progressCh != nil {
				select {
				case progressCh <- ScanProgress{
					Total:     total,
					Completed: int(atomic.LoadInt32(&completed)),
					Available: int(atomic.LoadInt32(&available)),
					Failed:    int(atomic.LoadInt32(&failed)),
				}:
				default:
					// 非阻塞发送，如果channel满了就跳过
				}
			}
		}(channel)
	}

	// 始终等待所有goroutine完成后再关闭channel
	wg.Wait()

	if progressCh != nil {
		close(progressCh)
	}
	if resultCh != nil {
		close(resultCh)
	}

	if cancelled {
		return ctx.Err()
	}
	return nil
}

// ScanResults holds all scan results
type ScanResults struct {
	Results   []ScanResult `json:"results"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
	Total     int          `json:"total"`
	Available int          `json:"available"`
	Failed    int          `json:"failed"`
}

// ScanSync performs a synchronous scan and returns all results
func (s *Scanner) ScanSync(ctx context.Context, playlist *parser.M3UPlaylist,
	progressCh chan<- ScanProgress) (*ScanResults, error) {

	startTime := time.Now()
	resultCh := make(chan ScanResult, len(playlist.Channels))

	err := s.Scan(ctx, playlist, progressCh, resultCh)
	if err != nil {
		return nil, err
	}

	results := &ScanResults{
		Results:   make([]ScanResult, 0, len(playlist.Channels)),
		StartTime: startTime,
		EndTime:   time.Now(),
		Total:     len(playlist.Channels),
	}

	for result := range resultCh {
		results.Results = append(results.Results, result)
		if result.StreamInfo != nil && result.StreamInfo.Available {
			results.Available++
		} else {
			results.Failed++
		}
	}

	return results, nil
}
