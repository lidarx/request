package request

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"time"
)

var requestPool sync.Pool

type Params map[string]string
type Data map[string]string // for post form
type Header map[string]string
type Files map[string]File // name ,file-content
type File struct {
	FileName    string
	ContentType string
	Content     []byte
}

var defaultClient = &fasthttp.Client{
	TLSConfig:                 &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionSSL30},
	MaxIdleConnDuration:       1 * time.Second, //减少等待时间
	ReadTimeout:               5 * time.Second,
	WriteTimeout:              5 * time.Second,
	MaxResponseBodySize:       10 * 1024 * 1024,
	MaxIdemponentCallAttempts: 1,
	RetryIf: func(request *fasthttp.Request) bool {
		return false
	},
	DisablePathNormalizing:        true,
	DisableHeaderNamesNormalizing: true,
}

// AcquireRequest returns an empty Request instance from request pool.
//
// The returned Request instance may be passed to ReleaseRequest when it is
// no longer needed. This allows Request recycling, reduces GC pressure
// and usually improves performance.
func AcquireRequest() *Request {
	v := requestPool.Get()
	jar, _ := cookiejar.New(nil)
	if v == nil {
		return &Request{
			Request: fasthttp.AcquireRequest(),
			Jar:     jar,
			client: &fasthttp.Client{
				TLSConfig:                     &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionSSL30},
				MaxIdleConnDuration:           defaultClient.MaxIdleConnDuration,
				ReadTimeout:                   defaultClient.ReadTimeout,
				WriteTimeout:                  defaultClient.WriteTimeout,
				MaxResponseBodySize:           defaultClient.MaxResponseBodySize,
				MaxIdemponentCallAttempts:     defaultClient.MaxIdemponentCallAttempts,
				RetryIf:                       defaultClient.RetryIf,
				Dial:                          defaultClient.Dial,
				DisablePathNormalizing:        true,
				DisableHeaderNamesNormalizing: true,
			},
		}
	}
	r := v.(*Request)
	r.Request = fasthttp.AcquireRequest()
	r.Jar = jar
	return r
}

func AcquireRequestResponse() (*Request, *Response) {
	return AcquireRequest(), AcquireResponse()
}

func ReplaceGlobalClient(c *fasthttp.Client) {
	defaultClient = c
}

func SetSocks5Proxy(proxy string) {
	defaultClient.Dial = fasthttpproxy.FasthttpSocksDialer(proxy)
	return
}

func SetHTTPProxy(proxy string) {
	if strings.HasPrefix(proxy, "http://") {
		proxy = strings.TrimPrefix(proxy, "http://")
	}
	defaultClient.Dial = fasthttpproxy.FasthttpHTTPDialer(proxy)
	return
}

func ReleaseRequestResponse(req *Request, resp *Response) {
	ReleaseRequest(req)
	ReleaseResponse(resp)
}

// ReleaseRequest returns req acquired via AcquireRequest to request pool.
//
// It is forbidden accessing req and/or its' members after returning
// it to request pool.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}

type TraceInfo struct {
	Request  string
	Response string
	Duration time.Duration
}

type Request struct {
	*fasthttp.Request
	Trace        *[]TraceInfo
	maxRedirects int
	maxRetry     int
	retryClient  *fasthttp.Client
	Jar          *cookiejar.Jar
	client       *fasthttp.Client
}

func (r *Request) Reset() {
	r.Trace = nil
	r.maxRedirects = 0
	r.maxRetry = 0
	r.Jar = nil
	fasthttp.ReleaseRequest(r.Request)
	r.Request = nil
	r.retryClient = nil
}

func (r *Request) SetSocks5Proxy(proxy string) *Request {
	r.client.Dial = fasthttpproxy.FasthttpSocksDialer(proxy)
	return r
}

func (r *Request) SetHTTPProxy(proxy string) *Request {
	if strings.HasPrefix(proxy, "http://") {
		proxy = strings.TrimPrefix(proxy, "http://")
	}
	r.client.Dial = fasthttpproxy.FasthttpHTTPDialer(proxy)
	return r
}

func (r *Request) SetMaxRedirects(t int) *Request {
	r.maxRedirects = t
	return r
}

