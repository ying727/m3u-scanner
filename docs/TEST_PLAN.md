# M3U Scanner 测试计划

> 这份文档是我自己写给自己看的：这个项目是我一个人维护的，没有测试团队，
> 所以策略很简单——能自动化的用 `go test` 跑起来，需要人眼看的（界面、真实扫描效果）
> 就手动测。面试的时候被问到"你怎么保证质量"，直接翻这份就行。

## 背景

M3U Scanner 是个 Go 写的桌面小工具：导入 M3U/M3U8 播放列表，批量检测频道能不能播，
顺带抓缩略图、看流信息、导出可用的频道。Web UI 跑在本地，后端是 Go 的 `net/http`，
前端是原生 JS。核心链路是：**导入列表 → 扫描 → 看结果 → 导出**，这条链路不能断。

之前只有 parser、scanner、ffprobe 的单测，web 接口（`/api/*` 二十来个）一个用例都没有，
全靠手点。这次补上。

## 测试范围

| 类型     | 内容 | 怎么测 |
|----------|------|--------|
| 接口测试 | `/api/*` 的正常/异常路径 | `go test ./internal/ui/` 自动化 |
| 功能测试 | 导入→扫描→导出整条链路 | 手动，Bruno 集合辅助（见 `bruno/`）|
| 异常测试 | 非法输入、超限参数、空状态 | 自动化为主 |
| 兼容测试 | Windows / macOS / Linux 编译产物能跑 | CI 里交叉编译 + 本地冒烟 |

不测的：真实直播源的扫描成功率（源本身天天变，测了也没意义，只保证"扫描流程"本身是对的）；
前端像素级样式（肉眼看）。

## 测试策略

1. **接口自动化优先**：后端逻辑都在 Go 里，用 `net/http/httptest` 直接调 `handler()`，
   不占端口、不弹浏览器，CI 里每次 push 都跑。断言只抓关键的：状态码 + 关键 JSON 字段，
   别把实现细节写死，不然一改代码就得改用例。
2. **扫描这类依赖网络的，不写自动化**：`ffprobe` 探流、真实 URL 检测，写了也是 flaky，
   留给手动。用例里只覆盖到"参数校验"这一层（比如内网 URL 直接 400 拦掉，不真发请求）。
3. **前端手动**：改完 UI 就本地起服务点一遍主流程，截图留档（`docs/screenshots/`）。

## 测试环境

- 本机：Go 1.21+，`go test ./...` 全过即算接口层 OK
- CI：GitHub Actions，push/PR 自动跑 `gofmt`、`go vet`、race test、跨平台编译
- 手动：Windows 11 / macOS / Ubuntu，Chrome/Edge 最新版
- 可选：装了 `ffmpeg`/`ffprobe` 的环境（测缩略图和流信息用，没装就跳过这部分）

## 用例表

下面是已经自动化的接口用例（`internal/ui/server_test.go`），手动回归时照着点也行。

| 用例 ID | 接口 | 前置条件 | 步骤 | 预期结果 |
|---------|------|----------|------|----------|
| API-01 | POST /api/upload | 无 | 上传合法 m3u 文件（含 2 个频道） | 200，`success=true`，`channels=2` |
| API-02 | POST /api/upload | 无 | multipart 里不带 file 字段 | 400，返回带 `error` |
| API-03 | GET /api/upload | 无 | 直接 GET | 405 |
| API-04 | POST /api/load-url | 无 | body 传非法 JSON | 400 |
| API-05 | POST /api/load-url | 无 | 传内网 IP（192.168.x.x / 127.0.0.1）、ftp 协议、空字符串 | 全部 400，且**不发起真实请求** |
| API-06 | POST /api/scan/start | 未加载播放列表 | 直接调 | 400，`error` 含"未加载播放列表" |
| API-07 | POST /api/scan/stop | 未在扫描 | 直接调 | 200，`success=true`（幂等） |
| API-08 | GET /api/settings | 无 | 直接调 | 200，`concurrency=20`，`timeout=15` |
| API-09 | POST /api/settings | 无 | 传正常值 | 200，原样存下 |
| API-10 | POST /api/settings | 无 | 传 `concurrency=500, timeout=500` | 200，被 clamp 到 100 / 120 |
| API-11 | POST /api/settings | 无 | 传非法 JSON | 400 |
| API-12 | GET /api/results | 空状态 | 直接调 | 200，`scanning=false`，带 `results` 字段 |
| API-13 | 上传→查结果 | 已上传 2 频道 | GET /api/results | 200，`results` 长度 2，第一个频道名正确 |
| API-14 | POST /api/results/clear | 已上传 | 清空后再查 | 200，`results` 为空数组（之前返回过 null，已修） |
| API-15 | POST /api/rename | 未加载播放列表 | 传 find/replace | 400 |
| API-16 | POST /api/deduplicate | 未加载播放列表 | 直接调 | 400 |
| API-17 | GET /api/status | 无 | 直接调 | 200，带 `ffprobe` 字段 |
| API-18 | GET /api/export | 空状态 | 直接调 | 200，`Content-Type: application/x-mpegurl`，body 以 `#EXTM3U` 开头 |

手动补充（Bruno 里都有）：

| 用例 ID | 内容 | 说明 |
|---------|------|------|
| MAN-01 | 真实 m3u8 列表走完"导入→扫描→导出" | 用几个公开测试源，确认流程不断 |
| MAN-02 | 扫描中点停止 | 进度停掉，不卡死 |
| MAN-03 | 改并发数/超时后扫描 | 设置实时生效 |
| MAN-04 | EPG 链接加载 | 有节目单的频道能显示"正在播" |

## 回归策略

- 每次改后端代码：`go test ./...` 必须全绿才 commit。
- 每次改前端：本地起服务，MAN-01 走一遍。
- 发版前：CI 全绿 + Windows/macOS 二进制各冒烟一次（能打开、能导入 example.m3u）。

## 风险

1. **网络 flaky**：扫描结果依赖上游源，CI 里不测真实扫描，避免误报。
2. **ffprobe 缺失**：没装 ffmpeg 的环境缩略图/流信息功能不可用，UI 要有降级提示（已做：状态接口返回 `ffprobe: false`）。
3. **SSRF**：`/api/load-url`、`/api/stream`、`/api/thumbnail` 会替用户 fetch URL，
   已加内网 IP 拦截（`isAllowedURL`），改这块代码时 API-05 必须过。
4. **一个人维护**：测试覆盖率就这样了，核心链路不断就行，别追求 100%。
