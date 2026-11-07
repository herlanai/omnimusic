package main

import "omnimusic/locales"

func topicName(key string) string {
	switch key {
	case recentPage:
		return locales.RecentPlays()
	case youtube:
		return "YouTube Music"
	case "每日精选":
		return locales.Daily()
	case "全部音乐":
		return locales.AllMusic()
	case "我的收藏":
		return locales.Favorites()
	case "本地音乐":
		return locales.LocalMusic()
	case "钢琴":
		return locales.Piano()
	case "纯音乐":
		return locales.Instrumental()
	case "现代流行":
		return locales.Pop()
	case "独立流行":
		return locales.IndiePop()
	case "电子流行":
		return locales.ElectroPop()
	case "室内乐":
		return locales.Chamber()
	case "管弦乐":
		return locales.Orchestra()
	}
	return key
}
func lyricStatusName(key string) string {
	switch key {
	case "播放后载入歌词":
		return locales.LyricsOnPlay()
	case "正在查找歌词…":
		return locales.FindingLyrics()
	case "暂未收录这首歌的歌词":
		return locales.NoLyrics()
	case "歌词服务暂时无法连接":
		return locales.LyricsOffline()
	case "歌词服务暂时不可用":
		return locales.LyricsUnavailable()
	case "同步歌词 · LRCLIB":
		return locales.SyncedLyrics()
	case "歌词 · LRCLIB":
		return locales.PlainLyrics()
	case "纯音乐":
		return locales.Instrumental()
	}
	return key
}
func (t track) artistLabel() string {
	if t.Digest == "" && t.Artist == "本地文件" {
		return locales.LocalFile()
	}
	return t.Artist
}
