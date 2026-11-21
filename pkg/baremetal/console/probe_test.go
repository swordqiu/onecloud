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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	in := `{"Password":"s3cret-pass","Token":"abcdef123456","UserName":"root","note":"s3cret-pass"}`
	out := redactSecrets(in, "s3cret-pass")
	if strings.Contains(out, "s3cret-pass") || strings.Contains(out, "abcdef123456") {
		t.Fatalf("secret leaked: %s", out)
	}
	if !strings.Contains(out, `"UserName":"root"`) {
		t.Fatalf("username was redacted: %s", out)
	}
	form := "password=" + url.QueryEscape("s3cret-pass!") + "&username=root"
	formOut := redactSecrets(form, "s3cret-pass!")
	if strings.Contains(formOut, "s3cret") || strings.Contains(formOut, "%21") {
		t.Fatalf("form secret leaked: %s", formOut)
	}
	if !strings.Contains(formOut, "username=root") {
		t.Fatalf("username was redacted: %s", formOut)
	}
	jnlp := `<argument>-kvmtoken</argument><argument>abcdef1234567890</argument><argument>-kvmport</argument><argument>7578</argument>`
	jnlpOut := redactSecrets(jnlp, "")
	if strings.Contains(jnlpOut, "abcdef1234567890") || !strings.Contains(jnlpOut, "-kvmtoken") || !strings.Contains(jnlpOut, "7578") {
		t.Fatalf("jnlp redaction = %s", jnlpOut)
	}
	href := `href="/bmc/pages/jar/vconsole.jar?authParam=abc123def456"`
	hrefOut := redactSecrets(href, "")
	if strings.Contains(hrefOut, "abc123def456") || !strings.Contains(hrefOut, "authParam=") {
		t.Fatalf("authParam leaked: %s", hrefOut)
	}
}

func TestHTML5ShimIsNotConsole(t *testing.T) {
	body := []byte(`<!doctype html><title>MegaRAC SPX</title><script src="/libs/js/html5shiv.js"></script><script src="/viewer.min.js"></script>`)
	if got := html5AccessURL("https://bmc", "/viewer.html", 200, body); got != "" {
		t.Fatalf("viewer shell matched as console: %s", got)
	}
}

func TestPageHints(t *testing.T) {
	body := []byte(`<html><title>iBMC</title><form action="/UI/Rest/Login"><input name="UserName"><input type="password" name="Password"></form><a href="/bmc/pages/kvm.jnlp">kvm</a></html>`)
	hints := pageHints(body)
	for _, want := range []string{"iBMC", "/UI/Rest/Login", "UserName", "Password", "/bmc/pages/kvm.jnlp"} {
		if !strings.Contains(hints, want) {
			t.Fatalf("hints %q missing %s", hints, want)
		}
	}
}

