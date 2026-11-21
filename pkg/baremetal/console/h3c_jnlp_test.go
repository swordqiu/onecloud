// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package console

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

const h3cFixtureJS = `
function linebrk(e,t){for(var n="",i=0;i+t<e.length;)n+=e.substring(i,i+t)+"\n",i+=t;return n+e.substring(i,e.length)}
login:function(){var n={username:e("#login-username").val(),password:e("#login-password").val(),log_type:1};n.username=linebrk(hex2b64(t.encrypt(e("#login-username").val())));n.password=linebrk(hex2b64(t.encrypt(e("#login-password").val())));}
fetchPort:function(e,t){e.ajax({url:"/api/settings/media/instance",type:"GET"})}
jnlpTemplate:function(t,n,i){var a="",l=r.langtype,s=t||2,o=i.kvmsecure?i.kvmsecure:0,c=i.vmsecure?i.vmsecure:0,d=i.kvmsecure?i.kvm_secure_port:4294967295===i.kvm_port?7578:2147483647&i.kvm_port,u=i.vmsecure?i.cd_secure_port:4294967295===i.cd_port?5120:2147483647&i.cd_port,p=i.vmsecure?i.fd_secure_port:4294967295===i.fd_port?5122:2147483647&i.fd_port,h=i.vmsecure?i.hd_secure_port:4294967295===i.hd_port?5123:2147483647&i.hd_port,f=i.retry_count,g=i.retry_interval,m=i.kvm_status,v=parseInt(e.cookie("extended_privilege")),b=parseInt(e.cookie("user_extendedpriv")),_="uis"==e.cookie("product_board_id")?1:0,y=parseInt(e.cookie("productID_num")),w=parseInt(e.cookie("custom_id"));return a='<?xml version="1.0" encoding="UTF-8"?><jnlp spec="1.0+" codebase="'+r.urlPath()+'/remote_console"><information><title>KVM</title></information><application-desc><argument>-kvmmode</argument><argument>'+s+'</argument><argument>-kvmtoken</argument><argument>'+n.token+'</argument><argument>-kvmport</argument><argument>'+d+'</argument><argument>-priv</argument><argument>'+v+'</argument></application-desc></jnlp>';}
`

func TestHex2B64(t *testing.T) {
	if got := hex2b64("abc"); got != "q8==" {
		t.Fatalf("hex2b64(abc)=%s", got)
	}
	if got := hex2b64("0"); got != "A===" {
		t.Fatalf("hex2b64(0)=%s", got)
	}
}

