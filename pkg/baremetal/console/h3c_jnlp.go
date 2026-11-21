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
	"math"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
)

// H3C HDM G5 builds KVM.jnlp in the browser. index.min.js login() encrypts the
// username and the password with the key from GET /api/rsapublickey, then
// form-posts /api/session once. javaKvm then reads /api/kvm/token and the
// media instance, and jnlpTemplate concatenates the file. The password is
// submitted only after that template parses. sendLogin posts that object as a
// form. hex2b64 is jsbn's hex encoder.
//
// h3cHoldLogin blocks the password POST. The G5 login sends the RSA_Value
// cookie from the same public-key response.
//
// G7 is a different Vue UI. It reads the key from Managers/1/SecurityService,
// encrypts both fields with the full-width hex form of PKCS#1, and posts
// SessionService.CreateSession once. StartKvm then GeneralDownload returns
// /tmp/web/KVM.jnlp.
var h3cHoldLogin = false

func runH3CJNLP(ctx context.Context, input *ConsoleJNLPInput) (string, error) {
	const tag = "console-jnlp"
	log.Infof("%s start driver=h3c host=%s model=%q sn=%q username=%q", tag, input.Host, input.Model, input.SerialNumber, input.Username)
	var last error
	for _, scheme := range []string{"https", "http"} {
		base := fmt.Sprintf("%s://%s", scheme, input.Host)
		result, connected, err := h3cJNLPAt(ctx, base, input)
		if !connected {
			log.Infof("%s driver=h3c host=%s scheme=%s unreachable err=%v", tag, input.Host, scheme, err)
			last = err
			continue
		}
		if err != nil {
			log.Infof("%s done driver=h3c host=%s scheme=%s result=error err=%s", tag, input.Host, scheme, err)
			return "", err
		}
		log.Infof("%s done driver=h3c host=%s scheme=%s result=ok bytes=%d", tag, input.Host, scheme, len(result))
		return result, nil
	}
	if last == nil {
		last = errors.Error("bmc web console unreachable")
	}
	return "", errors.Wrap(last, "bmc web console unreachable")
}

func h3cJNLPAt(ctx context.Context, base string, input *ConsoleJNLPInput) (string, bool, error) {
	const tag = "console-jnlp"
	client := newProbeClient()
	status, _, page, err := exchange(ctx, client, tag, "h3c", input.Password, base, http.MethodGet, "/", nil, nil)
	if err != nil && status == 0 {
		return "", false, err
	}
	js, err := h3cLoadScript(ctx, client, tag, input.Host, input.Password, base, string(page))
	if err != nil {
		log.Infof("%s driver=h3c host=%s login skipped reason=%s", tag, input.Host, err)
		return "", true, err
	}
	if h3cG7Page(js) && !h3cLoginSupported(js) {
		return h3cG7At(ctx, client, tag, base, input, js)
	}
	prepared, err := prepareH3CJNLP(js, base)
	if err != nil {
		log.Infof("%s driver=h3c host=%s login skipped reason=%s note=%s", tag, input.Host, err, h3cScriptNote(js, input.Password))
		return "", true, err
	}
	if h3cHoldLogin {
		log.Infof("%s driver=h3c host=%s login not submitted: previous page-format login returned cc=1", tag, input.Host)
		return "", true, errors.Error("h3c login is not submitted again after cc=1")
	}
	h3cPrimeSession(ctx, client, tag, input.Password, base)
	keyStatus, _, keyBody, err := exchange(ctx, client, tag, "h3c", input.Password, base, http.MethodGet, "/api/rsapublickey", h3cAjaxHeader(base), nil)
	if err != nil && keyStatus == 0 {
		return "", true, err
	}
	if keyStatus != http.StatusOK {
		return "", true, errors.Errorf("h3c public key status=%d", keyStatus)
	}
	var key struct {
		Exponent string          `json:"exponent"`
		Modulus  string          `json:"modulus"`
		RSAValue json.RawMessage `json:"RSA_Value"`
		CC       int             `json:"cc"`
	}
	if err := json.Unmarshal(keyBody, &key); err != nil {
		return "", true, errors.Wrap(err, "h3c public key")
	}
	if key.CC != 0 || key.Modulus == "" || key.Exponent == "" {
		return "", true, errors.Error("h3c public key response is incomplete")
	}
	encUser, err := h3cEncryptField(key.Modulus, key.Exponent, input.Username, prepared.lineWidth)
	if err != nil {
		return "", true, errors.Wrap(err, "h3c encrypt username")
	}
	encPass, err := h3cEncryptField(key.Modulus, key.Exponent, input.Password, prepared.lineWidth)
	if err != nil {
		return "", true, errors.Wrap(err, "h3c encrypt password")
	}
	if err := h3cSetKeyCookies(client, base, key.Modulus, key.Exponent, h3cRSACookie(key.RSAValue)); err != nil {
		return "", true, err
	}
	body := "username=" + url.QueryEscape(encUser) + "&password=" + url.QueryEscape(encPass) + "&log_type=1"
	log.Infof("%s driver=h3c host=%s login encrypt user_len=%d pass_len=%d jar=%s", tag, input.Host, len(encUser), len(encPass), h3cJarNames(client, base))
	hdr := h3cAjaxHeader(base)
	hdr.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	loginStatus, loginHdr, loginBody, err := exchange(ctx, client, tag, "h3c", input.Password, base, http.MethodPost, "/api/session", hdr, []byte(body))
	if err != nil && loginStatus == 0 {
		return "", true, err
	}
	if loginStatus < 200 || loginStatus >= 300 || h3cRejected(loginBody) {
		log.Infof("%s driver=h3c host=%s login /api/session status=%d cc=%d login_fail_time=%s, stop further login to avoid account lockout", tag, input.Host, loginStatus, h3cCC(loginBody), h3cFailTime(loginBody))
		return "", true, errors.Errorf("h3c login status=%d cc=%d", loginStatus, h3cCC(loginBody))
	}
	cookies, extra := sessionFromResponse(loginHdr, loginBody)
	for _, c := range (&http.Response{Header: loginHdr}).Cookies() {
		cookies[c.Name] = c.Value
	}
	session := sessionHeader(base, "application/json, text/javascript, */*", cookies, extra)
	tokenStatus, _, tokenBody, err := exchange(ctx, client, tag, "h3c", input.Password, base, http.MethodGet, "/api/kvm/token", session, nil)
	if err != nil && tokenStatus == 0 {
		return "", true, err
	}
	if tokenStatus != http.StatusOK {
		return "", true, errors.Errorf("h3c kvm token status=%d", tokenStatus)
	}
	mediaStatus, _, mediaBody, err := exchange(ctx, client, tag, "h3c", input.Password, base, http.MethodGet, prepared.mediaPath, session, nil)
	if err != nil && mediaStatus == 0 {
		return "", true, err
	}
	if mediaStatus != http.StatusOK {
		return "", true, errors.Errorf("h3c %s status=%d", prepared.mediaPath, mediaStatus)
	}
	jnlp, err := prepared.render(tokenBody, mediaBody, cookies)
	if err != nil {
		return "", true, err
	}
	if !looksLikeJNLP([]byte(jnlp)) {
		return "", true, errors.Error("h3c jnlp template did not produce a jnlp file")
	}
	return jnlp, true, nil
}

