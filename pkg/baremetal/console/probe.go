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
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/httputils"
)

// probeLogin is one BMC login request. A later login runs only when the
// previous path is missing, so a rejected password is not sent again.
type probeLogin struct {
	Path        string
	ContentType string
	Build       func(username, password string) []byte
}

type probeCall struct {
	Method string
	Path   string
	Body   []byte
}

type probePlan struct {
	Discover []string
	Logins   []probeLogin
	JNLP     []string
	JNLPCall []probeCall
	HTML5    []probeCall
	Scripts  []string
}

const (
	probeJNLP  = "jnlp"
	probeHTML5 = "html5"
)

func defaultDiscoverPaths() []string {
	return []string{
		"/",
		"/index.html",
		"/login.html",
		"/view/home",
		"/UI/Rest/Login",
		"/api/session",
		"/bmc/php/processparameter.php",
		"/page/login.html",
	}
}

func defaultJNLPPaths() []string {
	return []string{
		"/bmc/pages/kvm.jnlp",
		"/bmc/pages/remote/kvm.jnlp",
		"/UI/Rest/KvmService/GenerateJNLP",
		"/viewer.jnlp",
		"/Java/jviewer.jnlp",
		"/api/kvmjnlp",
		"/api/kvm/jnlp",
		"/page/jviewer.jnlp",
	}
}

func ibmcLogin(user, pass string) []byte {
	body, _ := json.Marshal(map[string]string{
		"UserName": user,
		"Password": pass,
		"Type":     "Local",
		"Domain":   "LocaliBMC",
	})
	return body
}

func amiLogin(user, pass string) []byte {
	body, _ := json.Marshal(map[string]string{
		"username": user,
		"password": pass,
	})
	return body
}

func kvmHandlerCalls() []probeCall {
	// openway=html5 is accepted but incomplete: Huawei returns Invalid param and xFusion
	// returns 500. A Redfish session is valid, and the KVM actions then answer 406, so
	// those calls are sent with Accept: application/json. GET KvmService lists the real actions.
	return []probeCall{
		{Method: http.MethodGet, Path: "/UI/Rest/KVMHandler?openway=html5"},
		{Method: http.MethodGet, Path: "/UI/Rest/KVMHandler?openway=html5&lang=en"},
		{Method: http.MethodGet, Path: "/redfish/v1/Managers/1/KvmService"},
		{Method: http.MethodPost, Path: "/redfish/v1/Managers/1/KvmService/Actions/KvmService.ExportKvmStartupFile", Body: []byte(`{"Mode":"Shared"}`)},
	}
}

func formLogin(user, pass string) []byte {
	values := url.Values{}
	values.Set("username", user)
	values.Set("password", pass)
	return []byte(values.Encode())
}

func zteProbePlan() probePlan {
	// Form login plus GET /api/remote_control/get/kvm/launch returns the JViewer JNLP.
	// The HTML5 viewer reads /api/kvm/token and /api/settings/media/h5viewercfg.
	return probePlan{
		Discover: []string{"/", "/index.html"},
		Logins: []probeLogin{
			{Path: "/api/session", ContentType: "application/x-www-form-urlencoded", Build: formLogin},
		},
		JNLPCall: []probeCall{
			{Method: http.MethodGet, Path: "/api/remote_control/get/kvm/launch"},
		},
		HTML5: []probeCall{
			{Method: http.MethodGet, Path: "/api/kvm/token"},
			{Method: http.MethodGet, Path: "/api/settings/media/h5viewercfg"},
		},
	}
}

func ibmcHTML5Calls() []probeCall {
	calls := kvmHandlerCalls()
	return append(calls,
		probeCall{Method: http.MethodGet, Path: "/UI/Rest/KvmService"},
		probeCall{Method: http.MethodGet, Path: "/bmc/pages/remote/kvm.html"},
		probeCall{Method: http.MethodGet, Path: "/bmc/pages/kvm.html"},
		probeCall{Method: http.MethodGet, Path: "/remoteconsole/index.html"},
		probeCall{Method: http.MethodGet, Path: "/UI/index.html"},
	)
}

func ibmcJNLPCalls() []probeCall {
	return kvmHandlerCalls()
}

func amiHTML5Calls() []probeCall {
	return []probeCall{
		{Method: http.MethodGet, Path: "/api/kvm/token"},
		{Method: http.MethodGet, Path: "/api/settings/media/remote/kvm"},
		{Method: http.MethodGet, Path: "/viewer.html"},
		{Method: http.MethodGet, Path: "/page/kvm.html"},
		{Method: http.MethodGet, Path: "/novnc/vnc.html"},
		{Method: http.MethodGet, Path: "/api/remote_control/get/kvm/launch"},
	}
}

func ibmcProbePlan() probePlan {
	return probePlan{
		Discover: defaultDiscoverPaths(),
		Logins: []probeLogin{
			{Path: "/UI/Rest/Login", ContentType: "application/json", Build: ibmcLogin},
			{Path: "/api/session", ContentType: "application/json", Build: amiLogin},
		},
		JNLP:     defaultJNLPPaths(),
		JNLPCall: ibmcJNLPCalls(),
		HTML5:    ibmcHTML5Calls(),
	}
}

func amiProbePlan() probePlan {
	// H3C HDM rejected plaintext JSON at /api/session as a bad password, and ZTE
	// answered that path with 501. Do not submit another guessed password.
	return probePlan{
		Discover: defaultDiscoverPaths(),
		JNLP:     defaultJNLPPaths(),
		HTML5:    amiHTML5Calls(),
	}
}

