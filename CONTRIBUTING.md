# Contributing

感谢你帮助改进 M3U Scanner。

## 开发环境

- Go 1.21 或更高版本
- FFprobe（完整扫描与媒体信息分析）
- FFmpeg（缩略图生成）

FFmpeg 和 FFprobe 应与程序可执行文件放在同一目录，或安装到系统 `PATH`。

## 本地检查

提交前请运行：

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
```

Windows 的 Go race detector 需要支持 CGO 的 C 编译器；没有该环境时，可以依赖 GitHub Actions 执行 race test。

## Pull Request

- 一个 PR 尽量只解决一个明确问题。
- 功能变化应同步更新 README。
- 修复解析、扫描或网络行为时，请补充相应测试。
- 不要提交 `settings.json`、扫描结果、FFmpeg 文件或编译产物。
