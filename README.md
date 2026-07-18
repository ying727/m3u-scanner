# M3U Scanner

[![CI](https://github.com/ying727/m3u-scanner/actions/workflows/ci.yml/badge.svg)](https://github.com/ying727/m3u-scanner/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ying727/m3u-scanner?display_name=tag)](https://github.com/ying727/m3u-scanner/releases)

M3U/M3U8 播放列表扫描与管理工具，提供现代化 Web 界面。

## 功能

- 从本地文件或远程 URL 导入 M3U/M3U8 播放列表
- 并发检测频道可用性，支持可调节并发数与超时时间
- 使用 FFprobe 分析视频编码、码率、分辨率和音频流信息
- 为可用频道生成预览缩略图
- 内置 Web 播放器、HLS 代理和第三方播放器调用
- 支持按名称、分组搜索和过滤，并可导出有效频道
- 支持 XMLTV 格式 EPG 节目单
- User-Agent、超时和扫描并发设置持久化保存

## 快速开始

可以从 [Releases](https://github.com/ying727/m3u-scanner/releases) 下载对应平台的程序，或自行编译。

### 编译

项目使用 Go 编写，编译时不强制依赖 C 编译器：

```bash
go build -o m3u-scanner.exe .
```

### 启动

```bash
# 默认使用 8080 端口
./m3u-scanner.exe

# 指定端口
./m3u-scanner.exe -port 9000
```

启动后打开 <http://localhost:8080>。服务默认仅监听本机地址。

## FFmpeg/FFprobe（可选）

基础连通性检测和代理功能不强制依赖外部工具。若需要获取详细视频流信息或生成缩略图，请安装 FFmpeg（包含 `ffmpeg` 与 `ffprobe`），并确保命令位于系统 `PATH` 中，或与程序放在同一目录。

```bash
# macOS
brew install ffmpeg

# Debian/Ubuntu
sudo apt install ffmpeg

# Fedora
sudo dnf install ffmpeg
```

## 使用指南

1. 点击工具栏中的“打开文件”或“打开 URL”导入播放列表。
2. 点击“开始扫描”检测频道；频道较多时可启用快速检测模式。
3. 点击频道查看连通状态、缩略图、流参数和 EPG 信息。
4. 点击“播放”在浏览器内预览，或调用 PotPlayer、VLC、IINA 等本地播放器。
5. 扫描完成后点击“导出”，生成仅包含有效频道的新 M3U 文件。

## 常见问题

- 某些频道需要特定的 `Referer` 或 `User-Agent`，可在“设置”中调整 User-Agent。
- 扫描超时较多时，可降低并发数并适当增加超时时间。
- EPG 设置中填入 XMLTV 链接（支持 `.xml` 和 `.gz`），程序会自动解析。
- 缩略图无法生成时，请确认 `ffmpeg` 命令可执行。

## 许可证

MIT License

## 开发与发布

Pull Request 和推送到 `main` 时，GitHub Actions 会自动执行格式检查、静态检查、race test 和跨平台编译。

推送形如 `v1.0.0` 的 tag 后，会自动构建 Windows、Linux 和 macOS 的 amd64/arm64 程序，生成 SHA-256 校验文件并发布 GitHub Release。贡献说明见 [CONTRIBUTING.md](CONTRIBUTING.md)。