func TestH3CTemplatePorts(t *testing.T) {
	prepared, err := prepareH3CJNLP(h3cFixtureJS, "https://10.22.41.30")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.lineWidth != 0 {
		t.Fatalf("line width %d", prepared.lineWidth)
	}
	if prepared.mediaPath != "/api/settings/media/instance" {
		t.Fatalf("media path %s", prepared.mediaPath)
	}
	cases := []struct {
		media string
		port  string
	}{
		{`{"kvm_port":4294967295}`, "7578"},
		{`{"kvm_port":7590}`, "7590"},
		{`{"kvmsecure":1,"kvm_secure_port":5900,"kvm_port":7590}`, "5900"},
	}
	for _, tc := range cases {
		out, err := prepared.render([]byte(`{"token":"abcdefgh"}`), []byte(tc.media), map[string]string{"extended_privilege": "7"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "<argument>"+tc.port+"</argument>") {
			t.Fatalf("port %s missing in %s", tc.port, out)
		}
		if !strings.Contains(out, `codebase="https://10.22.41.30/remote_console"`) {
			t.Fatalf("codebase missing in %s", out)
		}
		if !strings.Contains(out, "<argument>7</argument>") {
			t.Fatalf("privilege missing in %s", out)
		}
	}
}

func TestH3CLoginHeldDoesNotPost(t *testing.T) {
	h3cHoldLogin = true
	defer func() { h3cHoldLogin = false }()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			io.WriteString(w, `<html><script src="/index.min.js"></script></html>`)
		case "/index.min.js":
			io.WriteString(w, h3cFixtureJS)
		default:
			hits.Add(1)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_, err := runH3CJNLP(context.Background(), &ConsoleJNLPInput{Host: u.Host, Username: "admin", Password: "s3cret-pass"})
	if err == nil || !strings.Contains(err.Error(), "not submitted") {
		t.Fatalf("err %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("requests after the script: %d", hits.Load())
	}
}

func TestH3CJNLPPostsEncryptedPasswordOnce(t *testing.T) {
	h3cHoldLogin = false
	defer func() { h3cHoldLogin = true }()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	var posts atomic.Int32
	var keys atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<html><script src="/index.min.js?v=3.48"></script></html>`)
		case strings.HasPrefix(r.URL.Path, "/index.min.js"):
			w.Header().Set("Content-Type", "application/javascript")
			io.WriteString(w, h3cFixtureJS)
		case r.URL.Path == "/api/rsapublickey":
			keys.Add(1)
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"RSA_Value":1,"exponent":"`+hex.EncodeToString(big.NewInt(int64(key.E)).Bytes())+`","modulus":"`+key.N.Text(16)+`","cc":0}`)
		case r.URL.Path == "/api/session" && r.Method == http.MethodPost:
			posts.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse form: %v", err)
				http.Error(w, "bad", 400)
				return
			}
			if r.Form.Get("log_type") != "1" {
				t.Errorf("log_type %s", r.Form.Get("log_type"))
			}
			user, err := h3cTestDecrypt(key, r.Form.Get("username"))
			if err != nil || user != "admin" {
				t.Errorf("username decrypt %q %v", user, err)
			}
			pass, err := h3cTestDecrypt(key, r.Form.Get("password"))
			if err != nil || pass != "s3cret-pass" {
				t.Errorf("password decrypt %q %v", pass, err)
			}
			if cookie := r.Header.Get("Cookie"); !strings.Contains(cookie, "MODULUS=") || !strings.Contains(cookie, "RSA_Value=1") {
				t.Errorf("key cookies missing: %s", cookie)
			}
			w.Header().Set("Content-Type", "application/json")
			http.SetCookie(w, &http.Cookie{Name: "QSESSIONID", Value: "abc123456789"})
			http.SetCookie(w, &http.Cookie{Name: "extended_privilege", Value: "7"})
			io.WriteString(w, `{"CSRFToken":"csrf-token-value","ok":0}`)
		case r.URL.Path == "/api/kvm/token":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"token":"tokvalue1"}`)
		case r.URL.Path == "/api/settings/media/instance":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"kvm_port":7590}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	got, err := runH3CJNLP(context.Background(), &ConsoleJNLPInput{Host: u.Host, Username: "admin", Password: "s3cret-pass"})
	if err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 1 || keys.Load() != 1 {
		t.Fatalf("posts=%d keys=%d", posts.Load(), keys.Load())
	}
	if !strings.Contains(got, "<argument>tokvalue1</argument>") || !strings.Contains(got, "<argument>7590</argument>") {
		t.Fatalf("jnlp %s", got)
	}
}

func TestH3CTemplateLocationAndIndex(t *testing.T) {
	js := `fetchPort:function(){var u="/api/settings/media/instance";}