type h3cPrepared struct {
	fn        *jsFunc
	mediaPath string
	lineWidth int
	base      string
}

func prepareH3CJNLP(js, base string) (*h3cPrepared, error) {
	fnSrc, err := extractJSFunction(js, "jnlpTemplate:function")
	if err != nil {
		return nil, errors.Wrap(err, "h3c jnlp template")
	}
	fn, err := parseJSFunction(fnSrc)
	if err != nil {
		return nil, errors.Wrap(err, "h3c jnlp template")
	}
	if len(fn.params) < 3 {
		return nil, errors.Error("h3c jnlp template has no token parameters")
	}
	mediaFn, err := extractJSFunction(js, "fetchPort:function")
	if err != nil {
		return nil, errors.Wrap(err, "h3c media path")
	}
	mediaPath := h3cMediaPath(mediaFn)
	if mediaPath == "" {
		return nil, errors.Error("h3c media path is not in fetchPort")
	}
	prepared := &h3cPrepared{fn: fn, mediaPath: mediaPath, lineWidth: h3cLoginLineWidth(js), base: base}
	if _, err := prepared.render([]byte(`{}`), []byte(`{}`), nil); err != nil {
		return nil, errors.Wrap(err, "h3c jnlp template")
	}
	return prepared, nil
}

func (p *h3cPrepared) render(token, media []byte, cookies map[string]string) (string, error) {
	tokenVal, err := jsFromJSON(token)
	if err != nil {
		return "", errors.Wrap(err, "h3c kvm token")
	}
	mediaVal, err := jsFromJSON(media)
	if err != nil {
		return "", errors.Wrap(err, "h3c media settings")
	}
	env := &jsEnv{vars: map[string]jsVal{}, cookies: cookies, base: p.base}
	env.vars[p.fn.params[0]] = jsNum(2)
	env.vars[p.fn.params[1]] = tokenVal
	env.vars[p.fn.params[2]] = mediaVal
	for _, b := range p.fn.vars {
		val, err := evalJS(b.expr, env)
		if err != nil {
			return "", err
		}
		env.vars[b.name] = val
	}
	out, err := evalJS(p.fn.ret, env)
	if err != nil {
		return "", err
	}
	return out.jsString(), nil
}

func h3cLoadScript(ctx context.Context, client *http.Client, tag, host, password, base, page string) (string, error) {
	paths := h3cScriptPaths(page)
	if len(paths) == 0 {
		return "", errors.Error("h3c page has no script")
	}
	hdr := http.Header{}
	hdr.Set("Accept-Encoding", "identity")
	var sample, samplePath string
	var last error
	for _, path := range paths {
		code, body, err := fetchQuiet(ctx, client, base, path, hdr)
		if err != nil || code != http.StatusOK || len(body) == 0 {
			log.Infof("%s driver=h3c host=%s script=%s status=%d err=%v", tag, host, path, code, err)
			if err != nil {
				last = err
			} else {
				last = errors.Errorf("h3c script %s status=%d", path, code)
			}
			continue
		}
		text := string(body)
		if h3cLoginSupported(text) && strings.Contains(text, "jnlpTemplate:function") {
			log.Infof("%s driver=h3c host=%s script=%s bytes=%d login=known", tag, host, path, len(body))
			return text, nil
		}
		if h3cG7Page(text) {
			log.Infof("%s driver=h3c host=%s script=%s bytes=%d login=g7", tag, host, path, len(body))
			return text, nil
		}
		if sample == "" || strings.Contains(path, "/js/app.") {
			sample, samplePath = text, path
		}
		log.Infof("%s driver=h3c host=%s script=%s bytes=%d login=unknown", tag, host, path, len(body))
	}
	if sample != "" {
		log.Infof("%s driver=h3c host=%s script=%s note=%s", tag, host, samplePath, h3cScriptNote(sample, password))
		return "", errors.Errorf("h3c script %s has no known jnlp login", samplePath)
	}
	if last == nil {
		last = errors.Error("h3c script was not loaded")
	}
	return "", last
}

func h3cG7Page(js string) bool {
	return strings.Contains(js, "RSAEncryptionEnabled") && strings.Contains(js, "setPublic(")
}

const (
	h3cG7SecurityService = "/redfish/v1/Managers/1/SecurityService"
	h3cG7CreateSession   = "/redfish/v1/SessionService/Actions/Oem/Public/SessionService.CreateSession"
	h3cG7StartKvm        = "/redfish/v1/Managers/1/KvmService/Actions/Oem/Public/KvmService.StartKvm"
	h3cG7Download        = "/redfish/v1/Managers/1/Actions/Oem/Public/Manager.GeneralDownload"
	h3cG7JNLPPath        = "/tmp/web/KVM.jnlp"
)