func runProbe(ctx context.Context, driver string, input *ConsoleJNLPInput, plan probePlan, kind string) (string, error) {
	tag := "console-" + kind
	log.Infof("%s start driver=%s host=%s model=%q sn=%q username=%q", tag, driver, input.Host, input.Model, input.SerialNumber, input.Username)
	var last error
	for _, scheme := range []string{"https", "http"} {
		base := fmt.Sprintf("%s://%s", scheme, input.Host)
		result, connected, err := runProbeAt(ctx, driver, base, input, plan, kind)
		if !connected {
			log.Infof("%s driver=%s host=%s scheme=%s unreachable err=%v", tag, driver, input.Host, scheme, err)
			last = err
			continue
		}
		if err != nil {
			log.Infof("%s done driver=%s host=%s scheme=%s result=error err=%s", tag, driver, input.Host, scheme, err)
			return "", err
		}
		log.Infof("%s done driver=%s host=%s scheme=%s result=ok bytes=%d", tag, driver, input.Host, scheme, len(result))
		return result, nil
	}
	if last == nil {
		last = errors.Error("bmc web console unreachable")
	}
	return "", errors.Wrap(last, "bmc web console unreachable")
}

func runProbeAt(ctx context.Context, driver, base string, input *ConsoleJNLPInput, plan probePlan, kind string) (string, bool, error) {
	tag := "console-" + kind
	client := newProbeClient()
	connected := false
	pages := make([]string, 0)
	for _, path := range plan.Discover {
		status, _, body, err := exchange(ctx, client, tag, driver, input.Password, base, http.MethodGet, path, nil, nil)
		if err != nil && status == 0 {
			if !connected {
				return "", false, err
			}
			continue
		}
		connected = true
		if status == http.StatusOK && len(body) > 0 {
			pages = append(pages, string(body))
		}
	}
	if !connected {
		return "", false, errors.Errorf("no response from %s", base)
	}
	missed := scanWebAssets(ctx, client, tag, driver, base, input.Password, pages, nil, nil)

	var cookies map[string]string
	var extra http.Header
	loggedIn := false
	for i, login := range plan.Logins {
		body := login.Build(input.Username, input.Password)
		hdr := http.Header{}
		hdr.Set("Content-Type", login.ContentType)
		hdr.Set("Accept", "application/json, text/html, */*")
		hdr.Set("Referer", base+"/")
		status, respHdr, respBody, err := exchange(ctx, client, tag, driver, input.Password, base, http.MethodPost, login.Path, hdr, body)
		if err != nil && status == 0 {
			return "", true, errors.Wrapf(err, "login %s", login.Path)
		}
		if missingEndpoint(status) {
			log.Infof("%s driver=%s host=%s login %d/%d %s missing status=%d, try next path", tag, driver, input.Host, i+1, len(plan.Logins), login.Path, status)
			continue
		}
		if status >= 200 && status < 400 {
			cookies, extra = sessionFromResponse(respHdr, respBody)
			log.Infof("%s driver=%s host=%s login %s status=%d session_cookies=%s session_headers=%s", tag, driver, input.Host, login.Path, status, cookieNames(cookies), headerNames(extra))
			loggedIn = true
		} else {
			log.Infof("%s driver=%s host=%s login %s status=%d, stop further login to avoid account lockout", tag, driver, input.Host, login.Path, status)
		}
		break
	}
	if !loggedIn {
		if len(plan.Logins) == 0 {
			log.Infof("%s driver=%s host=%s login skipped reason=password_format_unknown", tag, driver, input.Host)
		}
		return "", true, errors.Errorf("%s probe for %s finished without login, collect logs matching %q", tag, driver, tag)
	}
	if len(missed) > 0 {
		scanWebAssets(ctx, client, tag, driver, base, input.Password, nil, missed, sessionHeader(base, "*/*", cookies, extra))
	}
	fetchScripts(ctx, client, tag, driver, base, input.Password, plan.Scripts, sessionHeader(base, "*/*", cookies, extra))
	auth := &redfishAuth{ctx: ctx, client: client, tag: tag, driver: driver, base: base, user: input.Username, password: input.Password}

	switch kind {
	case probeHTML5:
		return probeHTML5Paths(ctx, client, tag, driver, base, input.Password, plan.HTML5, cookies, extra, auth)
	default:
		found, ok, err := probeJNLPPaths(ctx, client, tag, driver, base, input.Password, plan.JNLP, cookies, extra)
		if err == nil || !ok {
			return found, ok, err
		}
		return probeJNLPCalls(ctx, client, tag, driver, base, input.Password, plan.JNLPCall, cookies, extra, auth)
	}
}

func probeJNLPCalls(ctx context.Context, client *http.Client, tag, driver, base, password string, calls []probeCall, cookies map[string]string, extra http.Header, auth *redfishAuth) (string, bool, error) {
	for _, call := range calls {
		method := call.Method
		if method == "" {
			method = http.MethodGet
		}
		hdr := sessionHeader(base, "application/x-java-jnlp-file, application/json, text/html, */*", cookies, extra)
		applyCallHeaders(hdr, call)
		status, _, body, err := exchange(ctx, client, tag, driver, password, base, method, call.Path, hdr, call.Body)
		if err != nil && status == 0 {
			log.Infof("%s driver=%s host=%s jnlp %s transport_error=%v", tag, driver, hostOf(base), call.Path, err)
			continue
		}
		if status == http.StatusUnauthorized && strings.Contains(call.Path, "/redfish/") {
			if next, ok := auth.refresh(extra); ok {
				extra = next
				hdr = sessionHeader(base, "application/x-java-jnlp-file, application/json, text/html, */*", cookies, extra)
				applyCallHeaders(hdr, call)
				status, _, body, err = exchange(ctx, client, tag, driver, password, base, method, call.Path, hdr, call.Body)
			}
		}
		if err == nil && looksLikeJNLP(body) {
			log.Infof("%s driver=%s host=%s jnlp %s %s status=%d matched=true bytes=%d", tag, driver, hostOf(base), method, call.Path, status, len(body))
			return string(body), true, nil
		}
	}
	return "", true, errors.Errorf("%s probe for %s finished without jnlp, collect logs matching %q", tag, driver, tag)
}

