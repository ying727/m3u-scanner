# 手动接口测试集合

用 [Bruno](https://www.usebruno.com/) 打开 `bruno/` 这个目录，
右上角环境选 `local`（默认 `http://localhost:8080`，端口不对自己改），
然后按文件名顺序从 01 点到 17 走一遍就行。

- `04-upload` 发之前先在 body 里选一个 `.m3u` 文件（比如仓库根目录的 `example.m3u`）。
- `15-epg` 的 `channel_id` 换成你列表里真实的台标 ID，不然返回 `available: false`。
- 06 开始扫描是真扫，台多的话会跑一阵；07 可以中途停掉。