func h3cG7At(ctx context.Context, client *http.Client, tag, base string, input *ConsoleJNLPInput, _ string) (string, bool, error) {
	if client.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return "", true, errors.Wrap(err, "h3c g7 cookie jar")
		}
		client.Jar = jar
	}
	hdr := h3cG7Header(base)
	keyStatus, _, keyBody, err := exchange(ctx, client, tag, "h3c", input.Password, base, http.MethodGet, h3cG7SecurityService, hdr, nil)
	if err != nil && keyStatus == 0 {
		return "", true, err
	}
	var keyParsed interface{}
	if keyStatus != http.StatusOK || json.Unmarshal(keyBody, &keyParsed) != nil {
		return "", true, errors.Errorf("h3c g7 security service status=%d", keyStatus)
	}
	if code, ok := h3cFindNumber(keyParsed, "CompletionCode"); ok && code != 0 {
		return "", true, errors.Errorf("h3c g7 security service completion=%d", code)
	}
	pub := h3cG7OemPublic(keyParsed)
	modulus, _ := pub["PublicKeyModulus"].(string)
	exponent := h3cG7Exponent(pub["PublicKeyExponent"])
	index := h3cG7Index(pub["RSAIndex"])
	if modulus == "" || exponent == "" || index == "" {
		return "", true, errors.Error("h3c g7 security service has no rsa key")
	}
	if err := h3cSetCookie(client, base, "RSAIndex", index); err != nil {
		return "", true, err
	}
	encUser, err := h3cG7EncryptField(modulus, exponent, input.Username)
	if err != nil {
		return "", true, errors.Wrap(err, "h3c g7 encrypt username")
	}
	encPass, err := h3cG7EncryptField(modulus, exponent, input.Password)
	if err != nil {
		return "", true, errors.Wrap(err, "h3c g7 encrypt password")
	}
	payload, err := json.Marshal(map[string]interface{}{
		"RSAEncryptionEnabled": true,
		"UserName":             encUser,
		"Password":             encPass,
	})
	if err != nil {
		return "", true, err
	}
	log.Infof("%s driver=h3c host=%s g7 login rsa_index=%s user_len=%d pass_len=%d", tag, input.Host, index, len(encUser), len(encPass))
	loginStatus, loginHdr, loginBody, err := exchange(ctx, client, tag, "h3c", input.Password, base, http.MethodPost, h3cG7CreateSession, hdr, payload)
	if err != nil && loginStatus == 0 {
		return "", true, err
	}
	var loginParsed interface{}
	_ = json.Unmarshal(loginBody, &loginParsed)
	code, hasCode := h3cFindNumber(loginParsed, "CompletionCode")
	if loginStatus < 200 || loginStatus >= 300 || (hasCode && code != 0) {
		log.Infof("%s driver=h3c host=%s g7 login status=%d completion=%d, stop further login", tag, input.Host, loginStatus, code)
		return "", true, errors.Errorf("h3c g7 login status=%d completion=%d", loginStatus, code)
	}
	token := h3cFindString(loginParsed, "X-Auth-Token")
	if token == "" && loginHdr != nil {
		token = loginHdr.Get("X-Auth-Token")
	}
	if token == "" {
		return "", true, errors.Error("h3c g7 login returned no token")
	}
	auth := h3cG7Header(base)
	auth.Set("X-Auth-Token", token)
	startBody, err := json.Marshal(map[string]string{"Language": "EN"})
	if err != nil {
		return "", true, err
	}
	startStatus, _, startResp, err := exchange(ctx, client, tag, "h3c", input.Password, base, http.MethodPost, h3cG7StartKvm, auth, startBody)
	if err != nil && startStatus == 0 {
		return "", true, err
	}
	var startParsed interface{}
	_ = json.Unmarshal(startResp, &startParsed)
	startCode, startHasCode := h3cFindNumber(startParsed, "CompletionCode")
	log.Infof("%s driver=h3c host=%s g7 startkvm status=%d completion=%d", tag, input.Host, startStatus, startCode)
	if startStatus < 200 || startStatus >= 300 || (startHasCode && startCode != 0) {
		return "", true, errors.Errorf("h3c g7 startkvm status=%d completion=%d", startStatus, startCode)
	}
	downBody, err := json.Marshal(map[string]string{
		"TransferProtocol": "HTTPS",
		"Content":          h3cG7JNLPPath,
	})
	if err != nil {
		return "", true, err
	}
	downStatus, _, downResp, err := exchange(ctx, client, tag, "h3c", input.Password, base, http.MethodPost, h3cG7Download, auth, downBody)
	if err != nil && downStatus == 0 {
		return "", true, err
	}
	log.Infof("%s driver=h3c host=%s g7 download status=%d bytes=%d", tag, input.Host, downStatus, len(downResp))
	if looksLikeJNLP(downResp) {
		return string(downResp), true, nil
	}
	if content := h3cFindString(startParsed, "Content"); looksLikeJNLP([]byte(content)) {
		return content, true, nil
	}
	return "", true, errors.Errorf("h3c g7 download status=%d has no jnlp", downStatus)
}

func h3cG7Header(base string) http.Header {
	hdr := http.Header{}
	hdr.Set("Accept", "application/json, text/plain, */*")
	hdr.Set("Content-Type", "application/json;charset=UTF-8")
	hdr.Set("Request-Via", "WEB")
	hdr.Set("Referer", base+"/")
	hdr.Set("Origin", base)
	return hdr
}

func h3cG7OemPublic(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	oem, _ := m["Oem"].(map[string]interface{})
	pub, _ := oem["Public"].(map[string]interface{})
	if pub == nil {
		return map[string]interface{}{}
	}
	return pub
}

func h3cG7Exponent(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return fmt.Sprintf("%X", int64(t))
	default:
		return ""
	}
}

func h3cG7Index(v interface{}) string {
	switch t := v.(type) {
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case string:
		return strings.TrimSpace(t)
	default:
		return ""
	}
}