func probeJNLPPaths(ctx context.Context, client *http.Client, tag, driver, base, password string, paths []string, cookies map[string]string, extra http.Header) (string, bool, error) {
	for _, path := range paths {
		hdr := sessionHeader(base, "application/x-java-jnlp-file, application/xml, text/html, */*", cookies, extra)
		status, _, body, err := exchange(ctx, client, tag, driver, password, base, http.MethodGet, path, hdr, nil)
		if err != nil && status == 0 {
			log.Infof("%s driver=%s host=%s jnlp %s transport_error=%v", tag, driver, hostOf(base), path, err)
			continue
		}
		if looksLikeJNLP(body) {
			log.Infof("%s driver=%s host=%s jnlp %s status=%d matched=true bytes=%d", tag, driver, hostOf(base), path, status, len(body))
			return string(body), true, nil
		}
	}
	return "", true, errors.Errorf("%s probe for %s finished without jnlp, collect logs matching %q", tag, driver, tag)
}

func probeHTML5Paths(ctx context.Context, client *http.Client, tag, driver, base, password string, calls []probeCall, cookies map[string]string, extra http.Header, auth *redfishAuth) (string, bool, error) {
	for _, call := range calls {
		method := call.Method
		if method == "" {
			method = http.MethodGet
		}
		hdr := sessionHeader(base, "application/json, text/html, */*", cookies, extra)
		applyCallHeaders(hdr, call)
		status, _, body, err := exchange(ctx, client, tag, driver, password, base, method, call.Path, hdr, call.Body)
		if err != nil && status == 0 {
			log.Infof("%s driver=%s host=%s html5 %s transport_error=%v", tag, driver, hostOf(base), call.Path, err)
			continue
		}
		if status == http.StatusUnauthorized && strings.Contains(call.Path, "/redfish/") {
			if next, ok := auth.refresh(extra); ok {
				extra = next
				hdr = sessionHeader(base, "application/json, text/html, */*", cookies, extra)
				applyCallHeaders(hdr, call)
				status, _, body, err = exchange(ctx, client, tag, driver, password, base, method, call.Path, hdr, call.Body)
			}
		}
		if err == nil {
			if accessURL := html5AccessURL(base, call.Path, status, body); accessURL != "" {
				log.Infof("%s driver=%s host=%s html5 %s status=%d matched=true url=%s", tag, driver, hostOf(base), call.Path, status, redactURL(accessURL, password))
				return accessURL, true, nil
			}
		}
	}
	return "", true, errors.Errorf("%s probe for %s finished without url, collect logs matching %q", tag, driver, tag)
}

type redfishAuth struct {
	ctx      context.Context
	client   *http.Client
	tag      string
	driver   string
	base     string
	user     string
	password string
	tried    bool
}

const (
	ibmcKvmService    = "/redfish/v1/Managers/1/KvmService"
	ibmcKvmExport     = "/redfish/v1/Managers/1/KvmService/Actions/KvmService.ExportKvmStartupFile"
	ibmcKvmExportInfo = "/redfish/v1/Managers/1/KvmService/ExportKvmStartupFileActionInfo"
)

func runIbmcRedfishConsole(ctx context.Context, driver string, input *ConsoleJNLPInput, kind string) (string, error) {
	tag := "console-" + kind
	log.Infof("%s start driver=%s host=%s model=%q sn=%q username=%q", tag, driver, input.Host, input.Model, input.SerialNumber, input.Username)
	var last error
	for _, scheme := range []string{"https", "http"} {
		base := fmt.Sprintf("%s://%s", scheme, input.Host)
		result, connected, err := ibmcRedfishAt(ctx, driver, base, input, kind)
		if !connected {
			log.Infof("%s driver=%s host=%s scheme=%s unreachable err=%v", tag, driver, input.Host, scheme, err)
			last = err
			continue
		}
		if err != nil {
			log.Infof("%s done driver=%s host=%s scheme=%s result=error err=%s", tag, driver, input.Host, scheme, err)
			return "", err
		}
		log.Infof("%s done driver=%s host=%s scheme=%s result=ok bytes=%d", tag, driver, input.Host, scheme, len(result))
		return result, nil
	}
	if last == nil {
		last = errors.Error("bmc web console unreachable")
	}
	return "", errors.Wrap(last, "bmc web console unreachable")
}

func ibmcRedfishAt(ctx context.Context, driver, base string, input *ConsoleJNLPInput, kind string) (string, bool, error) {
	tag := "console-" + kind
	client := newProbeClient()
	auth := &redfishAuth{ctx: ctx, client: client, tag: tag, driver: driver, base: base, user: input.Username, password: input.Password}
	extra, status, err := auth.open()
	if err != nil && status == 0 {
		return "", false, err
	}
	if extra.Get("X-Auth-Token") == "" {
		return "", true, errors.Errorf("%s redfish session for %s failed status=%d", tag, driver, status)
	}
	hdr := redfishHeader(extra.Get("X-Auth-Token"))
	exchange(ctx, client, tag, driver, input.Password, base, http.MethodGet, ibmcKvmService, hdr, nil)
	_, _, infoBody, err := exchange(ctx, client, tag, driver, input.Password, base, http.MethodGet, ibmcKvmExportInfo, hdr, nil)
	modes := exportModes(infoBody)
	if len(modes) == 0 {
		modes = []string{"Shared", "Private"}
	}
	log.Infof("%s driver=%s host=%s export modes=%s", tag, driver, input.Host, strings.Join(modes, ","))
	var last error
	for _, mode := range modes {
		body, _ := json.Marshal(map[string]string{"Mode": mode})
		postHdr := redfishHeader(extra.Get("X-Auth-Token"))
		status, _, respBody, reqErr := exchange(ctx, client, tag, driver, input.Password, base, http.MethodPost, ibmcKvmExport, postHdr, body)
		if reqErr != nil && status == 0 {
			last = reqErr
			continue
		}
		if found := exportedConsole(kind, base, status, respBody); found != "" {
			log.Infof("%s driver=%s host=%s export mode=%s status=%d matched=true bytes=%d", tag, driver, input.Host, mode, status, len(found))
			return found, true, nil
		}
		last = errors.Errorf("export mode %s status %d", mode, status)
	}
	if err != nil && last == nil {
		last = err
	}
	if last == nil {
		last = errors.Error("export returned no console")
	}
	return "", true, errors.Wrapf(last, "%s probe for %s finished without console", tag, driver)
}

