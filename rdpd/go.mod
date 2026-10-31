module qinjiardpd

go 1.21

require github.com/x90skysn3k/grdp v1.0.4

// 本地 fork：禁用位图缓存宣告（原库宣告了 5 级缓存但从不处理 CACHE_BITMAP
// 指令，服务端重绘引用缓存时画面出现大片黑区）；fork 同时保留 GPL-3.0 许可
replace github.com/x90skysn3k/grdp => ./grdp

require (
	github.com/lunixbochs/struc v0.0.0-20200707160740-784aaebc1d40 // indirect
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d // indirect
)
