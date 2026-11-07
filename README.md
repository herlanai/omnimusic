# OmniMusic

用 Golet 原生 UI、Glass 材质和 Golet Audio 构建的 macOS / Windows 音乐播放器。每日精选、资料库和唱片室共用底部播放栏。首次安装默认浅色。点击右上角的设置齿轮，在窗口内的设置页选择浅色或深色外观。窗口保留系统控制按钮，拖动应用名称或顶栏留白可移动窗口。

窗口内的设置页中的「语言 / Language」菜单支持简体中文、English 和跟随系统。Golet 首次按系统首选语言匹配，未匹配时回退到简体中文。手动选择后即时生效，并由 Golet 保存到下次启动。词条位于 locales 下的两个语言目录文件，修改后可运行 `go run github.com/HyperparameterSI/golet/backend/cmd/golet generate` 更新访问函数。

## 试听与导入

首次启动按空格或播放键，即可离线试听 Brad Sucks 的《Bad Sign》。该 MP3 原样取自 [Out of It](https://archive.org/details/Out_Of_It-3406)，采用 [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/)。原有巴赫离线曲保留在钢琴曲库中。点击曲目会按当前列表顺序播放。在线 FLAC 首次播放时边下载边解码，收到足够的音频数据即可开播。完整文件通过长度和 SHA-1 校验后写入用户缓存目录 OmniMusic/audio，再次播放直接读取缓存。切歌会取消尚未完成的下载；跳到尚未下载的位置时会等待对应数据到达。

通过「导入音乐」添加本地 FLAC、PCM WAV、MP3、Ogg Vorbis 文件。与歌曲同目录、同名的 UTF-8 LRC 文件会随播放进度逐行淡入；支持一行多个时间戳。收藏、本地文件路径和主题保存在用户配置目录下的 OmniMusic/library.json。

顶部的「音色」可在「温润」与「原声」之间切换，并记住选择。温润模式在解码后轻调 150 Hz 附近的低频和 6.5 kHz 以上的高频，并为强峰值留出余量。切换使用短渐变，已有的音频缓冲播完后即可听到变化。

## 曲源与发现

现代流行曲库包含 54 首独立流行与电子流行作品，按实际文件标注 FLAC 或 MP3。各曲的作者、专辑来源和许可保存在 assets/pop.json，可在唱片室打开原专辑。每日精选从流行曲库中推荐 12 首；钢琴和其他纯音乐有独立的每日歌单和换批按钮。在线人声曲播放时按艺术家、曲名、专辑及时长从 [LRCLIB](https://lrclib.net/docs) 匹配歌词。点击列表中的歌曲即可播放并继续浏览。点击底部播放条的曲名、封面或「歌词」进入主窗口播放页。点击左上角收起按钮或按 Esc 返回。

古典曲库包含 145 首 [Musopen 公共领域 FLAC 录音](https://archive.org/details/MusopenCollectionAsFlac)。文件路径、时长、演奏者和校验值来自该藏品的 [原始元数据](https://archive.org/metadata/MusopenCollectionAsFlac)。该藏品标记为 [Public Domain Mark 1.0](https://creativecommons.org/publicdomain/mark/1.0/)。图中的唱片封套是应用自制插画。Golet Audio 将解码后的音频混合为 48 kHz 立体声输出。

「全部音乐」汇总在线曲库和已导入的本地音乐。YouTube Music 从 28 首中英文歌曲中每批推荐 12 首。相邻日期的首批歌单及连续两次换批使用互不重叠的候选集合；集合内按 HN 历史讨论对相应音乐类别的支持度和界面语言加权抽样。点击歌曲进入官方嵌入播放器，返回列表会停止视频。每日精选根据当前有效的界面语言优先排列带对应演唱语言标签的歌曲，随后按 HN 主题权重补齐。中文曲源包含 [Miss Melody China 的 EP](https://archive.org/details/miss-melody-china-ep)。每日精选每天生成 12 首；点击「换一批」会跳过上一批曲目，正在播放的队列继续播放。重启后回到当天的首批推荐。

推荐模型使用 OpenIndex 的 [Hacker News 数据集](https://huggingface.co/datasets/open-index/hacker-news)（ODC-By）。本版离线索引扫描 3 个月份中 73,906 篇存活的主题帖，在音乐主题的标题及评论树里匹配流行和器乐词。独立流行与电子流行各有 8 篇支持讨论，钢琴、室内乐、管弦乐分别匹配 21、5、11 篇，同一讨论可匹配多个类别。HN 的分数衡量帖子热度，音乐是否合听仍取决于个人偏好。

每篇讨论对相应编制的贡献为 `ln(1+命中发言数) × [1+ln(1+帖子分数)+0.5×ln(1+评论数)]`。按编制汇总贡献，再取 `1+ln(1+总贡献)` 作为主题权重。每批按权重抽取主推编制，匹配曲目的权重乘 2.5，并按该编制的曲库规模校正。以本地日期和批次为种子做加权无放回抽样。侧栏预览相关讨论中的评论。在「查看评论」中可离线阅读作者及原文，窄窗口可从每日精选标题行打开「HN 评论」。评论快照来自同一批 OpenIndex 数据分片，过滤了已删除或失效的评论。

**复现数据**。下载 [2012-05](https://huggingface.co/datasets/open-index/hacker-news/resolve/main/data/2012/2012-05.parquet)、[2016-10](https://huggingface.co/datasets/open-index/hacker-news/resolve/main/data/2016/2016-10.parquet) 和 [2024-01](https://huggingface.co/datasets/open-index/hacker-news/resolve/main/data/2024/2024-01.parquet) 分片，安装 Python duckdb 后运行：

```sh
python3 tools/build_signals.py 2012-05.parquet 2016-10.parquet 2024-01.parquet > assets/signals.json
python3 tools/build_comments.py 2012-05.parquet 2016-10.parquet 2024-01.parquet
```

## 开发与构建

需要 Go 1.27.3 或更高版本。项目按 Golet 官方本地源码模式使用同级 golet 目录。当前验证的框架提交为 c1a190e95c77f01ad2f42388eb9a1c74fdfe0ad5。新环境可先运行以下命令准备框架。
 
```sh
git clone https://github.com/HyperparameterSI/golet ../golet
git -C ../golet checkout c1a190e95c77f01ad2f42388eb9a1c74fdfe0ad5
```

```sh
go run .
go test ./...
go run github.com/HyperparameterSI/golet/backend/cmd/golet build -platform darwin/universal -skip-dmg
go run github.com/HyperparameterSI/golet/backend/cmd/golet build -platform windows/amd64
```
