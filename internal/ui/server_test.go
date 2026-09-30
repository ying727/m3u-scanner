package ui

// 给 web 接口补的接口测试：直接调 s.handler()，不占端口也不弹浏览器。
// 只覆盖"不用真发网络请求"的路径（比如 load-url 只测到 URL 校验那一步）。

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testM3U = `#EXTM3U
#EXTINF:-1 group-title="测试" tvg-logo="http://example.com/logo.png",频道一
http://example.com/stream1.m3u8
#EXTINF:-1 group-title="测试",频道二
http://example.com/stream2.m3u8
`

func doAPI(t *testing.T, s *Server, method, path string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, body)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	s.handler().ServeHTTP(rr, req)
	return rr
}

func postJSON(t *testing.T, s *Server, path, payload string) *httptest.ResponseRecorder {
	t.Helper()
	return doAPI(t, s, http.MethodPost, path, bytes.NewBufferString(payload), "application/json")
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%q", err, rr.Body.String())
	}
	return m
}

func uploadFile(t *testing.T, s *Server, filename, content string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return doAPI(t, s, http.MethodPost, "/api/upload", &buf, w.FormDataContentType())
}

// 上传一个能解析的 m3u，看返回的频道数对不对
func TestUploadValidM3U(t *testing.T) {
	s := NewServer()
	rr := uploadFile(t, s, "test.m3u", testM3U)

	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d, body=%q", rr.Code, rr.Body.String())
	}
	m := decodeBody(t, rr)
	if m["success"] != true {
		t.Errorf("期望 success=true, 实际 %v", m["success"])
	}
	if m["channels"] != float64(2) {
		t.Errorf("期望 channels=2, 实际 %v", m["channels"])
	}
}

// 上传时不带文件，应该 400 而不是 500
func TestUploadNoFile(t *testing.T) {
	s := NewServer()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("note", "no file here")
	_ = w.Close()
	rr := doAPI(t, s, http.MethodPost, "/api/upload", &buf, w.FormDataContentType())

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("期望 400, 实际 %d", rr.Code)
	}
	if decodeBody(t, rr)["error"] == nil {
		t.Error("期望返回里带 error 字段")
	}
}

// upload 只接受 POST
func TestUploadMethodNotAllowed(t *testing.T) {
	s := NewServer()
	rr := doAPI(t, s, http.MethodGet, "/api/upload", nil, "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("期望 405, 实际 %d", rr.Code)
	}
}

// 传非法 JSON，应该 400
func TestLoadURLBadJSON(t *testing.T) {
	s := NewServer()
	rr := postJSON(t, s, "/api/load-url", "this is not json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("期望 400, 实际 %d", rr.Code)
	}
}

// 内网 IP / 非 http(s) 协议的 URL 直接拦掉，不真发请求
func TestLoadURLRejected(t *testing.T) {
	s := NewServer()
	for _, u := range []string{
		"http://192.168.1.1/list.m3u",
		"http://127.0.0.1:8080/x.m3u",
		"ftp://example.com/list.m3u",
		"not a url",
		"",
	} {
		payload, _ := json.Marshal(map[string]string{"url": u})
		rr := postJSON(t, s, "/api/load-url", string(payload))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("url=%q 期望 400, 实际 %d", u, rr.Code)
		}
	}
}

// 没加载播放列表就点开始扫描，应该 400
func TestScanStartWithoutPlaylist(t *testing.T) {
	s := NewServer()
	rr := postJSON(t, s, "/api/scan/start", "{}")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("期望 400, 实际 %d", rr.Code)
	}
	m := decodeBody(t, rr)
	if !strings.Contains(m["error"].(string), "未加载播放列表") {
		t.Errorf("错误信息不对: %v", m["error"])
	}
}

// 没在扫描时点停止，应该 200（幂等）
func TestScanStopWhenIdle(t *testing.T) {
	s := NewServer()
	rr := postJSON(t, s, "/api/scan/stop", "{}")
	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d", rr.Code)
	}
	if decodeBody(t, rr)["success"] != true {
		t.Error("期望 success=true")
	}
}

// 读默认设置
func TestGetSettings(t *testing.T) {
	s := NewServer()
	rr := doAPI(t, s, http.MethodGet, "/api/settings", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d", rr.Code)
	}
	m := decodeBody(t, rr)
	if m["concurrency"] != float64(20) {
		t.Errorf("默认并发期望 20, 实际 %v", m["concurrency"])
	}
	if m["timeout"] != float64(15) {
		t.Errorf("默认超时期望 15, 实际 %v", m["timeout"])
	}
}

