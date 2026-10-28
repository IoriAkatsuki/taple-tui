package applemusic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"strings"
	"time"
)

const maxCookieFileBytes = 1 << 20
const maxCookieCount = 256

// CookieJar is validated input, not a browser-cookie export API.
type CookieJar struct {
	cookies []cookieParam
	expired int
}

func (j CookieJar) Count() int   { return len(j.cookies) }
func (j CookieJar) Expired() int { return j.expired }
func (j CookieJar) String() string {
	return fmt.Sprintf("CookieJar(%d accepted, %d expired)", j.Count(), j.Expired())
}
func (j CookieJar) GoString() string { return j.String() }

type cookieParam struct {
	Name     string   `json:"name"`
	Value    string   `json:"value"`
	URL      string   `json:"url,omitempty"`
	Domain   string   `json:"domain,omitempty"`
	Path     string   `json:"path"`
	Secure   bool     `json:"secure"`
	HTTPOnly bool     `json:"httpOnly"`
	SameSite string   `json:"sameSite,omitempty"`
	Expires  *float64 `json:"expires,omitempty"`
}

type exportedCookie struct {
	Name           string          `json:"name"`
	Value          string          `json:"value"`
	Domain         string          `json:"domain"`
	URL            string          `json:"url"`
	Path           string          `json:"path"`
	Secure         bool            `json:"secure"`
	HTTPOnly       bool            `json:"httpOnly"`
	HostOnly       *bool           `json:"hostOnly"`
	SameSite       string          `json:"sameSite"`
	Expires        *float64        `json:"expires"`
	ExpirationDate *float64        `json:"expirationDate"`
	Session        *bool           `json:"session"`
	PartitionKey   json.RawMessage `json:"partitionKey"`
	Partitioned    bool            `json:"partitioned"`
}

func ReadCookieFile(path string) (CookieJar, error) {
	file, err := openCookieFile(path)
	if err != nil {
		return CookieJar{}, errors.New("无法读取 Cookie 文件；请使用当前用户独占的普通文件，禁止链接（Unix 权限 0600，Windows 使用当前用户专用受保护 ACL）")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCookieFileBytes+1))
	if err != nil {
		return CookieJar{}, errors.New("读取 Cookie 文件失败")
	}
	defer clear(data)
	return parseCookies(data, time.Now())
}

func parseCookies(data []byte, now time.Time) (CookieJar, error) {
	if len(data) > maxCookieFileBytes {
		return CookieJar{}, errors.New("Cookie 文件超过 1 MiB 限制")
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	trimmed := bytes.TrimSpace(data)
	var entries []exportedCookie
	if len(trimmed) == 0 {
		return CookieJar{}, errors.New("Cookie 文件为空")
	}
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return CookieJar{}, errors.New("Cookie JSON 格式无效，需要浏览器导出的 Cookie 数组")
		}
	} else {
		var err error
		entries, err = parseNetscapeCookies(data)
		if err != nil {
			return CookieJar{}, err
		}
	}
	if len(entries) == 0 || len(entries) > maxCookieCount {
		return CookieJar{}, errors.New("Cookie 文件必须包含 1 到 256 个条目")
	}
	jar := CookieJar{}
	seen := make(map[string]bool)
	for i, entry := range entries {
		c, key, expired, err := normalizeCookie(entry, now)
		if err != nil {
			return CookieJar{}, fmt.Errorf("Cookie 第 %d 项无效：%s", i+1, err)
		}
		if seen[key] {
			return CookieJar{}, fmt.Errorf("Cookie 第 %d 项与已有条目的名称、域和路径重复", i+1)
		}
		seen[key] = true
		if expired {
			jar.expired++
			continue
		}
		jar.cookies = append(jar.cookies, c)
	}
	if jar.Count() == 0 {
		return CookieJar{}, errors.New("Cookie 文件没有尚未过期的条目")
	}
	return jar, nil
}

func parseNetscapeCookies(data []byte) ([]exportedCookie, error) {
	var entries []exportedCookie
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		httpOnly := strings.HasPrefix(line, "#HttpOnly_")
		if strings.HasPrefix(line, "#") && !httpOnly {
			continue
		}
		line = strings.TrimPrefix(line, "#HttpOnly_")
		fields := strings.Split(line, "\t")
		if len(fields) != 7 || (fields[1] != "TRUE" && fields[1] != "FALSE") || (fields[3] != "TRUE" && fields[3] != "FALSE") {
			return nil, errors.New("Netscape Cookie Jar 格式无效，需要七个制表符分隔字段")
		}
		var expiry float64
		if err := json.Unmarshal([]byte(fields[4]), &expiry); err != nil || expiry < 0 {
			return nil, errors.New("Cookie 到期时间必须是 Unix 秒数")
		}
		hostOnly := fields[1] == "FALSE"
		if expiry == 0 {
			expiry = -1
		}
		entries = append(entries, exportedCookie{Domain: fields[0], HostOnly: &hostOnly, Path: fields[2], Secure: fields[3] == "TRUE", HTTPOnly: httpOnly, Expires: &expiry, Name: fields[5], Value: fields[6]})
		if len(entries) > maxCookieCount {
			return nil, errors.New("Cookie 条目超过 256 个限制")
		}
	}
	if scanner.Err() != nil {
		return nil, errors.New("Cookie 条目过长或格式无效")
	}
	return entries, nil
}