jnlpTemplate:function(t,n,i){var a="",s=!0?t||2:0;return a='<jnlp codebase="'+r.urlPath()+'"><argument>'+location.hostname+'</argument><argument>'+n["token"]+'</argument><argument>'+s.toString()+'</argument></jnlp>';}`
	prepared, err := prepareH3CJNLP(js, "https://10.22.41.30")
	if err != nil {
		t.Fatal(err)
	}
	out, err := prepared.render([]byte(`{"token":"tokvalue1"}`), []byte(`{}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"10.22.41.30", "tokvalue1", "<argument>2</argument>", `codebase="https://10.22.41.30"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestH3CScriptPathsDataMain(t *testing.T) {
	page := `<script data-main="index.min.js?v=3.48" src="/libs/require.js?v=3.48"></script>`
	got := h3cScriptPaths(page)
	if len(got) != 1 || got[0] != "/index.min.js?v=3.48" {
		t.Fatalf("paths %v", got)
	}
}

func TestH3CVuePageUsesAppScript(t *testing.T) {
	h3cHoldLogin = false
	defer func() { h3cHoldLogin = true }()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	var indexHits atomic.Int32
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			io.WriteString(w, `<html><script src="/js/chunk-vendors.0a42ef05.js"></script><script src="/js/app.4036c523.js"></script></html>`)
		case r.URL.Path == "/js/app.4036c523.js":
			if r.Header.Get("Accept-Encoding") != "identity" {
				t.Errorf("accept-encoding %s", r.Header.Get("Accept-Encoding"))
			}
			io.WriteString(w, h3cFixtureJS)
		case r.URL.Path == "/index.min.js" || strings.Contains(r.URL.Path, "vendors"):
			indexHits.Add(1)
			http.NotFound(w, r)
		case r.URL.Path == "/api/rsapublickey":
			io.WriteString(w, `{"RSA_Value":1,"exponent":"`+hex.EncodeToString(big.NewInt(int64(key.E)).Bytes())+`","modulus":"`+key.N.Text(16)+`","cc":0}`)
		case r.URL.Path == "/api/session" && r.Method == http.MethodPost:
			posts.Add(1)
			w.Header().Set("Content-Type", "application/json")
			http.SetCookie(w, &http.Cookie{Name: "QSESSIONID", Value: "abc123456789"})
			io.WriteString(w, `{"CSRFToken":"csrf-token-value"}`)
		case r.URL.Path == "/api/kvm/token":
			io.WriteString(w, `{"token":"tokvalue1"}`)
		case r.URL.Path == "/api/settings/media/instance":
			io.WriteString(w, `{"kvm_port":7590}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	got, err := runH3CJNLP(context.Background(), &ConsoleJNLPInput{Host: u.Host, Username: "admin", Password: "s3cret-pass"})
	if err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 1 || indexHits.Load() != 0 {
		t.Fatalf("posts=%d guessed_scripts=%d", posts.Load(), indexHits.Load())
	}
	if !strings.Contains(got, "<argument>7590</argument>") {
		t.Fatalf("jnlp %s", got)
	}
}

func TestH3CDoesNotLoginWhenTemplateIsMissing(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			io.WriteString(w, `<html><script src="/index.min.js"></script></html>`)
		case "/index.min.js":
			io.WriteString(w, `var x=1;`)
		default:
			posts.Add(1)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_, err := runH3CJNLP(context.Background(), &ConsoleJNLPInput{Host: u.Host, Username: "admin", Password: "s3cret-pass"})
	if err == nil {
		t.Fatal("expected error")
	}
	if posts.Load() != 0 {
		t.Fatalf("requests after the script: %d", posts.Load())
	}
}