// h3cG7EncryptField pads the PKCS#1 block to the full modulus width before
// hex2b64. G5 strips leading zero bytes; G7's RSAKey.encrypt does not.
func h3cG7EncryptField(modulusHex, exponentHex, plain string) (string, error) {
	n := new(big.Int)
	if _, ok := n.SetString(strings.TrimSpace(modulusHex), 16); !ok {
		return "", errors.Error("invalid modulus")
	}
	e := new(big.Int)
	if _, ok := e.SetString(strings.TrimSpace(exponentHex), 16); !ok || !e.IsInt64() {
		return "", errors.Error("invalid exponent")
	}
	enc, err := rsa.EncryptPKCS1v15(rand.Reader, &rsa.PublicKey{N: n, E: int(e.Int64())}, []byte(plain))
	if err != nil {
		return "", err
	}
	width := (n.BitLen() + 7) / 8 * 2
	h := hex.EncodeToString(enc)
	if len(h) < width {
		h = strings.Repeat("0", width-len(h)) + h
	}
	return hex2b64(h), nil
}

func h3cSetCookie(client *http.Client, base, name, value string) error {
	if client.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return errors.Wrap(err, "h3c cookie jar")
		}
		client.Jar = jar
	}
	u, err := url.Parse(base + "/")
	if err != nil {
		return errors.Wrap(err, "h3c cookie url")
	}
	client.Jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
	return nil
}

func h3cFindString(v interface{}, names ...string) string {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			for _, name := range names {
				if strings.EqualFold(k, name) {
					if s, ok := val.(string); ok && s != "" {
						return s
					}
				}
			}
		}
		for _, val := range t {
			if s := h3cFindString(val, names...); s != "" {
				return s
			}
		}
	case []interface{}:
		for _, val := range t {
			if s := h3cFindString(val, names...); s != "" {
				return s
			}
		}
	}
	return ""
}

func h3cFindNumber(v interface{}, names ...string) (int, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			for _, name := range names {
				if strings.EqualFold(k, name) {
					if n, ok := val.(float64); ok {
						return int(n), true
					}
				}
			}
		}
		for _, val := range t {
			if n, ok := h3cFindNumber(val, names...); ok {
				return n, true
			}
		}
	case []interface{}:
		for _, val := range t {
			if n, ok := h3cFindNumber(val, names...); ok {
				return n, true
			}
		}
	}
	return 0, false
}

func h3cLoginSupported(js string) bool {
	login, err := extractJSFunction(js, "login:function")
	if err != nil {
		return false
	}
	return strings.Contains(login, "linebrk(hex2b64(") && strings.Contains(login, "log_type")
}

func h3cScriptPaths(page string) []string {
	reSrc := regexp.MustCompile(`(?i)src=["']([^"']+\.js[^"']*)["']`)
	reMain := regexp.MustCompile(`(?i)data-main=["']([^"']+\.js[^"']*)["']`)
	var mains, preferred, rest []string
	seen := map[string]bool{}
	add := func(raw string, main bool) {
		path := h3cAbsScript(raw)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		low := strings.ToLower(path)
		if main || strings.Contains(low, "index.min.js") || strings.Contains(low, "/js/app.") || strings.Contains(low, "/app.") {
			if main {
				mains = append(mains, path)
			} else {
				preferred = append(preferred, path)
			}
			return
		}
		if strings.Contains(low, "vendors") || strings.Contains(low, "ant-design") || strings.Contains(low, "require.js") {
			return
		}
		rest = append(rest, path)
	}
	for _, m := range reMain.FindAllStringSubmatch(page, -1) {
		add(m[1], true)
	}
	for _, m := range reSrc.FindAllStringSubmatch(page, -1) {
		add(m[1], false)
	}
	return append(append(mains, preferred...), rest...)
}

func h3cAbsScript(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return raw
}

func h3cScriptNote(js, password string) string {
	lower := strings.ToLower(js)
	needles := []string{"login:function", "jnlptemplate:function", "rsapublickey", "setpublic(", "/api/session"}
	parts := make([]string, 0, len(needles))
	for _, needle := range needles {
		i := strings.Index(lower, needle)
		if i < 0 {
			continue
		}
		begin := i - 40
		if begin < 0 {
			begin = 0
		}
		end := i + 500
		if end > len(js) {
			end = len(js)
		}
		parts = append(parts, strings.Join(strings.Fields(js[begin:end]), " "))
	}
	if len(parts) == 0 {
		return "no login markers"
	}
	return redactPasswordOnly(strings.Join(parts, " || "), password)
}

func h3cMediaPath(fn string) string {
	re := regexp.MustCompile(`["'](/api/[^"']+)["']`)
	found := ""
	for _, m := range re.FindAllStringSubmatch(fn, -1) {
		if strings.Contains(m[1], "/settings/media/") {
			return m[1]
		}
		if found == "" {
			found = m[1]
		}
	}
	return found
}

func h3cLoginLineWidth(js string) int {
	login, err := extractJSFunction(js, "login:function")
	if err != nil {
		return h3cLineBreakDefault(js)
	}
	const call = "linebrk(hex2b64("
	i := strings.Index(login, call)
	if i < 0 {
		return h3cLineBreakDefault(js)
	}
	depth := 0
	for j := i + len("linebrk"); j < len(login); j++ {
		switch login[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return h3cLineBreakDefault(js)
			}
		case ',':
			if depth == 1 {
				rest := strings.TrimSpace(login[j+1:])
				if strings.HasPrefix(rest, "64") {
					return 64
				}
				return h3cLineBreakDefault(js)
			}
		}
	}
	return h3cLineBreakDefault(js)
}

func h3cLineBreakDefault(js string) int {
	for _, marker := range []string{"function linebrk", "linebrk=function", "linebrk:function"} {
		body, err := extractJSFunction(js, marker)
		if err != nil {
			continue
		}
		if strings.Contains(body, "||64") || strings.Contains(body, "|| 64") {
			return 64
		}
		return 0
	}
	return 0
}