// 改设置，正常值原样存
func TestPostSettingsValid(t *testing.T) {
	s := NewServer()
	rr := postJSON(t, s, "/api/settings", `{"concurrency":30,"timeout":20,"quickCheck":true,"userAgent":"test-agent"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d", rr.Code)
	}
	m := decodeBody(t, rr)
	if m["concurrency"] != float64(30) || m["timeout"] != float64(20) {
		t.Errorf("设置没存进去: %v", m)
	}
	if m["quickCheck"] != true || m["userAgent"] != "test-agent" {
		t.Errorf("字段不对: %v", m)
	}
}

// 超限的值要被 clamp，不能让前端传个 9999 把机器打爆
func TestPostSettingsClamped(t *testing.T) {
	s := NewServer()
	rr := postJSON(t, s, "/api/settings", `{"concurrency":500,"timeout":500}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d", rr.Code)
	}
	m := decodeBody(t, rr)
	if m["concurrency"] != float64(100) {
		t.Errorf("并发期望被 clamp 到 100, 实际 %v", m["concurrency"])
	}
	if m["timeout"] != float64(120) {
		t.Errorf("超时期望被 clamp 到 120, 实际 %v", m["timeout"])
	}
}

// 设置接口传非法 JSON，应该 400
func TestPostSettingsBadJSON(t *testing.T) {
	s := NewServer()
	rr := postJSON(t, s, "/api/settings", "{bad json")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("期望 400, 实际 %d", rr.Code)
	}
}

// 空状态下拉结果列表
func TestGetResultsEmpty(t *testing.T) {
	s := NewServer()
	rr := doAPI(t, s, http.MethodGet, "/api/results", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d", rr.Code)
	}
	m := decodeBody(t, rr)
	if m["scanning"] != false {
		t.Errorf("期望 scanning=false, 实际 %v", m["scanning"])
	}
	if _, ok := m["results"]; !ok {
		t.Error("期望返回里带 results 字段")
	}
}

// 一条龙：上传 -> 查结果里能看到两个频道
func TestUploadThenResults(t *testing.T) {
	s := NewServer()
	uploadFile(t, s, "test.m3u", testM3U)

	rr := doAPI(t, s, http.MethodGet, "/api/results", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d", rr.Code)
	}
	m := decodeBody(t, rr)
	results, ok := m["results"].([]interface{})
	if !ok {
		t.Fatalf("results 不是数组: %v", m["results"])
	}
	if len(results) != 2 {
		t.Fatalf("期望 2 条结果, 实际 %d", len(results))
	}
	first := results[0].(map[string]interface{})["channel"].(map[string]interface{})
	if first["name"] != "频道一" {
		t.Errorf("第一个频道名不对: %v", first["name"])
	}
}

// 清空结果
func TestClearResults(t *testing.T) {
	s := NewServer()
	uploadFile(t, s, "test.m3u", testM3U)
	rr := postJSON(t, s, "/api/results/clear", "{}")
	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d", rr.Code)
	}
	rr = doAPI(t, s, http.MethodGet, "/api/results", nil, "")
	if n := len(decodeBody(t, rr)["results"].([]interface{})); n != 0 {
		t.Errorf("清空后期望 0 条, 实际 %d", n)
	}
}

// 没加载列表就重命名，应该 400
func TestRenameWithoutPlaylist(t *testing.T) {
	s := NewServer()
	rr := postJSON(t, s, "/api/rename", `{"find":"a","replace":"b"}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("期望 400, 实际 %d", rr.Code)
	}
}

// 没加载列表就去重，应该 400
func TestDeduplicateWithoutPlaylist(t *testing.T) {
	s := NewServer()
	rr := postJSON(t, s, "/api/deduplicate", "{}")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("期望 400, 实际 %d", rr.Code)
	}
}

// 状态接口，至少能通
func TestStatus(t *testing.T) {
	s := NewServer()
	rr := doAPI(t, s, http.MethodGet, "/api/status", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d", rr.Code)
	}
	if _, ok := decodeBody(t, rr)["ffprobe"]; !ok {
		t.Error("期望返回里带 ffprobe 字段")
	}
}

// 导出接口，空结果时也该 200 且是 m3u 格式头
func TestExportEmpty(t *testing.T) {
	s := NewServer()
	rr := doAPI(t, s, http.MethodGet, "/api/export", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/x-mpegurl" {
		t.Errorf("Content-Type 期望 application/x-mpegurl, 实际 %s", ct)
	}
	if !strings.HasPrefix(rr.Body.String(), "#EXTM3U") {
		t.Errorf("body 应该以 #EXTM3U 开头, 实际 %q", rr.Body.String())
	}
}