func redfishHeader(token string) http.Header {
	hdr := http.Header{}
	hdr.Set("Accept", "application/json")
	hdr.Set("Content-Type", "application/json")
	hdr.Set("OData-Version", "4.0")
	if token != "" {
		hdr.Set("X-Auth-Token", token)
	}
	return hdr
}

func (a *redfishAuth) open() (http.Header, int, error) {
	if a.tried {
		return http.Header{}, 0, errors.Error("redfish session already attempted")
	}
	a.tried = true
	body, _ := json.Marshal(map[string]string{"UserName": a.user, "Password": a.password})
	hdr := redfishHeader("")
	status, respHdr, respBody, err := exchange(a.ctx, a.client, a.tag, a.driver, a.password, a.base, http.MethodPost, "/redfish/v1/SessionService/Sessions", hdr, body)
	if err != nil || status < 200 || status >= 300 {
		log.Infof("%s driver=%s host=%s redfish session status=%d, stop further redfish login", a.tag, a.driver, hostOf(a.base), status)
		return http.Header{}, status, err
	}
	_, got := sessionFromResponse(respHdr, respBody)
	if tok := respHdr.Get("X-Auth-Token"); tok != "" {
		got.Set("X-Auth-Token", tok)
	}
	log.Infof("%s driver=%s host=%s redfish session status=%d session_headers=%s", a.tag, a.driver, hostOf(a.base), status, headerNames(got))
	return got, status, nil
}

func exportModes(body []byte) []string {
	var doc interface{}
	if len(body) == 0 || json.Unmarshal(body, &doc) != nil {
		return nil
	}
	modes := make([]string, 0)
	var walk func(interface{})
	walk = func(v interface{}) {
		m, ok := v.(map[string]interface{})
		if !ok {
			return
		}
		if name, _ := m["Name"].(string); strings.EqualFold(name, "Mode") {
			if vals, ok := m["AllowableValues"].([]interface{}); ok {
				for _, item := range vals {
					if s, ok := item.(string); ok && s != "" {
						modes = append(modes, s)
					}
				}
			}
		}
		for _, child := range m {
			walk(child)
		}
	}
	walk(doc)
	if len(modes) > 4 {
		modes = modes[:4]
	}
	return modes
}

func exportedConsole(kind, base string, status int, body []byte) string {
	if status < 200 || status >= 300 {
		return ""
	}
	if kind == probeHTML5 {
		return html5AccessURL(base, ibmcKvmExport, status, body)
	}
	if looksLikeJNLP(body) {
		if text := jsonJNLP(body); text != "" {
			return text
		}
		if json.Valid(body) {
			return ""
		}
		return string(body)
	}
	return ""
}