func (r *Request) SetRetry(t int) *Request {
	r.maxRetry = t
	r.retryClient = &fasthttp.Client{
		TLSConfig:           r.client.TLSConfig,
		MaxIdleConnDuration: r.client.MaxIdleConnDuration,
		ReadTimeout:         r.client.ReadTimeout,
		WriteTimeout:        r.client.WriteTimeout,
		MaxResponseBodySize: r.client.MaxResponseBodySize,
		RetryIf: func(request *fasthttp.Request) bool {
			return false
		},
	}
	return r
}

func (r *Request) SetRetrySocks5Proxy(proxy string) *Request {
	r.retryClient.Dial = fasthttpproxy.FasthttpSocksDialer(proxy)
	return r
}

func (r *Request) SetRetryHTTPProxy(proxy string) *Request {
	if strings.HasPrefix(proxy, "http://") {
		proxy = strings.TrimPrefix(proxy, "http://")
	}
	r.retryClient.Dial = fasthttpproxy.FasthttpHTTPDialer(proxy)
	return r
}

func (r *Request) String() string {
	return r.Request.String()
}

func (r *Request) Method(method string) *Request {
	r.Request.Header.SetMethod(method)
	return r
}

func (r *Request) URI(u string) *Request {
	r.Request.SetRequestURI(u)
	return r
}

func (r *Request) UserAgent(ua string) *Request {
	r.Request.Header.SetUserAgent(ua)
	return r
}

func (r *Request) ContentType(c string) *Request {
	r.Request.Header.SetContentType(c)
	return r
}

func (r *Request) SetParams(p Params) *Request {
	r.Request.URI().QueryArgs().Reset()
	for k, v := range p {
		r.Request.URI().QueryArgs().Set(k, v)
	}
	return r
}

func (r *Request) SetTimeout(t time.Duration) *Request {
	r.Request.SetTimeout(t)
	r.client.ReadTimeout = t
	r.client.WriteTimeout = t
	return r
}

func (r *Request) SetData(p Data) *Request {
	r.ContentType("application/x-www-form-urlencoded")
	r.ResetBody()
	r.PostArgs().Reset()
	for k, v := range p {
		r.Request.PostArgs().Set(k, v)
	}
	return r
}

func (r *Request) DisableNormalizing() *Request {
	r.Request.Header.DisableNormalizing()
	r.Request.URI().DisablePathNormalizing = true
	r.client.DisablePathNormalizing = true
	r.client.DisableHeaderNamesNormalizing = true
	return r
}

func (r *Request) BodyRaw(s string) *Request {
	r.Request.SetBodyRaw([]byte(s))
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		r.ContentType(ContentTypeJson)
	} else if strings.HasPrefix(s, "<") {
		r.ContentType(ContentTypeXml)
	} else if strings.Contains(s, "=") || strings.Contains(s, "%") {
		r.ContentType(ContentTypeForm)
	} else if len(s) > 0 {
		r.ContentType(ContentTypeOctetStream)
	}
	return r
}

func (r *Request) FromRaw(s string) error {
	return r.Request.Read(bufio.NewReader(strings.NewReader(s)))
}