func h3cEncryptField(modulusHex, exponentHex, plain string, lineWidth int) (string, error) {
	n := new(big.Int)
	if _, ok := n.SetString(strings.TrimSpace(modulusHex), 16); !ok {
		return "", errors.Error("invalid modulus")
	}
	e := new(big.Int)
	if _, ok := e.SetString(strings.TrimSpace(exponentHex), 16); !ok || !e.IsInt64() {
		return "", errors.Error("invalid exponent")
	}
	enc, err := rsa.EncryptPKCS1v15(rand.Reader, &rsa.PublicKey{N: n, E: int(e.Int64())}, []byte(plain))
	if err != nil {
		return "", err
	}
	// RSAKey.encrypt returns hex without leading zero bytes. hex2b64 then
	// groups those hex digits. A full-width standard Base64 block is a
	// different string.
	return h3cLineBreak(hex2b64(jsbnHex(enc)), lineWidth), nil
}

func jsbnHex(block []byte) string {
	i := 0
	for i < len(block)-1 && block[i] == 0 {
		i++
	}
	h := hex.EncodeToString(block[i:])
	if len(h)%2 == 1 {
		h = "0" + h
	}
	return h
}

func h3cAjaxHeader(base string) http.Header {
	hdr := http.Header{}
	hdr.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	hdr.Set("X-Requested-With", "XMLHttpRequest")
	hdr.Set("Referer", base+"/")
	return hdr
}

// h3cPrimeSession repeats the anonymous calls the login page makes before it
// reads the RSA key. A session cookie from one of them is what later requests
// must send. None of these calls carries the password.
func h3cPrimeSession(ctx context.Context, client *http.Client, tag, password, base string) {
	if client.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			log.Infof("%s driver=h3c jar err=%v", tag, err)
			return
		}
		client.Jar = jar
	}
	hdr := h3cAjaxHeader(base)
	jsonHdr := hdr.Clone()
	jsonHdr.Set("Content-Type", "application/json; charset=UTF-8")
	exchange(ctx, client, tag, "h3c", password, base, http.MethodPost, "/api/fru", jsonHdr, []byte(`{"type":"dashboard"}`))
	for _, path := range []string{"/api/settings/dns/host", "/api/system/firmware", "/api/tfa_status", "/api/safe_switch"} {
		exchange(ctx, client, tag, "h3c", password, base, http.MethodGet, path, hdr, nil)
	}
	log.Infof("%s driver=h3c host=%s prime jar=%s", tag, hostOf(base), h3cJarNames(client, base))
}

func h3cRSACookie(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	return strings.Trim(s, `"`)
}

func h3cFailTime(body []byte) string {
	var v struct {
		Time *int `json:"login_fail_time"`
	}
	if json.Unmarshal(body, &v) != nil || v.Time == nil {
		return "absent"
	}
	return strconv.Itoa(*v.Time)
}

func h3cSetKeyCookies(client *http.Client, base, modulus, exponent, rsaValue string) error {
	if client.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return errors.Wrap(err, "h3c cookie jar")
		}
		client.Jar = jar
	}
	u, err := url.Parse(base + "/")
	if err != nil {
		return errors.Wrap(err, "h3c cookie url")
	}
	client.Jar.SetCookies(u, []*http.Cookie{
		{Name: "MODULUS", Value: modulus, Path: "/"},
		{Name: "EXPONENT", Value: exponent, Path: "/"},
		{Name: "RSA_Value", Value: rsaValue, Path: "/"},
	})
	return nil
}

func h3cJarNames(client *http.Client, base string) string {
	if client == nil || client.Jar == nil {
		return ""
	}
	u, err := url.Parse(base + "/")
	if err != nil {
		return ""
	}
	var names []string
	for _, c := range client.Jar.Cookies(u) {
		names = append(names, fmt.Sprintf("%s(len=%d)", c.Name, len(c.Value)))
	}
	return strings.Join(names, ",")
}

func h3cCC(body []byte) int {
	var v struct {
		CC *int `json:"cc"`
	}
	if json.Unmarshal(body, &v) != nil || v.CC == nil {
		return -1
	}
	return *v.CC
}

func h3cRejected(body []byte) bool {
	cc := h3cCC(body)
	return cc >= 0 && cc != 0
}

const h3cB64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// hex2b64 is the jsbn encoder used by HDM. It groups hex digits, not ciphertext bytes.
func hex2b64(h string) string {
	var b strings.Builder
	i := 0
	for i+3 <= len(h) {
		c, _ := strconv.ParseInt(h[i:i+3], 16, 32)
		b.WriteByte(h3cB64[c>>6])
		b.WriteByte(h3cB64[c&63])
		i += 3
	}
	if i+1 == len(h) {
		c, _ := strconv.ParseInt(h[i:i+1], 16, 32)
		b.WriteByte(h3cB64[c<<2])
	} else if i+2 == len(h) {
		c, _ := strconv.ParseInt(h[i:i+2], 16, 32)
		b.WriteByte(h3cB64[c>>2])
		b.WriteByte(h3cB64[(c&3)<<4])
	}
	for b.Len()%4 > 0 {
		b.WriteByte('=')
	}
	return b.String()
}

func h3cLineBreak(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	var b strings.Builder
	for len(s) > width {
		b.WriteString(s[:width])
		b.WriteByte('\n')
		s = s[width:]
	}
	b.WriteString(s)
	return b.String()
}

func extractJSFunction(src, marker string) (string, error) {
	i := strings.Index(src, marker)
	if i < 0 {
		return "", errors.Errorf("missing %s", marker)
	}
	rest := src[i:]
	brace := strings.Index(rest, "{")
	if brace < 0 {
		return "", errors.Errorf("%s has no body", marker)
	}
	depth := 0
	var quote byte
	esc := false
	for j := brace; j < len(rest); j++ {
		c := rest[j]
		if quote != 0 {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return rest[:j+1], nil
			}
		}
	}
	return "", errors.Errorf("%s body is unclosed", marker)
}

type jsKind int

const (
	kindUndef jsKind = iota
	kindNum
	kindStr
	kindObj
	kindFn
)