func jsonJNLP(body []byte) string {
	var doc interface{}
	if json.Unmarshal(body, &doc) != nil {
		return ""
	}
	var found string
	var walk func(interface{})
	walk = func(v interface{}) {
		if found != "" {
			return
		}
		switch n := v.(type) {
		case string:
			if looksLikeJNLP([]byte(n)) {
				found = n
			}
		case map[string]interface{}:
			for _, child := range n {
				walk(child)
			}
		case []interface{}:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(doc)
	return found
}

func (a *redfishAuth) refresh(extra http.Header) (http.Header, bool) {
	if a == nil || a.tried {
		return extra, false
	}
	a.tried = true
	body, _ := json.Marshal(map[string]string{"UserName": a.user, "Password": a.password})
	hdr := http.Header{}
	hdr.Set("Content-Type", "application/json")
	hdr.Set("Accept", "application/json")
	status, respHdr, respBody, err := exchange(a.ctx, a.client, a.tag, a.driver, a.password, a.base, http.MethodPost, "/redfish/v1/SessionService/Sessions", hdr, body)
	if err != nil || status < 200 || status >= 300 {
		log.Infof("%s driver=%s host=%s redfish session status=%d, stop further redfish login", a.tag, a.driver, hostOf(a.base), status)
		return extra, false
	}
	_, got := sessionFromResponse(respHdr, respBody)
	if tok := respHdr.Get("X-Auth-Token"); tok != "" {
		got.Set("X-Auth-Token", tok)
	}
	merged := extra.Clone()
	for key, vals := range got {
		for _, val := range vals {
			merged.Set(key, val)
		}
	}
	log.Infof("%s driver=%s host=%s redfish session status=%d session_headers=%s", a.tag, a.driver, hostOf(a.base), status, headerNames(got))
	return merged, merged.Get("X-Auth-Token") != ""
}

func applyCallHeaders(hdr http.Header, call probeCall) {
	if len(call.Body) > 0 {
		hdr.Set("Content-Type", "application/json")
	}
	if strings.Contains(call.Path, "/redfish/") {
		hdr.Set("Accept", "application/json")
		hdr.Set("OData-Version", "4.0")
	}
}

func sessionHeader(base, accept string, cookies map[string]string, extra http.Header) http.Header {
	hdr := http.Header{}
	hdr.Set("Referer", base+"/")
	hdr.Set("Accept", accept)
	setCookieHeader(hdr, cookies)
	for k, vs := range extra {
		for _, v := range vs {
			hdr.Add(k, v)
		}
	}
	return hdr
}

func hostOf(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	return u.Host
}

var (
	reAssetRef = regexp.MustCompile(`(?i)(?:src|data-main)=["']([^"']+)["']`)
	reBaseHref = regexp.MustCompile(`(?i)<base[^>]*href=["']([^"']+)["']`)
	reAPIHint  = regexp.MustCompile(`(?i)(?:/UI/Rest/[A-Za-z0-9_./-]{1,100}|/api/[A-Za-z0-9_./-]{1,100}|/bmc/[A-Za-z0-9_./-]{1,100})`)
)

func scanWebAssets(ctx context.Context, client *http.Client, tag, driver, base, password string, pages, extraRefs []string, hdr http.Header) []string {
	refs := append(append([]string{}, extraRefs...), assetRefs(pages)...)
	limit := 2
	if len(extraRefs) > 0 && len(refs) > limit {
		limit = len(refs)
		if limit > 4 {
			limit = 4
		}
	}
	fetched := 0
	missed := make([]string, 0)
	for _, ref := range refs {
		if fetched >= limit {
			break
		}
		status, body, err := fetchQuiet(ctx, client, base, ref, hdr)
		if err != nil && status == 0 {
			log.Infof("%s driver=%s host=%s asset=%s transport_error=%v", tag, driver, hostOf(base), ref, err)
			continue
		}
		fetched++
		hints := apiPathHints(body)
		words := quotedKeywordHints(body)
		log.Infof("%s driver=%s host=%s asset=%s status=%d bytes=%d api_hints=%s keywords=%s context=%s", tag, driver, hostOf(base), ref, status, len(body), redactSecrets(hints, password), redactSecrets(words, password), sourceWindows(body, password))
		if status == http.StatusNotFound {
			missed = append(missed, ref)
			if i := strings.Index(ref, "?"); i > 0 {
				plain := ref[:i]
				status, body, err = fetchQuiet(ctx, client, base, plain, hdr)
				if err == nil {
					log.Infof("%s driver=%s host=%s asset=%s status=%d bytes=%d api_hints=%s keywords=%s context=%s", tag, driver, hostOf(base), plain, status, len(body), redactSecrets(apiPathHints(body), password), redactSecrets(quotedKeywordHints(body), password), sourceWindows(body, password))
					if status == http.StatusNotFound {
						missed = append(missed, plain)
					}
				}
			}
		}
	}
	return missed
}

func assetRefs(pages []string) []string {
	seen := map[string]bool{}
	refs := make([]string, 0)
	for _, page := range pages {
		baseHref := "/"
		if m := reBaseHref.FindStringSubmatch(page); len(m) > 1 {
			baseHref = m[1]
		}
		for _, match := range reAssetRef.FindAllStringSubmatch(page, 40) {
			ref := resolveAsset(baseHref, match[1])
			name := strings.ToLower(ref)
			if strings.Contains(name, "require.js") || strings.Contains(name, "jquery") || strings.Contains(name, "polyfill") || strings.Contains(name, "favicon") || strings.Contains(name, ".css") {
				continue
			}
			if !(strings.Contains(name, "main.") || strings.Contains(name, "index.min.js") || strings.Contains(name, "scripts.") || strings.Contains(name, "app.")) {
				continue
			}
			if seen[ref] {
				continue
			}
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	return refs
}

func resolveAsset(baseHref, ref string) string {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "/") {
		return ref
	}
	return strings.TrimRight(baseHref, "/") + "/" + strings.TrimPrefix(ref, "./")
}

func apiPathHints(body []byte) string {
	found := reAPIHint.FindAll(body, 80)
	if len(found) == 0 {
		return ""
	}
	seen := map[string]bool{}
	hints := make([]string, 0)
	for _, item := range found {
		hint := string(item)
		if seen[hint] {
			continue
		}
		lower := strings.ToLower(hint)
		if !(strings.Contains(lower, "kvm") || strings.Contains(lower, "jnlp") || strings.Contains(lower, "session") || strings.Contains(lower, "login") || strings.Contains(lower, "h5") || strings.Contains(lower, "console") || strings.Contains(lower, "viewer") || strings.Contains(lower, "encrypt") || strings.Contains(lower, "rsa") || strings.Contains(lower, "cipher") || strings.Contains(lower, "public")) {
			continue
		}
		seen[hint] = true
		hints = append(hints, hint)
		if len(hints) == 40 {
			break
		}
	}
	return strings.Join(hints, ",")
}

func fetchScripts(ctx context.Context, client *http.Client, tag, driver, base, password string, paths []string, hdr http.Header) {
	for _, path := range paths {
		status, body, err := fetchQuiet(ctx, client, base, path, hdr)
		if err != nil && status == 0 {
			log.Infof("%s driver=%s host=%s asset=%s transport_error=%v", tag, driver, hostOf(base), path, err)
			continue
		}
		log.Infof("%s driver=%s host=%s asset=%s status=%d bytes=%d api_hints=%s keywords=%s context=%s", tag, driver, hostOf(base), path, status, len(body), redactSecrets(apiPathHints(body), password), redactSecrets(quotedKeywordHints(body), password), sourceWindows(body, password))
	}
}

func sourceWindows(body []byte, password string) string {
	if len(body) == 0 {
		return ""
	}
	text := string(body)
	lower := strings.ToLower(text)
	needles := []string{"login:function(){var t=new rsakey", "jnlptemplate:function", "openh5kvm:function"}
	parts := make([]string, 0)
	for _, needle := range needles {
		i := strings.Index(lower, needle)
		if i < 0 {
			continue
		}
		begin := i - 40
		if begin < 0 {
			begin = 0
		}
		end := i + 800
		if end > len(text) {
			end = len(text)
		}
		parts = append(parts, strings.Join(strings.Fields(text[begin:end]), " "))
		if len(parts) == 4 {
			break
		}
	}
	return redactPasswordOnly(strings.Join(parts, " || "), password)
}

func redactPasswordOnly(s, password string) string {
	if password == "" {
		return s
	}
	s = strings.ReplaceAll(s, password, "<redacted-password>")
	return strings.ReplaceAll(s, url.QueryEscape(password), "<redacted-password>")
}

func fetchQuiet(ctx context.Context, client *http.Client, base, path string, hdr http.Header) (int, []byte, error) {
	resp, err := httputils.Request(client, ctx, httputils.GET, joinURL(base, path), hdr, nil, false)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	return resp.StatusCode, body, err
}

var reQuoted = regexp.MustCompile(`(?:"([^"\\]{4,80})"|'([^'\\]{4,80})')`)

func quotedKeywordHints(body []byte) string {
	seen := map[string]bool{}
	hints := make([]string, 0)
	for _, match := range reQuoted.FindAllSubmatch(body, -1) {
		text := string(match[1])
		if text == "" {
			text = string(match[2])
		}
		lower := strings.ToLower(text)
		if !(strings.Contains(lower, "kvm") || strings.Contains(lower, "jnlp") || strings.Contains(lower, "html5") || strings.Contains(lower, "openway") || strings.Contains(lower, "encrypt") || strings.Contains(lower, "rsa") || strings.Contains(lower, "cipher") || strings.Contains(lower, "publickey") || strings.Contains(lower, "public_key")) {
			continue
		}
		if seen[text] {
			continue
		}
		seen[text] = true
		hints = append(hints, text)
		if len(hints) == 40 {
			break
		}
	}
	return strings.Join(hints, " | ")
}

func newProbeClient() *http.Client {
	client := httputils.GetLegacyTLSClient(15 * time.Second)
	client.Transport = literalHeaderTransport{base: client.Transport}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}

// literalHeaderTransport sends X-CSRFTOKEN with that exact spelling. net/http
// rewrites it to X-Csrf-Token, and the ZTE BMC treats the rewritten name as a
// missing credential.
type literalHeaderTransport struct {
	base http.RoundTripper
}

func (t literalHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := req.Header.Get("X-Csrf-Token")
	tr, ok := t.base.(*http.Transport)
	if token == "" || strings.ContainsAny(token, "\r\n") || req.URL.Scheme != "https" || !ok || tr.DialContext == nil {
		return t.base.RoundTrip(req)
	}
	return roundTripCSRF(req, tr, token)
}

type closeConn struct {
	io.ReadCloser
	conn net.Conn
}

func (c closeConn) Close() error {
	err := c.ReadCloser.Close()
	c.conn.Close()
	return err
}

func roundTripCSRF(req *http.Request, tr *http.Transport, token string) (*http.Response, error) {
	addr := req.URL.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "443")
	}
	raw, err := tr.DialContext(req.Context(), "tcp", addr)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10}
	if tr.TLSClientConfig != nil {
		cfg = tr.TLSClientConfig.Clone()
	}
	if cfg.ServerName == "" {
		cfg.ServerName = req.URL.Hostname()
	}
	conn := tls.Client(raw, cfg)
	if err := conn.HandshakeContext(req.Context()); err != nil {
		raw.Close()
		return nil, err
	}
	var body []byte
	if req.Body != nil {
		body, err = io.ReadAll(req.Body)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}
	var buf bytes.Buffer
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	fmt.Fprintf(&buf, "%s %s HTTP/1.1\r\nHost: %s\r\n", req.Method, req.URL.RequestURI(), host)
	fmt.Fprintf(&buf, "X-CSRFTOKEN: %s\r\n", token)
	for key, vals := range req.Header {
		if strings.EqualFold(key, "X-Csrf-Token") || strings.EqualFold(key, "Host") || strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Accept-Encoding") {
			continue
		}
		for _, val := range vals {
			if val == "" || strings.ContainsAny(val, "\r\n") {
				continue
			}
			fmt.Fprintf(&buf, "%s: %s\r\n", key, val)
		}
	}
	fmt.Fprintf(&buf, "Accept-Encoding: identity\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", len(body))
	buf.Write(body)
	if _, err := conn.Write(buf.Bytes()); err != nil {
		conn.Close()
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	resp.Body = closeConn{ReadCloser: resp.Body, conn: conn}
	return resp, nil
}

