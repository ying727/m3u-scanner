# M3U Scanner

[![CI](https://github.com/ying727/m3u-scanner/actions/workflows/ci.yml/badge.svg)](https://github.com/ying727/m3u-scanner/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ying727/m3u-scanner?display_name=tag)](https://github.com/ying727/m3u-scanner/releases)

## 为啥做这个

网上找的 IPTV 播放列表，十个有九个播不了。一个几百上千频道的列表，
一个个点开试能试到天荒地老。于是写了个小工具：列表导进去，点一下扫描，
哪些能播、清晰度怎么样、有没有节目单，一目了然，扫完一键导出能播的。

Go 写的，打包出来是单个可执行文件，双击即用，不用配环境。

## 能干啥

- 本地文件 / URL 导入 M3U/M3U8 播放列表
- 并发检测频道能不能播，并发数、超时时间自己调
- 有 ffprobe 的话，还能看编码、分辨率、码率，顺手截一张缩略图
- 内置网页播放器直接播 HLS，也能甩给 PotPlayer / VLC / IINA 这些本地播放器
- 按名字、分组搜索过滤；重命名（支持正则）、去重
- 扫完导出"能播的"：m3u / json / csv 都行
- XMLTV 格式的 EPG 节目单，能看到"正在播什么"

## 快速开始

去 [Releases](https://github.com/ying727/m3u-scanner/releases) 下对应系统的包，
解压双击就行，会自动弹浏览器。

自己编译也行：

```bash
go build -o m3u-scanner .
./m3u-scanner            # 默认 8080 端口
./m3u-scanner -port 9000 # 换个端口
```

然后打开 <http://localhost:8080>。服务只监听本机（127.0.0.1），不会把端口暴露到局域网。

### 关于 ffmpeg（可选，不装也能用）

连通性检测、代理播放这些核心功能不依赖它。想看详细的流信息（编码/分辨率/码率）
或者要缩略图的话，装一个就行：`ffmpeg` 和 `ffprobe` 扔进系统 PATH，
或者跟程序放同一个目录。

```bash
brew install ffmpeg          # macOS
sudo apt install ffmpeg      # Debian/Ubuntu
sudo dnf install ffmpeg      # Fedora
```

## 截图

`docs/screenshots/` —— 待补，我找个频道多的列表截两张真实的。

## 玩法

1. 点"打开文件" / "打开 URL"，把播放列表导进来。
2. 点"开始扫描"。频道多的话可以开快速检测模式（只测连通，不探流信息）。
3. 点某个频道：看它能不能播、缩略图、流参数、节目单。
4. 点"播放"在网页里直接看，或者调用本地播放器。
5. 扫完点"导出"，拿到一份只剩能播频道的新 M3U。

设置（User-Agent、并发数、超时）改完会自动存，下次打开还在。

## FAQ

**有些台播不了？**
有些源要特定的 `Referer` 或 `User-Agent` 才给播，去设置里换个 UA 试试。
我默认拦掉了内网 IP（防 SSRF），所以别指望它能播你局域网的源。

**超时太多？**
并发数调小一点，超时调大一点。源服务器在国外的话，超时 15 秒可能不够。

**EPG 怎么配？**
设置里填 XMLTV 链接（`.xml` 或 `.gz` 结尾的都行），程序自动解析。
列表里自带 `x-tvg-url` 的话也会自动加载。

**缩略图出不来？**
先确认 `ffmpeg` 能在命令行里跑起来。有些流本身就不让截图，正常。

## 更新日志

- **v0.1.0**：EPG 节目单、安全加固（内网 URL 拦截、上传大小限制）、补单测
- **v0.0.1**：第一个能用的版本，导入/扫描/导出主流程跑通
- 之后：给 `/api/*` 补了接口测试（18 个用例，还顺手抓到个返回 null 的 bug），写了份测试计划，详见 [docs/TEST_PLAN.md](docs/TEST_PLAN.md)

## 许可证

MIT License，拿去随便用。出问题提 issue，我看到会回。