func TestH3CLoginRejectStops(t *testing.T) {
	h3cHoldLogin = false
	defer func() { h3cHoldLogin = true }()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	var tokens atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			io.WriteString(w, `<html><script src="/index.min.js"></script></html>`)
		case "/index.min.js":
			io.WriteString(w, h3cFixtureJS)
		case "/api/rsapublickey":
			io.WriteString(w, `{"RSA_Value":1,"exponent":"`+hex.EncodeToString(big.NewInt(int64(key.E)).Bytes())+`","modulus":"`+key.N.Text(16)+`","cc":0}`)
		case "/api/session":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"cc":1}`)
		default:
			if r.URL.Path == "/api/kvm/token" || strings.Contains(r.URL.Path, "media") {
				tokens.Add(1)
			}
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_, err = runH3CJNLP(context.Background(), &ConsoleJNLPInput{Host: u.Host, Username: "admin", Password: "s3cret-pass"})
	if err == nil {
		t.Fatal("expected login rejection")
	}
	if tokens.Load() != 0 {
		t.Fatalf("requests after rejected login: %d", tokens.Load())
	}
}

func TestH3CG7EncryptKeepsLeadingZero(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	got, err := h3cG7EncryptField(key.N.Text(16), "010001", "admin")
	if err != nil {
		t.Fatal(err)
	}
	h := h3cB64ToHex(got)
	width := (key.N.BitLen() + 7) / 8 * 2
	if len(h) == width+1 && strings.HasSuffix(h, "0") {
		h = h[:width]
	}
	raw, err := hex.DecodeString(h)
	if err != nil || len(raw) != width/2 {
		t.Fatalf("cipher bytes=%d hex=%d err=%v", len(raw), len(h), err)
	}
	plain, err := h3cTestDecrypt(key, got)
	if err != nil || plain != "admin" {
		t.Fatalf("plain %q %v", plain, err)
	}
}

func TestH3CG7JNLP(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	var logins, starts, downloads atomic.Int32
	js := `setPublic("MODULUS","EXPONENT");RSAEncryptionEnabled:!0;`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/js/app.4036c523.js" && r.Header.Get("Request-Via") != "WEB" {
			t.Errorf("path %s via %q", r.URL.Path, r.Header.Get("Request-Via"))
		}
		switch {
		case r.URL.Path == "/":
			io.WriteString(w, `<html><script src="/js/app.4036c523.js"></script></html>`)
		case r.URL.Path == "/js/app.4036c523.js":
			io.WriteString(w, js)
		case r.URL.Path == h3cG7SecurityService && r.Method == http.MethodGet:
			io.WriteString(w, `{"Oem":{"Public":{"PublicKeyModulus":"`+key.N.Text(16)+`","PublicKeyExponent":"010001","RSAIndex":7,"CompletionCode":0}}}`)
		case r.URL.Path == h3cG7CreateSession && r.Method == http.MethodPost:
			logins.Add(1)
			if !strings.Contains(r.Header.Get("Cookie"), "RSAIndex=7") {
				t.Errorf("cookie %q", r.Header.Get("Cookie"))
			}
			var body struct {
				UserName string
				Password string
				Enabled  bool `json:"RSAEncryptionEnabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode %v", err)
				return
			}
			if !body.Enabled || body.UserName == "admin" {
				t.Errorf("user %q enabled %v", body.UserName, body.Enabled)
			}
			user, err := h3cTestDecrypt(key, body.UserName)
			pass, err2 := h3cTestDecrypt(key, body.Password)
			if err != nil || err2 != nil || user != "admin" || pass != "s3cret-pass" {
				t.Errorf("user %q pass %q err %v %v", user, pass, err, err2)
			}
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"Oem":{"Public":{"X-Auth-Token":"tokvalue1","CompletionCode":0}}}`)
		case r.URL.Path == h3cG7StartKvm && r.Method == http.MethodPost:
			starts.Add(1)
			if r.Header.Get("X-Auth-Token") != "tokvalue1" {
				t.Errorf("token %q", r.Header.Get("X-Auth-Token"))
			}
			var body struct{ Language string }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Language != "EN" {
				t.Errorf("language %q %v", body.Language, err)
			}
			io.WriteString(w, `{"Oem":{"Public":{"CompletionCode":0,"Description":"Success"}}}`)
		case r.URL.Path == h3cG7Download && r.Method == http.MethodPost:
			downloads.Add(1)
			var body struct {
				TransferProtocol string
				Content          string
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TransferProtocol != "HTTPS" || body.Content != h3cG7JNLPPath {
				t.Errorf("download %+v %v", body, err)
			}
			io.WriteString(w, `<?xml version="1.0"?><jnlp spec="1.0+"><argument>7590</argument></jnlp>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	got, err := runH3CJNLP(context.Background(), &ConsoleJNLPInput{Host: u.Host, Username: "admin", Password: "s3cret-pass"})
	if err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 1 || starts.Load() != 1 || downloads.Load() != 1 || !strings.Contains(got, "<jnlp") {
		t.Fatalf("login=%d start=%d download=%d jnlp=%s", logins.Load(), starts.Load(), downloads.Load(), got)
	}
}