func missingEndpoint(status int) bool {
	return status == http.StatusNotFound || status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented
}

func looksLikeJNLP(body []byte) bool {
	return strings.Contains(strings.ToLower(string(body)), "<jnlp")
}

func html5AccessURL(base, path string, status int, body []byte) string {
	if status < 200 || status >= 400 || looksLikeJNLP(body) {
		return ""
	}
	if ref := jsonHTML5URL(body); ref != "" {
		return absoluteConsoleURL(base, ref)
	}
	if looksLikeHTML5Console(body) {
		return joinURL(base, path)
	}
	return ""
}

func looksLikeHTML5Console(body []byte) bool {
	text := strings.ToLower(string(body))
	if !strings.Contains(text, "<html") && !strings.Contains(text, "<!doctype html") && !strings.Contains(text, "<script") {
		return false
	}
	for _, marker := range []string{
		"novnc",
		"h5viewer",
		"h5_kvm",
		"h5kvm",
		"html5viewer",
		"rfb.js",
		"websockify",
		"kvm_h5",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func jsonHTML5URL(body []byte) string {
	var v interface{}
	if len(body) == 0 || json.Unmarshal(body, &v) != nil {
		return ""
	}
	return findHTML5URL(v, 0)
}

func findHTML5URL(v interface{}, depth int) string {
	switch node := v.(type) {
	case map[string]interface{}:
		if depth > 3 {
			return ""
		}
		for key, val := range node {
			s, ok := val.(string)
			if ok && html5Key(key) && html5Ref(s) {
				return s
			}
		}
		for _, val := range node {
			if ref := findHTML5URL(val, depth+1); ref != "" {
				return ref
			}
		}
	case []interface{}:
		for _, item := range node {
			if ref := findHTML5URL(item, depth+1); ref != "" {
				return ref
			}
		}
	}
	return ""
}

func html5Key(key string) bool {
	lk := strings.ToLower(key)
	for _, part := range []string{"url", "uri", "href", "link", "kvm", "viewer", "console", "html5", "h5", "launch"} {
		if strings.Contains(lk, part) {
			return true
		}
	}
	return false
}

func html5Ref(s string) bool {
	ref := strings.ToLower(strings.TrimSpace(s))
	if !(strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "/")) {
		return false
	}
	for _, part := range []string{"kvm", "viewer", "console", "html5", "h5", "novnc", "remote"} {
		if strings.Contains(ref, part) {
			return true
		}
	}
	return false
}

func absoluteConsoleURL(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	if strings.HasPrefix(ref, "/") {
		return joinURL(base, ref)
	}
	return ""
}

func redactURL(raw, password string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return redactSecrets(raw, password)
	}
	if u.User != nil {
		u.User = url.User("redacted")
	}
	q := u.Query()
	for key, vals := range q {
		lk := strings.ToLower(key)
		if strings.Contains(lk, "token") || strings.Contains(lk, "session") || strings.Contains(lk, "pass") || strings.Contains(lk, "pwd") || strings.Contains(lk, "auth") || strings.Contains(lk, "csrf") || strings.Contains(lk, "cookie") {
			for i := range vals {
				vals[i] = fmt.Sprintf("redacted-len-%d", len(vals[i]))
			}
			q[key] = vals
		}
	}
	u.RawQuery = q.Encode()
	return redactSecrets(u.String(), password)
}