type jsVal struct {
	kind jsKind
	num  float64
	nan  bool
	str  string
	obj  map[string]jsVal
}

func jsNum(n float64) jsVal { return jsVal{kind: kindNum, num: n} }

func (v jsVal) truthy() bool {
	switch v.kind {
	case kindNum:
		return !v.nan && v.num != 0
	case kindStr:
		return v.str != ""
	case kindObj, kindFn:
		return true
	default:
		return false
	}
}

func (v jsVal) jsString() string {
	switch v.kind {
	case kindNum:
		if v.nan {
			return "NaN"
		}
		if !math.IsInf(v.num, 0) && v.num == math.Trunc(v.num) {
			return strconv.FormatInt(int64(v.num), 10)
		}
		return strconv.FormatFloat(v.num, 'f', -1, 64)
	case kindStr:
		return v.str
	case kindUndef:
		return "undefined"
	default:
		return ""
	}
}

func jsFromJSON(raw []byte) (jsVal, error) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return jsVal{}, err
	}
	return jsFromAny(v), nil
}

func jsFromAny(v interface{}) jsVal {
	switch t := v.(type) {
	case nil:
		return jsVal{}
	case float64:
		return jsNum(t)
	case string:
		return jsVal{kind: kindStr, str: t}
	case bool:
		if t {
			return jsNum(1)
		}
		return jsNum(0)
	case map[string]interface{}:
		obj := make(map[string]jsVal, len(t))
		for k, child := range t {
			obj[k] = jsFromAny(child)
		}
		return jsVal{kind: kindObj, obj: obj}
	default:
		return jsVal{}
	}
}

type jsExpr interface{ jsExpr() }

type jsLit struct{ v jsVal }
type jsIdent struct{ name string }
type jsMember struct {
	obj  jsExpr
	name string
}
type jsCall struct {
	fn   jsExpr
	args []jsExpr
}
type jsBinary struct {
	op          string
	left, right jsExpr
}
type jsTernary struct{ cond, yes, no jsExpr }
type jsUnary struct{ expr jsExpr }
type jsIndex struct {
	obj jsExpr
	key jsExpr
}

func (jsLit) jsExpr()     {}
func (jsIdent) jsExpr()   {}
func (jsMember) jsExpr()  {}
func (jsCall) jsExpr()    {}
func (jsBinary) jsExpr()  {}
func (jsTernary) jsExpr() {}
func (jsUnary) jsExpr()   {}
func (jsIndex) jsExpr()   {}

type jsBinding struct {
	name string
	expr jsExpr
}

type jsFunc struct {
	params []string
	vars   []jsBinding
	ret    jsExpr
}

type jsEnv struct {
	vars    map[string]jsVal
	cookies map[string]string
	base    string
}

func parseJSFunction(src string) (*jsFunc, error) {
	open := strings.Index(src, "(")
	brace := strings.Index(src, "{")
	if open < 0 || brace < 0 || open > brace {
		return nil, errors.Error("function signature")
	}
	fn := &jsFunc{}
	for _, name := range strings.Split(src[open+1:brace-1], ",") {
		name = strings.TrimSpace(strings.TrimSuffix(name, ")"))
		if name != "" {
			fn.params = append(fn.params, name)
		}
	}
	body := src[brace+1 : len(src)-1]
	p := &jsParser{s: body}
	for {
		p.skip()
		if p.eof() {
			break
		}
		if p.starts("var ") || p.starts("var\t") {
			p.i += 3
			bindings, err := p.parseVar()
			if err != nil {
				return nil, err
			}
			fn.vars = append(fn.vars, bindings...)
			continue
		}
		if p.starts("return") {
			p.i += len("return")
			expr, err := p.parseAssign()
			if err != nil {
				return nil, err
			}
			fn.ret = expr
			break
		}
		return nil, errors.Errorf("unsupported statement near %q", p.snippet())
	}
	if fn.ret == nil {
		return nil, errors.Error("function has no return")
	}
	return fn, nil
}

type jsParser struct {
	s string
	i int
}

func (p *jsParser) eof() bool { return p.i >= len(p.s) }