func (r *Request) Host(host string) *Request {
	if host != "" {
		r.Request.UseHostHeader = true
		r.Request.Header.SetHostBytes([]byte(host))
		if bytes.Equal(r.Request.URI().Scheme(), []byte("https")) {
			// servername 不能包含端口
			servername := strings.Split(host, ":")[0]
			r.client.TLSConfig.ServerName = servername
		}
	}
	return r
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (r *Request) BasicAuth(u, p string) *Request {
	r.Header.Set("Authorization", "Basic "+basicAuth(u, p))
	return r
}

func (r *Request) ClearTrace() *Request {
	*r.Trace = []TraceInfo{}
	return r
}

func (r *Request) SetHeader(h Header) *Request {
	for k, v := range h {
		r.Header.Set(k, v)
	}
	return r
}

func (r *Request) WithTrace(t *[]TraceInfo) *Request {
	r.Trace = t
	return r
}

func (r *Request) Client(c *fasthttp.Client) *Request {
	if c != nil {
		r.client = c
	}
	return r
}

func (r *Request) MultipartFiles(fs Files) *Request {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	for n, f := range fs {
		h := make(textproto.MIMEHeader)
		if f.ContentType != "" {
			h.Set("Content-Type", f.ContentType)
		}
		if f.FileName != "" {
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
					escapeQuotes(n), escapeQuotes(f.FileName)))
		} else {
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"`, escapeQuotes(n)))
		}
		part, err := w.CreatePart(h)
		if err != nil {
			fmt.Printf("Upload %s failed!", n)
			panic(err)
		}
		if len(f.Content) > 0 {
			reader := bytes.NewReader(f.Content)
			_, _ = io.Copy(part, reader)
		}
	}
	w.Close()

	r.Request.SetBodyRaw(b.Bytes())
	r.Request.Header.SetMultipartFormBoundary(w.Boundary())
	return r
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}
func (r *Request) Do(resp *Response) error {
	if len(r.Header.UserAgent()) == 0 {
		// default useragent
		r.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.6312.122 Safari/537.36")
	}
	resp.body = ""
	resp.title = ""
	u, err := url.Parse(string(r.Request.Header.RequestURI()))
	if err == nil {
		if r.Jar.Cookies(u) != nil {
			//r.Header.DelAllCookies() // 此处不应该清除cookie
			cookies := r.Jar.Cookies(u)
			for _, c := range cookies {
				r.Header.SetCookie(c.Name, c.Value)
			}
		}
		defer func() {
			if resp.Header.Peek("Set-Cookie") != nil {
				httpResp := http.Response{Header: map[string][]string{}}
				resp.Header.VisitAllCookie(func(key, value []byte) {
					httpResp.Header.Add("Set-Cookie", string(value))
				})
				r.Jar.SetCookies(u, httpResp.Cookies())
			}
		}()
	}

	start := time.Now()
	defer func() {
		if r.Trace != nil {
			*r.Trace = append(*r.Trace, TraceInfo{
				Request:  r.String(),
				Response: resp.String(),
				Duration: time.Since(start),
			})
		}
	}()
	client := r.client
	for i := 0; i <= r.maxRetry; i++ {
		if i != 0 {
			client = r.retryClient
		}
		if r.maxRedirects > 1 {
			err = client.DoRedirects(r.Request, resp.Response, r.maxRedirects)
		} else {
			err = client.Do(r.Request, resp.Response)
		}
		// 忽略user canceled错误
		if err == nil || strings.Contains(err.Error(), "user canceled") {
			return nil
		} else if !errors.Is(err, fasthttp.ErrTimeout) && !errors.Is(err, fasthttp.ErrConnectionClosed) {
			return err
		}
	}
	return err
}

func (r *Request) ResetBody() *Request {
	r.Request.ResetBody()
	return r
}

func (r *Request) ResetParam() *Request {
	r.Request.URI().Reset()
	return r
}

func (r *Request) ResetHeader() *Request {
	r.Request.Header.Reset()
	return r
}

func (r *Request) prepare(u string, args ...interface{}) *Request {
	r.ResetBody()
	r.URI(u)
	for _, arg := range args {
		switch arg.(type) {
		case string:
			r.BodyRaw(arg.(string))
		case []byte:
			r.BodyRaw(string(arg.([]byte)))
		case Files:
			r.MultipartFiles(arg.(Files))
		case Header:
			r.SetHeader(arg.(Header))
		case Params:
			r.SetParams(arg.(Params))
		case Data:
			r.SetData(arg.(Data))
		}
	}
	return r
}

func (r *Request) Get(u string, args ...interface{}) *Request {
	return r.Method(MethodGet).prepare(u, args...)
}

func (r *Request) Post(u string, args ...interface{}) *Request {
	return r.Method(MethodPost).prepare(u, args...)
}

func (r *Request) Move(u string, args ...interface{}) *Request {
	return r.Method(MethodMove).prepare(u, args...)
}

func (r *Request) Put(u string, args ...interface{}) *Request {
	return r.Method(MethodPut).prepare(u, args...)
}

func (r *Request) Delete(u string, args ...interface{}) *Request {
	return r.Method(MethodDelete).prepare(u, args...)
}

func (r *Request) Head(u string, args ...interface{}) *Request {
	return r.Method(MethodHead).prepare(u, args...)
}

func (r *Request) Options(u string, args ...interface{}) *Request {
	return r.Method(MethodOptions).prepare(u, args...)
}

func (r *Request) Patch(u string, args ...interface{}) *Request {
	return r.Method(MethodPatch).prepare(u, args...)
}
