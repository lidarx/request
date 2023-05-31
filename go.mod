module github.com/lidarx/request

go 1.20

require (
	github.com/lidarx/tls v0.0.0-20230510162658-b002c600017d
	github.com/valyala/fasthttp v1.46.0
	golang.org/x/text v0.9.0
)

require (
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	golang.org/x/crypto v0.9.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
)

replace github.com/valyala/fasthttp v1.46.0 => github.com/lidarx/fasthttp v0.0.0-20230510163503-26c151bcdfef