func (p *jsParser) skip() {
	for !p.eof() {
		switch p.s[p.i] {
		case ' ', '\n', '\t', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *jsParser) starts(s string) bool {
	return strings.HasPrefix(p.s[p.i:], s)
}

func (p *jsParser) snippet() string {
	end := p.i + 40
	if end > len(p.s) {
		end = len(p.s)
	}
	return p.s[p.i:end]
}

func (p *jsParser) parseVar() ([]jsBinding, error) {
	var out []jsBinding
	for {
		p.skip()
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		p.skip()
		if p.eof() || p.s[p.i] != '=' {
			return nil, errors.Errorf("expected assignment near %q", p.snippet())
		}
		p.i++
		expr, err := p.parseTernary()
		if err != nil {
			return nil, err
		}
		out = append(out, jsBinding{name: name, expr: expr})
		p.skip()
		if p.eof() {
			return nil, errors.Error("var statement is unclosed")
		}
		switch p.s[p.i] {
		case ',':
			p.i++
		case ';':
			p.i++
			return out, nil
		default:
			return nil, errors.Errorf("expected comma near %q", p.snippet())
		}
	}
}

func (p *jsParser) parseAssign() (jsExpr, error) {
	p.skip()
	start := p.i
	if _, err := p.parseName(); err == nil {
		p.skip()
		if !p.eof() && p.s[p.i] == '=' && (p.i+1 >= len(p.s) || p.s[p.i+1] != '=') {
			p.i++
			return p.parseTernary()
		}
	}
	p.i = start
	expr, err := p.parseTernary()
	if err != nil {
		return nil, err
	}
	p.skip()
	if !p.eof() && p.s[p.i] == ';' {
		p.i++
	}
	return expr, nil
}

func (p *jsParser) parseTernary() (jsExpr, error) {
	cond, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	p.skip()
	if p.eof() || p.s[p.i] != '?' {
		return cond, nil
	}
	p.i++
	yes, err := p.parseTernary()
	if err != nil {
		return nil, err
	}
	p.skip()
	if p.eof() || p.s[p.i] != ':' {
		return nil, errors.Errorf("expected ':' near %q", p.snippet())
	}
	p.i++
	no, err := p.parseTernary()
	if err != nil {
		return nil, err
	}
	return jsTernary{cond: cond, yes: yes, no: no}, nil
}

func (p *jsParser) parseOr() (jsExpr, error) { return p.parseBinary("||", (*jsParser).parseAnd) }
func (p *jsParser) parseAnd() (jsExpr, error) {
	return p.parseBinary("&&", (*jsParser).parseEq)
}

func (p *jsParser) parseEq() (jsExpr, error) {
	left, err := p.parseBitAnd()
	if err != nil {
		return nil, err
	}
	for {
		p.skip()
		op := ""
		switch {
		case p.starts("==="):
			op = "==="
		case p.starts("!=="):
			op = "!=="
		case p.starts("=="):
			op = "=="
		case p.starts("!="):
			op = "!="
		default:
			return left, nil
		}
		p.i += len(op)
		right, err := p.parseBitAnd()
		if err != nil {
			return nil, err
		}
		left = jsBinary{op: op, left: left, right: right}
	}
}

func (p *jsParser) parseBitAnd() (jsExpr, error) {
	return p.parseBinary("&", (*jsParser).parseAdd)
}

func (p *jsParser) parseAdd() (jsExpr, error) {
	return p.parseBinary("+", (*jsParser).parseUnary)
}

func (p *jsParser) parseBinary(op string, next func(*jsParser) (jsExpr, error)) (jsExpr, error) {
	left, err := next(p)
	if err != nil {
		return nil, err
	}
	for {
		p.skip()
		if !p.starts(op) {
			return left, nil
		}
		if op == "&" && p.starts("&&") {
			return left, nil
		}
		p.i += len(op)
		right, err := next(p)
		if err != nil {
			return nil, err
		}
		left = jsBinary{op: op, left: left, right: right}
	}
}

func (p *jsParser) parseUnary() (jsExpr, error) {
	p.skip()
	if p.eof() {
		return nil, errors.Error("unexpected end of expression")
	}
	if p.s[p.i] == '!' {
		p.i++
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return jsUnary{expr: expr}, nil
	}
	return p.parsePostfix()
}

func (p *jsParser) parsePostfix() (jsExpr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		p.skip()
		if p.eof() {
			return expr, nil
		}
		switch p.s[p.i] {
		case '.':
			p.i++
			name, err := p.parseName()
			if err != nil {
				return nil, err
			}
			expr = jsMember{obj: expr, name: name}
		case '[':
			p.i++
			key, err := p.parseTernary()
			if err != nil {
				return nil, err
			}
			p.skip()
			if p.eof() || p.s[p.i] != ']' {
				return nil, errors.Errorf("expected ']' near %q", p.snippet())
			}
			p.i++
			expr = jsIndex{obj: expr, key: key}
		case '(':
			p.i++
			var args []jsExpr
			for {
				p.skip()
				if !p.eof() && p.s[p.i] == ')' {
					p.i++
					break
				}
				arg, err := p.parseTernary()
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				p.skip()
				if p.eof() {
					return nil, errors.Error("call is unclosed")
				}
				if p.s[p.i] == ',' {
					p.i++
					continue
				}
				if p.s[p.i] == ')' {
					p.i++
					break
				}
				return nil, errors.Errorf("expected ')' near %q", p.snippet())
			}
			expr = jsCall{fn: expr, args: args}
		default:
			return expr, nil
		}
	}
}

func (p *jsParser) parsePrimary() (jsExpr, error) {
	p.skip()
	if p.eof() {
		return nil, errors.Error("unexpected end of expression")
	}
	switch c := p.s[p.i]; c {
	case '\'', '"':
		s, err := p.parseString()
		if err != nil {
			return nil, err
		}
		return jsLit{v: jsVal{kind: kindStr, str: s}}, nil
	case '(':
		p.i++
		expr, err := p.parseTernary()
		if err != nil {
			return nil, err
		}
		p.skip()
		if p.eof() || p.s[p.i] != ')' {
			return nil, errors.Errorf("expected ')' near %q", p.snippet())
		}
		p.i++
		return expr, nil
	default:
		if c >= '0' && c <= '9' {
			return p.parseNumber()
		}
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		return jsIdent{name: name}, nil
	}
}

func (p *jsParser) parseNumber() (jsExpr, error) {
	start := p.i
	for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i++
	}
	n, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil {
		return nil, err
	}
	return jsLit{v: jsNum(n)}, nil
}

func (p *jsParser) parseString() (string, error) {
	quote := p.s[p.i]
	p.i++
	var b strings.Builder
	for !p.eof() {
		c := p.s[p.i]
		p.i++
		if c == '\\' {
			if p.eof() {
				return "", errors.Error("string escape is unclosed")
			}
			n := p.s[p.i]
			p.i++
			switch n {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(n)
			}
			continue
		}
		if c == quote {
			return b.String(), nil
		}
		b.WriteByte(c)
	}
	return "", errors.Error("string is unclosed")
}

func (p *jsParser) parseName() (string, error) {
	p.skip()
	if p.eof() {
		return "", errors.Error("expected name")
	}
	start := p.i
	c := p.s[p.i]
	if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || c == '$') {
		return "", errors.Errorf("expected name near %q", p.snippet())
	}
	p.i++
	for p.i < len(p.s) {
		c = p.s[p.i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '$' {
			p.i++
			continue
		}
		break
	}
	return p.s[start:p.i], nil
}

