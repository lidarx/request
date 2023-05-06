module github.com/lidarx/request

go 1.20

require (
	github.com/refraction-networking/utls v1.3.2
	github.com/valyala/fasthttp v1.46.0
	golang.org/x/text v0.8.0
)

require (
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/gaukas/godicttls v0.0.3 // indirect
	github.com/klauspost/compress v1.16.3 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
)

replace github.com/valyala/fasthttp v1.46.0 => github.com/lidarx/fasthttp v0.0.0-20230506153217-37f0f97c170c

replace github.com/refraction-networking/utls v1.3.2 => github.com/lidarx/utls v0.0.0-20230506151644-ffe48ddeeb26
