package bilibili

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/Sophomoresty/mediago/internal/util"
)

const biliNavURL = "https://api.bilibili.com/x/web-interface/nav"

func ensureBilibiliLogin(client *util.Client, jar http.CookieJar) error {
	if !hasBilibiliLoginCookie(jar) {
		return fmt.Errorf("bilibili requires SESSDATA cookie")
	}
	return validateBilibiliLogin(client)
}

func hasBilibiliLoginCookie(jar http.CookieJar) bool {
	if jar == nil {
		return false
	}
	for _, host := range []string{"www.bilibili.com", "api.bilibili.com", "bilibili.com"} {
		for _, ck := range jar.Cookies(&url.URL{Scheme: "https", Host: host, Path: "/"}) {
			if ck.Value != "" && strings.EqualFold(ck.Name, "SESSDATA") {
				return true
			}
		}
	}
	return false
}

func validateBilibiliLogin(client *util.Client) error {
	body, err := client.GetString(biliNavURL, biliHeaders())
	if err != nil {
		return fmt.Errorf("bilibili nav login check: %w", err)
	}
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			IsLogin bool `json:"isLogin"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return fmt.Errorf("parse bilibili nav login check: %w", err)
	}
	if resp.Code == 0 && resp.Data.IsLogin {
		return nil
	}
	return fmt.Errorf("bilibili nav login check failed: code=%d message=%q isLogin=%v", resp.Code, resp.Message, resp.Data.IsLogin)
}

type biliStringID string

func (v *biliStringID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*v = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*v = biliStringID(strings.TrimSpace(s))
		return nil
	}
	*v = biliStringID(strings.TrimSpace(string(b)))
	return nil
}

func (v biliStringID) String() string { return string(v) }

func biliFirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func biliPickFormat(rawURL, hint string) string {
	if h := strings.Trim(strings.TrimSpace(hint), "."); h != "" {
		return strings.ToLower(h)
	}
	p := rawURL
	if u, err := url.Parse(rawURL); err == nil {
		p = u.Path
	}
	if ext := strings.TrimPrefix(strings.ToLower(path.Ext(p)), "."); ext != "" {
		return ext
	}
	return "bin"
}