func evalJS(expr jsExpr, env *jsEnv) (jsVal, error) {
	switch e := expr.(type) {
	case jsLit:
		return e.v, nil
	case jsIdent:
		if v, ok := env.vars[e.name]; ok {
			return v, nil
		}
		switch e.name {
		case "r":
			return jsVal{kind: kindObj, obj: map[string]jsVal{
				"langtype": {kind: kindStr, str: "en"},
				"urlPath":  {kind: kindFn, str: "urlPath"},
			}}, nil
		case "e":
			return jsVal{kind: kindObj, obj: map[string]jsVal{
				"cookie": {kind: kindFn, str: "cookie"},
			}}, nil
		case "parseInt":
			return jsVal{kind: kindFn, str: "parseInt"}, nil
		case "location":
			return h3cLocation(env.base), nil
		default:
			return jsVal{}, errors.Errorf("unknown value %s", e.name)
		}
	case jsMember:
		obj, err := evalJS(e.obj, env)
		if err != nil {
			return jsVal{}, err
		}
		if obj.kind == kindStr && e.name == "length" {
			return jsNum(float64(len(obj.str))), nil
		}
		if obj.kind != kindObj {
			return jsVal{}, nil
		}
		v, ok := obj.obj[e.name]
		if !ok {
			return jsVal{}, nil
		}
		return v, nil
	case jsIndex:
		obj, err := evalJS(e.obj, env)
		if err != nil {
			return jsVal{}, err
		}
		key, err := evalJS(e.key, env)
		if err != nil {
			return jsVal{}, err
		}
		if obj.kind != kindObj {
			return jsVal{}, nil
		}
		v, ok := obj.obj[key.jsString()]
		if !ok {
			return jsVal{}, nil
		}
		return v, nil
	case jsUnary:
		v, err := evalJS(e.expr, env)
		if err != nil {
			return jsVal{}, err
		}
		if v.truthy() {
			return jsNum(0), nil
		}
		return jsNum(1), nil
	case jsCall:
		if mem, ok := e.fn.(jsMember); ok && mem.name == "toString" && len(e.args) == 0 {
			obj, err := evalJS(mem.obj, env)
			if err != nil {
				return jsVal{}, err
			}
			return jsVal{kind: kindStr, str: obj.jsString()}, nil
		}
		fn, err := evalJS(e.fn, env)
		if err != nil {
			return jsVal{}, err
		}
		if fn.kind != kindFn {
			return jsVal{}, errors.Error("call target is not a function")
		}
		args := make([]jsVal, 0, len(e.args))
		for _, arg := range e.args {
			v, err := evalJS(arg, env)
			if err != nil {
				return jsVal{}, err
			}
			args = append(args, v)
		}
		return callJS(fn.str, args, env)
	case jsBinary:
		left, err := evalJS(e.left, env)
		if err != nil {
			return jsVal{}, err
		}
		if e.op == "||" {
			if left.truthy() {
				return left, nil
			}
			return evalJS(e.right, env)
		}
		if e.op == "&&" {
			if !left.truthy() {
				return left, nil
			}
			return evalJS(e.right, env)
		}
		right, err := evalJS(e.right, env)
		if err != nil {
			return jsVal{}, err
		}
		return applyJS(e.op, left, right)
	case jsTernary:
		cond, err := evalJS(e.cond, env)
		if err != nil {
			return jsVal{}, err
		}
		if cond.truthy() {
			return evalJS(e.yes, env)
		}
		return evalJS(e.no, env)
	default:
		return jsVal{}, errors.Error("empty expression")
	}
}

func h3cLocation(base string) jsVal {
	host, hostname, protocol := base, "", ""
	if u, err := url.Parse(base); err == nil {
		host = u.Host
		hostname = u.Hostname()
		if u.Scheme != "" {
			protocol = u.Scheme + ":"
		}
	}
	return jsVal{kind: kindObj, obj: map[string]jsVal{
		"host":     {kind: kindStr, str: host},
		"hostname": {kind: kindStr, str: hostname},
		"protocol": {kind: kindStr, str: protocol},
	}}
}

func callJS(name string, args []jsVal, env *jsEnv) (jsVal, error) {
	switch name {
	case "urlPath":
		return jsVal{kind: kindStr, str: env.base}, nil
	case "cookie":
		if len(args) == 0 {
			return jsVal{}, nil
		}
		if v, ok := env.cookies[args[0].jsString()]; ok {
			return jsVal{kind: kindStr, str: v}, nil
		}
		return jsVal{}, nil
	case "parseInt":
		if len(args) == 0 || args[0].kind == kindUndef {
			return jsVal{kind: kindNum, nan: true}, nil
		}
		s := args[0].jsString()
		i := 0
		for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || (i == 0 && s[i] == '-')) {
			i++
		}
		if i == 0 {
			return jsVal{kind: kindNum, nan: true}, nil
		}
		n, err := strconv.ParseFloat(s[:i], 64)
		if err != nil {
			return jsVal{kind: kindNum, nan: true}, nil
		}
		return jsNum(n), nil
	default:
		return jsVal{}, errors.Errorf("unsupported call %s", name)
	}
}

func applyJS(op string, left, right jsVal) (jsVal, error) {
	switch op {
	case "+":
		if left.kind == kindNum && right.kind == kindNum && !left.nan && !right.nan {
			return jsNum(left.num + right.num), nil
		}
		return jsVal{kind: kindStr, str: left.jsString() + right.jsString()}, nil
	case "&":
		return jsNum(float64(toInt32(left) & toInt32(right))), nil
	case "===", "!==":
		same := jsSame(left, right)
		if op == "!==" {
			same = !same
		}
		if same {
			return jsNum(1), nil
		}
		return jsNum(0), nil
	case "==", "!=":
		same := left.jsString() == right.jsString()
		if op == "!=" {
			same = !same
		}
		if same {
			return jsNum(1), nil
		}
		return jsNum(0), nil
	default:
		return jsVal{}, errors.Errorf("unsupported operator %s", op)
	}
}

func jsSame(left, right jsVal) bool {
	if left.kind != right.kind || left.nan || right.nan {
		return false
	}
	switch left.kind {
	case kindNum:
		return left.num == right.num
	case kindStr:
		return left.str == right.str
	default:
		return false
	}
}

func toInt32(v jsVal) int32 {
	if v.kind != kindNum || v.nan {
		return 0
	}
	return int32(uint32(int64(v.num)))
}