func exchange(ctx context.Context, client *http.Client, tag, driver, password, base, method, path string, header http.Header, body []byte) (int, http.Header, []byte, error) {
	urlStr := joinURL(base, path)
	if header == nil {
		header = http.Header{}
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	sentCookies := cookieHeaderNames(header)
	reqPreview := bodyPreview(body, password)
	resp, err := httputils.Request(client, ctx, httputils.THttpMethod(method), urlStr, header, reader, false)
	if err != nil {
		log.Infof("%s driver=%s method=%s url=%s transport_error=%s sent_cookies=%s req=%s", tag, driver, method, urlStr, err, sentCookies, reqPreview)
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	respBody, rerr := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if rerr != nil {
		log.Infof("%s driver=%s method=%s url=%s status=%d read_error=%s sent_cookies=%s req=%s", tag, driver, method, urlStr, resp.StatusCode, rerr, sentCookies, reqPreview)
		return resp.StatusCode, resp.Header, respBody, rerr
	}
	log.Infof("%s driver=%s method=%s url=%s status=%d ctype=%q location=%q cookies=%s headers=%s json_keys=%s hints=%s sent_cookies=%s req=%s body=%s",
		tag, driver, method, urlStr, resp.StatusCode,
		resp.Header.Get("Content-Type"), resp.Header.Get("Location"),
		cookieSummary(resp.Header), headerNames(resp.Header), jsonKeys(respBody), pageHints(respBody),
		sentCookies, reqPreview, bodyPreview(respBody, password))
	return resp.StatusCode, resp.Header, respBody, nil
}

func joinURL(base, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(base, "/") + path
}

func setCookieHeader(hdr http.Header, cookies map[string]string) {
	if len(cookies) == 0 {
		return
	}
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+cookies[name])
	}
	hdr.Set("Cookie", strings.Join(parts, "; "))
}

func sessionFromResponse(header http.Header, body []byte) (map[string]string, http.Header) {
	cookies := map[string]string{}
	extra := http.Header{}
	for _, c := range (&http.Response{Header: header}).Cookies() {
		if c.Name != "" {
			cookies[c.Name] = c.Value
		}
	}
	for _, key := range []string{"Token", "X-Auth-Token", "X-CSRF-Token", "X-Csrftoken", "X-CSRFTOKEN"} {
		if v := header.Get(key); v != "" {
			extra.Set(key, v)
		}
	}
	if extra.Get("X-Auth-Token") == "" && extra.Get("Token") != "" {
		extra.Set("X-Auth-Token", extra.Get("Token"))
	}
	absorbJSONSession(body, cookies, extra)
	return cookies, extra
}

func absorbJSONSession(body []byte, cookies map[string]string, extra http.Header) {
	var v interface{}
	if err := json.Unmarshal(body, &v); err != nil {
		return
	}
	walkSession(v, cookies, extra, 0)
}

func walkSession(v interface{}, cookies map[string]string, extra http.Header, depth int) {
	m, ok := v.(map[string]interface{})
	if !ok || depth > 2 {
		return
	}
	for key, val := range m {
		if s, ok := val.(string); ok {
			applySessionValue(key, s, cookies, extra)
		}
		if child, ok := val.(map[string]interface{}); ok {
			walkSession(child, cookies, extra, depth+1)
		}
	}
}

func applySessionValue(key, val string, cookies map[string]string, extra http.Header) {
	if len(val) < 8 {
		return
	}
	lk := strings.ToLower(key)
	switch {
	case strings.Contains(lk, "pass") || strings.Contains(lk, "pwd"):
		return
	case lk == "session" || lk == "sessionid" || strings.HasSuffix(lk, "sessionid"):
		if _, exists := cookies[key]; !exists {
			cookies[key] = val
		}
	case strings.Contains(lk, "csrf"):
		extra.Set("X-CSRF-Token", val)
	case lk == "token" || strings.HasSuffix(lk, "token"):
		extra.Set("Token", val)
		extra.Set("X-Auth-Token", val)
	}
}

func cookieNames(cookies map[string]string) string {
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func cookieSummary(header http.Header) string {
	if header == nil {
		return ""
	}
	parts := make([]string, 0)
	for _, c := range (&http.Response{Header: header}).Cookies() {
		parts = append(parts, fmt.Sprintf("%s(len=%d)", c.Name, len(c.Value)))
	}
	return strings.Join(parts, ",")
}

func cookieHeaderNames(header http.Header) string {
	if header == nil {
		return ""
	}
	raw := header.Get("Cookie")
	if raw == "" {
		return ""
	}
	names := make([]string, 0)
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if i := strings.Index(part, "="); i > 0 {
			names = append(names, part[:i])
		}
	}
	return strings.Join(names, ",")
}

