# M3U Scanner

[![CI](https://github.com/ying727/m3u-scanner/actions/workflows/ci.yml/badge.svg)](https://github.com/ying727/m3u-scanner/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ying727/m3u-scanner?display_name=tag)](https://github.com/ying727/m3u-scanner/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个轻量、纯 Go 编写的 IPTV (M3U / M3U8) 直播源批量扫描、流媒体深度探测与列表清洗工具。

前端界面通过 Go `embed` 直接打进单个可执行文件，无任何运行时或 CGO 依赖，双击即跑。

---

## 起因（为啥写这个）

家里电视盒子和电脑用播放器看电视，网上抓回来的直播源列表动辄几百上千条，实际上 70%~80% 要么是死链，要么是卡成幻灯片的低清废源。

之前找过一些现成工具，体验都很割裂：
- 命令行脚本跑完只给一堆冷冰冰的 txt/m3u，看不到流的具体参数；
- 某些带有 GUI 的工具体积几十兆还塞广告，甚至不支持 HLS 跨域代理；
- 遇到需要特定 Referer / User-Agent 防盗链的源直接阵亡。

于是干脆抽空用 Go 写了这个轻量级小工具：把列表拖进去点开始，它会自动并发测活、用 ffprobe 抓编码和真实码率、截取预览图、拉取 EPG 节目单。扫完把能播的高清源一键导出，随手就能丢给 PotPlayer、VLC 或者电视盒子。

---

## 核心功能

- **多源导入与容错解析**：支持本地 `.m3u` / `.m3u8` 文件拖拽上传或远程 URL 拉取。针对不规范换行、带引号逗号的频道名（EXTINF 属性混乱）做了流式解析兼容。
- **两阶段并发探活**：带信号量限制的 Goroutine Worker Pool，先走 HTTP HEAD 嗅探，对端拒绝时平滑降级 GET 首分片，并发度与超时均可动态调节。
- **音视频深度探测（可选 FFprobe）**：外部子进程安全调度，通过 packet 采样估算直播流的实时动态码率、判断 CBR/VBR，精准提取分辨率、HDR10 / 杜比视界及音频流编码。
- **画面实时截图（可选 FFmpeg）**：对有效流抓取一帧实时画面生成缩略图，一眼就能看出是不是彩条卡或黑屏。
- **内置 HLS 反向代理**：解决浏览器端直接播放直播源的跨域（CORS）与防盗链限制；动态重写 M3U8 内切片的相对路径，并重算 Content-Length 避免播放器截断报错。
- **EPG 节目指南匹配**：支持 XMLTV（`.xml` 与 `.xml.gz`）节目单解析与展示，直接看“当前正在播什么”。
- **列表整理与多格式导出**：支持名称/分组检索、批量正则重命名、URL 快速去重，一键导出有效频道的 M3U、JSON 或 CSV。

---

## 核心设计与技术考量

> 面试或者做技术交流时，大家最关心的往往不是“有哪些界面”，而是“后台是怎么调度的”、“遇到网络流卡死怎么处理”。这里记录一下核心链路的设计细节。

```
[M3U / M3U8 输入] 
       │
       ▼
 [流式词法解析器] ── 提取 EXTINF 元数据 (tvg-id, group, name)
       │
       ▼
 [扫描调度引擎] ── 动态信号量限流 (控制并发 Goroutines)
       │
       ├─► 阶段 1: HTTP 快速连通性测试 (HEAD -> 降级轻量 GET)
       │
       └─► 阶段 2 (可选): FFprobe 流探测管道
                 ├─ 采样 packets 估算动态码率 & 区分 CBR/VBR
                 ├─ 提取视频规格 (H.264/HEVC/HDR/Dolby)
                 └─ FFmpeg 截取第一帧缩略图
       │
       ▼
 [结果汇总 & 事件流] ── SSE / HTTP API 实时推送到原生 Web 界面
       │
       ├─► 内置 HLS 反向代理 (CORS 绕过 / 防盗链 UA 注入 / M3U8 切片重写)
       └─► 导出清洗后的可用源列表 (M3U / CSV / JSON)
```

### 1. 并发调度与防爆破（Worker Pool & Dynamic Semaphore）
如果粗暴地为每个频道开一个 `go probe()`，几千个频道瞬间拉起会直接撑爆系统文件描述符（`too many open files`），且会导致部分源服务器将扫描机 IP 判定为 DDoS 攻击而直接拉黑。
- 采用 **带缓冲的通道实现动态信号量**，限制并发探活协程数（默认 20，可随机器性能动态调整）。
- 探测使用带有 `context.WithTimeout` 的统一 Client。
- 探活流程分为两阶段：优先用 HEAD 尝试降低网络流量，若对端返回 `405 Method Not Allowed` 则无缝降级为 GET 仅读取头部和第一个 chunk，兼顾速度与准确度。

### 2. 音视频子进程管理与防卡死（FFprobe Pipeline）
很多直播流没有 `Content-Length`，也不会主动关闭连接。如果直接调用 `ffprobe stream_url`，子进程可能会永远挂在网络 I/O 上，导致 Goroutine 和进程泄漏。
- 通过 `-analyzeduration 1000000 -probesize 1000000` 强行将探测窗口限制在首个 1 秒 / 1MB 数据以内。
- 统一使用 `exec.CommandContext(ctx, ...)` 封装外部命令，一旦 context 超时，底层强杀进程树，绝不残留僵尸子进程。
- **码率估算算法**：大部分直播流的 metadata 里根本没有 `bit_rate` 字段。这里单独拉取采样数据包累加 size 并除以采样时长，估算实时码率，并通过方差判定是恒定码率（CBR）还是可变码率（VBR）。

### 3. HLS 反向代理与流重写（Reverse Proxy & M3U8 Rewriter）
浏览器直接播放直播源常常报错：要么是远端源没有配置 `Access-Control-Allow-Origin: *`，要么是需要特定的 `User-Agent` 或 `Referer`。
- 本地启动轻量反向代理，对流请求统一伪装合法的 User-Agent 并剥除安全跨域限制。
- **切片路径重写**：针对 `.m3u8` 索引文件，用正则逐行替换内部的相对 URL，将其重写为 `/api/stream?url=...` 代理路径。
- **Payload 长度修正**：路径重写后文本体积改变，如果直接透传上游的 `Content-Length`，浏览器收到会被截断引发解码异常。代理层对修改后的 payload 重新计算长度并写回 header。

### 4. 全链路 SSRF 安全防御
因为程序提供了“输入任意 URL 由服务端拉取”的功能（`/api/load-url`、`/api/stream` 等），必须严格防范 SSRF：
- 严格校验协议仅允许 `http` 与 `https`。
- 通过 `net.ParseIP` 与 `net.LookupIP` 解析目标地址，过滤掉所有私网 IP、回环地址（`127.0.0.0/8`）与云服务元数据 IP（如 `169.254.169.254`），避免本地服务沦为内网渗透的跳板。

---

## 踩坑与调优实录

在写这个项目的过程中，踩过几个很有意思的坑：

1. **M3U 频道名里的半角逗号**：
   标准 M3U 的 `#EXTINF` 格式一般是 `#EXTINF:-1 tvg-id="xx",频道名`。但网上不少源是这样写的：`#EXTINF:-1,CCTV-1,综合频道`。最初用简单的 `strings.Split(line, ",")` 直接把频道名字切碎了。后来改成从行末反向定位逗号，结合引号闭合状态做状态机扫描，才把所有野鸡源的命名兼容下来。

2. **清空后返回 null 的诡异前端 Bug**：
   在单测覆盖 API 时抓出来的一个问题：当用户清空扫描结果时，Go 切片序列化出的 JSON 变成了 `{"results": null}` 而不是空数组 `{"results": []}`。原生前端 JS 里的 `data.results.forEach(...)` 当场白屏崩溃。后来在 handler 层统一保证零值切片返回前使用 `make([]ChannelResult, 0)` 初始化。

3. **User-Agent 防盗链轮询回退**：
   一些流媒体源对特定 UA 极其敏感（比如仅允许 `okhttp` 或 `TiviMate`，遇到标准浏览器 UA 直接 403）。扫描器里加了一套回退机制：当首选 UA 返回 401/403 时，自动选用内置的高频播放器 UA 字典进行二重重试，有效挽救了将近 20% 的“假死”频道。

---

## 快速上手

### 1. 直接运行编译好的二进制

到 [Releases](https://github.com/ying727/m3u-scanner/releases) 下载对应系统的压缩包，解压后双击运行即可，程序会自动在浏览器里打开控制台。

### 2. 自行从源码编译

要求 Go 1.21 或更高版本：

```bash
# 克隆仓库
git clone https://github.com/ying727/m3u-scanner.git
cd m3u-scanner

# 编译纯 Go 二进制
go build -o m3u-scanner .

# 启动（默认监听本机 127.0.0.1:8080）
./m3u-scanner

# 指定端口
./m3u-scanner -port 9090
```

启动后在浏览器打开 <http://localhost:8080> 即可。

### 3. 关于 FFmpeg / FFprobe（可选）

基础连通性检测、M3U 导入导出、内置播放器均**不需要**安装任何外部软件。

如果你需要**获取音视频编码规格、计算码率、抓取预览缩略图**，系统需安装 `ffmpeg` 与 `ffprobe`（扔进系统 `PATH` 或与本程序放在同一目录均可）：

```bash
# macOS
brew install ffmpeg

# Ubuntu / Debian
sudo apt install ffmpeg

# Windows (Scoop / Chocolatey)
scoop install ffmpeg
```

---

## 项目结构

```
.
├── cmd/ (main.go)           # 入口：参数解析、服务拉起、浏览器唤醒
├── internal/
│   ├── parser/              # M3U/M3U8 流式解析器与容错测试
│   ├── scanner/             # 扫描调度引擎、并发信号量、网络探测
│   ├── ffprobe/             # FFprobe/FFmpeg 进程封装、流信息计算、缩略图
│   ├── epg/                 # XMLTV / XML.GZ 节目单拉取与解析
│   └── ui/                  # Web 服务核心、HLS 代理、静态资源 embed、API 单元测试
│       └── static/          # 原生前端页面 (HTML / CSS / Vanilla JS)
├── bruno/                   # Bruno 接口测试集合（覆盖 15 个主要接口）
├── docs/                    # 测试计划 (TEST_PLAN.md) 等说明文档
├── example.m3u              # 随仓附带的测试播放列表
└── .github/workflows/       # CI 自动化流水线（格式化检查、单测、跨平台 release）
```

---

## 测试与质量保障

项目虽然规模不大，但核心链路均配有测试：

```bash
# 运行全部单测（含解析器、探针、接口用例）
go test -v ./...

# 跑 race 检测（Windows 需要 MinGW-w64，CI 中自动跑）
go test -race ./...
```

- **自动化接口测试**：`internal/ui/server_test.go` 中通过 `net/http/httptest` 对 18 个 API 场景（文件上传、SSRF 非法拦截、并发限流、参数截断、空状态防御等）做了全覆盖测试，无需启动真实端口即可瞬间跑完。
- **手动冒烟测试**：附带完整的 [Bruno](https://www.usebruno.com/) 接口用例（见 `bruno/` 目录），方便手动调试和端到端验证。详细测试策略见 [docs/TEST_PLAN.md](docs/TEST_PLAN.md)。

---

## 开源协议

[MIT License](LICENSE)
