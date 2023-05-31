package request

import (
	"bytes"
	"github.com/valyala/fasthttp"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"html"
	"io"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

var responsePool sync.Pool
var titleReg = regexp.MustCompile("(?ims)<title.*?>(.*?)</title>")
var emptyReg = regexp.MustCompile(`[\n\r\t]+`)

func AcquireResponse() *Response {
	v := responsePool.Get()
	if v == nil {
		return &Response{
			Response: fasthttp.AcquireResponse(),
		}
	}
	r := v.(*Response)
	r.Response = fasthttp.AcquireResponse()
	return r
}

func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}

type Response struct {
	*fasthttp.Response
	body  string
	title string
}

func (r *Response) Reset() {
	fasthttp.ReleaseResponse(r.Response)
	r.Response = nil
	r.title = ""
	r.body = ""
}

func (r *Response) GetHeader(k string) (string, bool) {
	vb := r.Response.Header.Peek(k)
	if vb == nil {
		return "", false
	} else {
		return string(vb), true
	}
}

func (r *Response) Text() string {
	if r.body != "" {
		return r.body
	}
	body, err := r.Response.BodyUncompressed()
	if err != nil {
		body = r.Response.Body()
	}
	r.body = string(decodeBody(body))
	return r.body
}

func (r *Response) Title() string {
	if r.title != "" {
		return r.title
	}
	find := titleReg.FindStringSubmatch(r.Text())
	if len(find) > 1 {
		r.title = find[1]
		r.title = emptyReg.ReplaceAllString(html.UnescapeString(r.title), "")
		r.title = strings.TrimSpace(r.title)
	}
	return r.title
}

func (r *Response) BodyContains(s string) bool {
	return strings.Contains(r.Text(), s)
}

func (r *Response) HeaderContains(s string) bool {
	return bytes.Contains(r.Response.Header.Header(), []byte(s))
}

func (r *Response) Cookie(k string) (string, bool) {
	v := r.Response.Header.PeekCookie(k)
	if v == nil {
		return "", false
	}
	c := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(c)
	err := c.ParseBytes(v)
	if err != nil {
		return "", false
	}
	return string(c.Value()), true
}

func (r *Response) String() string {
	return r.Response.String()
}

func (r *Response) Search(reg *regexp.Regexp) map[string]string {
	match := reg.FindStringSubmatch(r.Text())
	groupNames := reg.SubexpNames()
	result := make(map[string]string)
	if len(match) < len(groupNames) {
		return result
	}
	for i, name := range groupNames {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}
	return result
}

func decodeBody(s []byte) []byte {
	I := bytes.NewReader(s)
	var O io.Reader
	if !utf8.Valid(s) {
		O = transform.NewReader(I, simplifiedchinese.GB18030.NewDecoder())
		d, e := io.ReadAll(O)
		if e != nil {
			return s
		} else {
			return d
		}
	} else {
		return s
	}
}