func TestProbeLoginThenJNLP(t *testing.T) {
	const password = "s3cret-pass"
	posts := make([]string, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<html><title>HDM</title><form action="/api/session" method="post"><input name="username"><input type="password" name="password"></form><a href="/Java/jviewer.jnlp">jnlp</a></html>`)
		case r.Method == http.MethodPost && r.URL.Path == "/UI/Rest/Login":
			posts = append(posts, r.URL.Path)
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			posts = append(posts, r.URL.Path)
			var got map[string]string
			if err := json.Unmarshal(body, &got); err != nil {
				t.Errorf("login json: %v body=%s", err, body)
			}
			if got["username"] != "root" || got["password"] != password {
				t.Errorf("login body = %#v", got)
			}
			http.SetCookie(w, &http.Cookie{Name: "QSESSIONID", Value: "sess-1"})
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"CSRFToken":"csrf-token-1","ok":"success"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/Java/jviewer.jnlp":
			if c, err := r.Cookie("QSESSIONID"); err != nil || c.Value != "sess-1" {
				http.Error(w, "no cookie", http.StatusUnauthorized)
				return
			}
			if r.Header.Get("X-CSRF-Token") != "csrf-token-1" {
				http.Error(w, "no csrf", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/x-java-jnlp-file")
			io.WriteString(w, `<?xml version="1.0"?><jnlp spec="1.0"><application-desc/></jnlp>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	jnlp, connected, err := runProbeAt(context.Background(), "huawei", srv.URL, &ConsoleJNLPInput{
		Host:     "bmc",
		Username: "root",
		Password: password,
		Model:    "2288H",
	}, ibmcProbePlan(), probeJNLP)
	if err != nil {
		t.Fatalf("probe error: %v posts=%v", err, posts)
	}
	if !connected {
		t.Fatal("expected a connection")
	}
	if !looksLikeJNLP([]byte(jnlp)) {
		t.Fatalf("jnlp = %s", jnlp)
	}
	if strings.Join(posts, ",") != "/UI/Rest/Login,/api/session" {
		t.Fatalf("login posts = %v", posts)
	}
}

func TestProbeStopsAfterRejectedLogin(t *testing.T) {
	posts := make([]string, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts = append(posts, r.URL.Path)
			http.Error(w, `{"error":"Invalid credentials"}`, http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	plan := amiProbePlan()
	plan.Logins = []probeLogin{
		{Path: "/api/session", ContentType: "application/json", Build: amiLogin},
		{Path: "/UI/Rest/Login", ContentType: "application/json", Build: ibmcLogin},
	}
	_, connected, err := runProbeAt(context.Background(), "sugon", srv.URL, &ConsoleJNLPInput{
		Host:     "bmc",
		Username: "root",
		Password: "s3cret-pass",
	}, plan, probeJNLP)
	if err == nil {
		t.Fatal("expected probe error")
	}
	if !connected {
		t.Fatal("expected a connection")
	}
	if strings.Join(posts, ",") != "/api/session" {
		t.Fatalf("login posts = %v, want only the first attempt", posts)
	}
}

func TestProbeHTML5URL(t *testing.T) {
	const want = "https://bmc.example/bmc/pages/remote/kvm.html?token=abcdefghijklmnop"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/UI/Rest/Login":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"Token":"tokentokentoken"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/UI/Rest/KvmService":
			if r.Header.Get("X-Auth-Token") != "tokentokentoken" {
				http.Error(w, "no token", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"H5Url":"`+want+`"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/viewer.html":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<!doctype html><script src="/novnc/rfb.js"></script>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	got, connected, err := runProbeAt(context.Background(), "huawei", srv.URL, &ConsoleJNLPInput{
		Host:     "bmc",
		Username: "root",
		Password: "s3cret-pass",
	}, ibmcProbePlan(), probeHTML5)
	if err != nil {
		t.Fatalf("html5 probe: %v", err)
	}
	if !connected {
		t.Fatal("expected a connection")
	}
	if got != want {
		t.Fatalf("url = %s, want %s", got, want)
	}
	logged := redactURL(got, "s3cret-pass")
	if strings.Contains(logged, "abcdefghijklmnop") {
		t.Fatalf("token leaked in log url: %s", logged)
	}

	ami := amiProbePlan()
	ami.Logins = []probeLogin{
		{Path: "/api/session", ContentType: "application/json", Build: amiLogin},
		{Path: "/UI/Rest/Login", ContentType: "application/json", Build: ibmcLogin},
	}
	pageURL, _, err := runProbeAt(context.Background(), "zte", srv.URL, &ConsoleJNLPInput{
		Host:     "bmc",
		Username: "root",
		Password: "s3cret-pass",
	}, ami, probeHTML5)
	if err != nil {
		t.Fatalf("ami html5 probe: %v", err)
	}
	if pageURL != srv.URL+"/viewer.html" {
		t.Fatalf("page url = %s", pageURL)
	}
}

func TestAssetRefsAndAPIHints(t *testing.T) {
	pages := []string{
		`<base href="/UI/Static/"><script src="main.abc123.js"></script><script src="polyfills.def.js"></script>`,
		`<script data-main="index.min.js?v=3.48" src="/libs/require.js?v=3.48"></script>`,
	}
	refs := assetRefs(pages)
	got := strings.Join(refs, ",")
	for _, want := range []string{"/UI/Static/main.abc123.js", "/index.min.js?v=3.48"} {
		if !strings.Contains(got, want) {
			t.Fatalf("refs %s missing %s", got, want)
		}
	}
	if strings.Contains(got, "require.js") || strings.Contains(got, "polyfills") {
		t.Fatalf("refs include loaders: %s", got)
	}
	hints := apiPathHints([]byte(`var a="/UI/Rest/KvmService/GenerateJNLP"; var b="/api/session"; var c="/UI/Rest/Managers";`))
	if !strings.Contains(hints, "/UI/Rest/KvmService/GenerateJNLP") || !strings.Contains(hints, "/api/session") {
		t.Fatalf("hints = %s", hints)
	}
	if strings.Contains(hints, "/UI/Rest/Managers") {
		t.Fatalf("unrelated path leaked: %s", hints)
	}
}

func TestIbmcExportJNLP(t *testing.T) {
	const jnlp = `<?xml version="1.0"?><jnlp spec="1.0+"></jnlp>`
	posts := make([]string, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts = append(posts, r.URL.Path)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/redfish/v1/SessionService/Sessions":
			w.Header().Set("X-Auth-Token", "tokentokentoken")
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"Id":"1"}`)
		case r.Method == http.MethodGet && r.URL.Path == ibmcKvmExportInfo:
			if r.Header.Get("X-Auth-Token") != "tokentokentoken" {
				http.Error(w, "no token", http.StatusUnauthorized)
				return
			}
			io.WriteString(w, `{"Parameters":[{"Name":"Mode","AllowableValues":["Shared"]}]}`)
		case r.Method == http.MethodPost && r.URL.Path == ibmcKvmExport:
			var got map[string]string
			json.NewDecoder(r.Body).Decode(&got)
			if got["Mode"] != "Shared" || r.Header.Get("X-Auth-Token") != "tokentokentoken" {
				http.Error(w, "bad export", http.StatusBadRequest)
				return
			}
			io.WriteString(w, jnlp)
		default:
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `{}`)
		}
	}))
	defer srv.Close()

	got, connected, err := ibmcRedfishAt(context.Background(), "huawei", srv.URL, &ConsoleJNLPInput{
		Host:     "bmc",
		Username: "root",
		Password: "s3cret-pass",
	}, probeJNLP)
	if err != nil {
		t.Fatalf("export jnlp: %v", err)
	}
	if !connected || got != jnlp {
		t.Fatalf("connected=%v jnlp=%q", connected, got)
	}
	if strings.Join(posts, ",") != "/redfish/v1/SessionService/Sessions,"+ibmcKvmExport {
		t.Fatalf("posts = %v", posts)
	}
}