func normalizeCookie(entry exportedCookie, now time.Time) (cookieParam, string, bool, error) {
	bad := func(reason string) (cookieParam, string, bool, error) {
		return cookieParam{}, "", false, errors.New(reason)
	}
	hostOnly := !strings.HasPrefix(entry.Domain, ".")
	if entry.HostOnly != nil {
		hostOnly = *entry.HostOnly
	}
	domain := strings.ToLower(strings.TrimPrefix(entry.Domain, "."))
	if domain != "music.apple.com" && !(domain == "apple.com" && !hostOnly) {
		return bad("仅允许 music.apple.com 及其继承的 .apple.com Cookie")
	}
	if hostOnly && strings.HasPrefix(entry.Domain, ".") {
		return bad("hostOnly 与域范围冲突")
	}
	if entry.Partitioned || (len(entry.PartitionKey) > 0 && string(entry.PartitionKey) != "null") {
		return bad("暂不支持分区 Cookie")
	}
	if entry.Name == "" || len(entry.Name)+len(entry.Value) > 4096 || strings.ContainsAny(entry.Name, "()<>@,;:\\\"/[]?={} \t") {
		return bad("名称或条目大小无效")
	}
	for _, c := range []byte(entry.Name) {
		if c < 0x21 || c > 0x7e {
			return bad("名称包含不支持的字符")
		}
	}
	for _, c := range []byte(entry.Value) {
		if c < 0x21 || c > 0x7e || c == '"' || c == ',' || c == ';' || c == '\\' {
			return bad("值包含不支持的字符，请使用原始浏览器导出格式")
		}
	}
	if entry.Path == "" {
		entry.Path = "/"
	}
	if !strings.HasPrefix(entry.Path, "/") || len(entry.Path) > 2048 || strings.ContainsAny(entry.Path, "\r\n\x00;") {
		return bad("路径无效")
	}
	for _, ch := range []byte(entry.Path) {
		if ch < 0x20 || ch == 0x7f {
			return bad("路径包含控制字符")
		}
	}
	if entry.URL != "" {
		u, err := url.Parse(entry.URL)
		if err != nil || u.Scheme != "https" || u.Host != domain || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return bad("URL 与 Cookie 域不一致")
		}
	}
	c := cookieParam{Name: entry.Name, Value: entry.Value, Path: entry.Path, Secure: entry.Secure, HTTPOnly: entry.HTTPOnly}
	if hostOnly {
		c.URL = (&url.URL{Scheme: "https", Host: domain, Path: entry.Path}).String()
	} else {
		c.Domain = "." + domain
	}
	switch strings.ToLower(entry.SameSite) {
	case "", "unspecified":
	case "strict":
		c.SameSite = "Strict"
	case "lax":
		c.SameSite = "Lax"
	case "none", "no_restriction":
		c.SameSite = "None"
		if !c.Secure {
			return bad("SameSite=None 必须使用 Secure")
		}
	default:
		return bad("SameSite 属性无效")
	}
	if strings.HasPrefix(c.Name, "__Secure-") && !c.Secure {
		return bad("__Secure- Cookie 必须使用 Secure")
	}
	if strings.HasPrefix(c.Name, "__Host-") && (!c.Secure || !hostOnly || c.Path != "/") {
		return bad("__Host- Cookie 的域、路径或 Secure 属性无效")
	}
	expires := entry.Expires
	if entry.ExpirationDate != nil {
		if expires != nil && *expires != *entry.ExpirationDate {
			return bad("存在冲突的到期时间")
		}
		expires = entry.ExpirationDate
	}
	if expires != nil && (math.IsNaN(*expires) || math.IsInf(*expires, 0) || *expires < -1 || *expires > 253402300799) {
		return bad("到期时间必须是有效的 Unix 秒数")
	}
	if entry.Session != nil && *entry.Session && expires != nil && *expires > 0 {
		return bad("会话属性与到期时间冲突")
	}
	if expires != nil && *expires >= 0 && (entry.Session == nil || !*entry.Session) {
		c.Expires = expires
	}
	key := domain + "\x00" + c.Path + "\x00" + c.Name
	return c, key, c.Expires != nil && *c.Expires <= float64(now.Unix()), nil
}