func TestH3CG7LoginRejectStops(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	var later atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			io.WriteString(w, `<html><script src="/js/app.4036c523.js"></script></html>`)
		case r.URL.Path == "/js/app.4036c523.js":
			io.WriteString(w, `setPublic("MODULUS");RSAEncryptionEnabled:!0;`)
		case r.URL.Path == h3cG7SecurityService:
			io.WriteString(w, `{"Oem":{"Public":{"PublicKeyModulus":"`+key.N.Text(16)+`","PublicKeyExponent":"010001","RSAIndex":7,"CompletionCode":0}}}`)
		case r.URL.Path == h3cG7CreateSession:
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"Oem":{"Public":{"CompletionCode":1291845637}}}`)
		default:
			later.Add(1)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_, err = runH3CJNLP(context.Background(), &ConsoleJNLPInput{Host: u.Host, Username: "admin", Password: "s3cret-pass"})
	if err == nil {
		t.Fatal("expected rejected login")
	}
	if later.Load() != 0 {
		t.Fatalf("requests after rejected login: %d", later.Load())
	}
}

func TestH3CG7DoesNotPostWithoutKey(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			io.WriteString(w, `<html><script src="/js/app.4036c523.js"></script></html>`)
		case "/js/app.4036c523.js":
			io.WriteString(w, `setPublic("MODULUS");RSAEncryptionEnabled:!0;`)
		default:
			if r.Method == http.MethodPost {
				posts.Add(1)
			}
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_, err := runH3CJNLP(context.Background(), &ConsoleJNLPInput{Host: u.Host, Username: "admin", Password: "s3cret-pass"})
	if err == nil {
		t.Fatal("expected missing key")
	}
	if posts.Load() != 0 {
		t.Fatalf("password posts: %d", posts.Load())
	}
}

func h3cTestDecrypt(key *rsa.PrivateKey, field string) (string, error) {
	field = strings.ReplaceAll(field, "\n", "")
	h := h3cB64ToHex(field)
	size := (key.N.BitLen() + 7) / 8 * 2
	// hex2b64 of a width that is 1 mod 3 decodes with one trailing zero nibble.
	if len(h) == size+1 && strings.HasSuffix(h, "0") {
		h = h[:size]
	}
	if len(h)%2 == 1 {
		h = "0" + h
	}
	if len(h) > size {
		return "", fmt.Errorf("ciphertext hex %d longer than modulus", len(h))
	}
	buf, err := hex.DecodeString(strings.Repeat("0", size-len(h)) + h)
	if err != nil {
		return "", err
	}
	plain, err := rsa.DecryptPKCS1v15(nil, key, buf)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func h3cB64ToHex(s string) string {
	const digits = "0123456789abcdef"
	var b strings.Builder
	k := 0
	var slop int
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			continue
		}
		v := strings.IndexByte(h3cB64, s[i])
		if v < 0 {
			continue
		}
		switch k {
		case 0:
			b.WriteByte(digits[v>>2])
			slop = v & 3
			k = 1
		case 1:
			b.WriteByte(digits[(slop<<2)|(v>>4)])
			slop = v & 0xf
			k = 2
		case 2:
			b.WriteByte(digits[slop])
			b.WriteByte(digits[v>>2])
			slop = v & 3
			k = 3
		default:
			b.WriteByte(digits[(slop<<2)|(v>>4)])
			b.WriteByte(digits[v&0xf])
			k = 0
		}
	}
	if k == 1 {
		b.WriteByte(digits[slop<<2])
	}
	return b.String()
}