func headerNames(header http.Header) string {
	if header == nil {
		return ""
	}
	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func jsonKeys(body []byte) string {
	var v interface{}
	if len(body) == 0 || json.Unmarshal(body, &v) != nil {
		return ""
	}
	keys := make([]string, 0)
	collectJSONKeys(v, &keys, 0)
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func collectJSONKeys(v interface{}, keys *[]string, depth int) {
	m, ok := v.(map[string]interface{})
	if !ok || depth > 2 {
		return
	}
	for key, val := range m {
		*keys = append(*keys, key)
		if child, ok := val.(map[string]interface{}); ok {
			collectJSONKeys(child, keys, depth+1)
		}
	}
}

var (
	reTitle      = regexp.MustCompile(`(?i)<title[^>]*>([^<]{0,200})</title>`)
	reFormAction = regexp.MustCompile(`(?i)<form[^>]*action=["']([^"']+)["']`)
	reInput      = regexp.MustCompile(`(?i)<input\b[^>]*>`)
	reName       = regexp.MustCompile(`(?i)\bname=["']([^"']+)["']`)
	reSrc        = regexp.MustCompile(`(?i)(?:src|href)=["']([^"']+)["']`)
)

func pageHints(body []byte) string {
	if len(body) == 0 || bytes.IndexByte(body, 0) >= 0 {
		return ""
	}
	text := string(body)
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "<html") && !strings.Contains(lower, "<form") && !strings.Contains(lower, "<title") && !strings.Contains(lower, "href=") && !strings.Contains(lower, "src=") {
		return ""
	}
	title := ""
	if m := reTitle.FindStringSubmatch(text); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	}
	forms := make([]string, 0)
	for _, action := range reFormAction.FindAllStringSubmatch(text, 10) {
		fields := make([]string, 0)
		for _, input := range reInput.FindAllString(text, 30) {
			name := reName.FindStringSubmatch(input)
			if len(name) > 1 {
				fields = append(fields, name[1])
			}
		}
		forms = append(forms, fmt.Sprintf("action=%s fields=%s", action[1], strings.Join(fields, ",")))
	}
	links := make([]string, 0)
	seen := map[string]bool{}
	for _, m := range reSrc.FindAllStringSubmatch(text, 80) {
		ref := m[1]
		lref := strings.ToLower(ref)
		if seen[ref] {
			continue
		}
		if strings.Contains(lref, "jnlp") || strings.Contains(lref, "kvm") || strings.Contains(lref, "login") || strings.Contains(lref, "session") || strings.Contains(lref, "viewer") || strings.Contains(lref, "console") || strings.Contains(lref, "ibmc") || strings.Contains(lref, "hdm") || strings.Contains(lref, "html5") || strings.Contains(lref, "novnc") || strings.Contains(lref, "websocket") || strings.Contains(lref, "websock") || strings.Contains(lref, "h5viewer") {
			seen[ref] = true
			links = append(links, ref)
		}
	}
	if title == "" && len(forms) == 0 && len(links) == 0 {
		return ""
	}
	return fmt.Sprintf("title=%q forms=%q links=%q", title, strings.Join(forms, " | "), strings.Join(links, " | "))
}

func bodyPreview(body []byte, password string) string {
	if len(body) == 0 {
		return ""
	}
	if bytes.IndexByte(body, 0) >= 0 {
		return fmt.Sprintf("<binary len=%d>", len(body))
	}
	return oneLine(redactSecrets(string(body), password))
}

func oneLine(s string) string {
	const limit = 4096
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= limit {
		return s
	}
	head := limit / 2
	tail := limit / 2
	return s[:head] + fmt.Sprintf(" ...<%d bytes>... ", len(s)) + s[len(s)-tail:]
}

var sensitiveJSONValue = regexp.MustCompile(`(?i)("(?:[^"]*(?:pass|pwd|token|session|csrf|cookie|auth|secret)[^"]*)"\s*:\s*")([^"]*)(")`)

var (
	sensitiveFormValue  = regexp.MustCompile(`(?i)(^|&)((?:[^&=]*(?:pass|pwd|token|session|csrf|cookie|auth|secret)[^&=]*)=)([^&]*)`)
	sensitiveQueryValue = regexp.MustCompile(`(?i)([?&](?:[^?&=\s"]*(?:auth|token|session|pass|pwd|csrf|cookie)[^?&=\s"]*)=)([^&"\s<]*)`)
	jnlpArgument        = regexp.MustCompile(`(?i)(<argument>)([^<]*)(</argument>)`)
)

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func redactSecrets(s, password string) string {
	if password != "" {
		s = strings.ReplaceAll(s, password, "<redacted-password>")
		s = strings.ReplaceAll(s, url.QueryEscape(password), "<redacted-password>")
	}
	s = sensitiveQueryValue.ReplaceAllStringFunc(s, func(match string) string {
		parts := sensitiveQueryValue.FindStringSubmatch(match)
		if len(parts) != 3 {
			return "<redacted>"
		}
		return parts[1] + fmt.Sprintf("<redacted len=%d>", len(parts[2]))
	})
	s = sensitiveFormValue.ReplaceAllStringFunc(s, func(match string) string {
		parts := sensitiveFormValue.FindStringSubmatch(match)
		if len(parts) != 4 {
			return "<redacted>"
		}
		if strings.HasPrefix(parts[3], "<redacted") {
			return match
		}
		return parts[1] + parts[2] + fmt.Sprintf("<redacted len=%d>", len(parts[3]))
	})
	s = jnlpArgument.ReplaceAllStringFunc(s, func(match string) string {
		parts := jnlpArgument.FindStringSubmatch(match)
		if len(parts) != 4 {
			return "<redacted>"
		}
		val := parts[2]
		if strings.HasPrefix(val, "-") || (len(val) < 8 && isDigits(val)) {
			return match
		}
		return parts[1] + fmt.Sprintf("<redacted len=%d>", len(val)) + parts[3]
	})
	return sensitiveJSONValue.ReplaceAllStringFunc(s, func(match string) string {
		parts := sensitiveJSONValue.FindStringSubmatch(match)
		if len(parts) != 4 {
			return "<redacted>"
		}
		return parts[1] + fmt.Sprintf("<redacted len=%d>", len(parts[2])) + parts[3]
	})
}
