package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHostGuardRejectsForeignHostAndOrigin(t *testing.T) {
	a := newApp(Config{WebPort: 10005}, "/tmp/config.json")
	a.lanIP = "192.168.50.1"
	h := a.guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))

	r := httptest.NewRequest("GET", "http://192.168.50.1:10005/", nil)
	r.Host = "bad.example:10005"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("foreign Host: %d", w.Code)
	}

	r = httptest.NewRequest("POST", "http://192.168.50.1:10005/api/config", strings.NewReader("{}"))
	r.Host = "192.168.50.1:10005"
	r.Header.Set("Origin", "http://bad.example:10005")
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("foreign Origin: %d", w.Code)
	}
}

func TestFirstRunBaseConfigDoesNotRequireSMTP(t *testing.T) {
	cfg := Config{
		WANInterface: "auto",
		CheckTimes: []string{"07:00", "19:00"},
		HTTPTimeoutSeconds: 8,
		WebPort: 10005,
		ExternalIPURLs: domesticExternalIPURLs(),
	}
	if err := validateBaseConfig(cfg); err != nil {
		t.Fatalf("base config should allow Web UI first-run setup: %v", err)
	}
	if err := validateSMTPConfig(cfg.SMTP); err == nil {
		t.Fatal("empty SMTP config must not be considered configured")
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatal("full config must require completed SMTP settings")
	}
}

func TestConfigSavesDeviceIdentityWithoutReturningSavedValues(t *testing.T) {
	path := t.TempDir() + "/wanipv4-watcher.json"
	cfg := Config{
		WANInterface: "auto",
		CheckTimes: []string{"07:00", "19:00"},
		HTTPTimeoutSeconds: 8,
		WebPort: 10005,
		ExternalIPURLs: domesticExternalIPURLs(),
		SMTP: SMTPConfig{
			Host: "smtp.qq.com",
			Port: 465,
			Username: "sender@qq.com",
			Password: "test-auth-code",
			From: "sender@qq.com",
			To: []string{"receiver@qq.com"},
		},
	}
	a := newApp(cfg, path)
	a.lanIP = "192.168.50.1"
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", a.config)
	h := a.guard(mux)

	body := `{"device_name":"苏州主路由","device_note":"电信宽带 · 客厅弱电箱"}`
	req := httptest.NewRequest("POST", "http://192.168.50.1:10005/api/config", strings.NewReader(body))
	req.Host = "192.168.50.1:10005"
	req.Header.Set("Origin", "http://192.168.50.1:10005")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("save config status = %d, body=%s", resp.Code, resp.Body.String())
	}

	saved, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.DeviceName != "苏州主路由" || saved.DeviceNote != "电信宽带 · 客厅弱电箱" {
		t.Fatalf("device identity not saved: name=%q note=%q", saved.DeviceName, saved.DeviceNote)
	}
	if saved.SMTP.Password != "test-auth-code" {
		t.Fatal("saving identity fields must preserve the existing SMTP authorization code")
	}

	req = httptest.NewRequest("GET", "http://192.168.50.1:10005/api/config", nil)
	req.Host = "192.168.50.1:10005"
	resp = httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("read config status = %d", resp.Code)
	}
	var public map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &public); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"device_name", "device_note", "smtp_username", "smtp_password", "smtp_to"} {
		if _, ok := public[forbidden]; ok {
			t.Fatalf("GET /api/config must not return saved setting %q: %#v", forbidden, public)
		}
	}
	if public["device_name_configured"] != true || public["device_note_configured"] != true {
		t.Fatalf("identity-presence flags missing: %#v", public)
	}
}

func TestUpdateProxyConfigPersistsWithoutReturningCredentials(t *testing.T) {
	path:=t.TempDir()+"/wanipv4-watcher.json"
	a:=newApp(Config{WANInterface:"auto",CheckTimes:[]string{"07:00","19:00"},HTTPTimeoutSeconds:8,WebPort:10005,ExternalIPURLs:domesticExternalIPURLs()},path)
	a.lanIP="192.168.50.1"
	mux:=http.NewServeMux()
	mux.HandleFunc("/api/update-proxy",a.updateProxyConfig)
	h:=a.guard(mux)
	body:=`{"type":"socks5","host":"proxy.example.com","port":1081,"username":"proxy-user","password":"proxy-secret"}`
	req:=httptest.NewRequest("POST","http://192.168.50.1:10005/api/update-proxy",strings.NewReader(body))
	req.Host="192.168.50.1:10005"
	req.Header.Set("Origin","http://192.168.50.1:10005")
	req.Header.Set("Content-Type","application/json")
	resp:=httptest.NewRecorder()
	h.ServeHTTP(resp,req)
	if resp.Code!=http.StatusOK {t.Fatalf("save proxy status=%d body=%s",resp.Code,resp.Body.String())}
	saved,err:=loadConfig(path)
	if err!=nil{t.Fatal(err)}
	if saved.UpdateProxy.Type!="socks5"||saved.UpdateProxy.Host!="proxy.example.com"||saved.UpdateProxy.Port!=1081||saved.UpdateProxy.Username!="proxy-user"||saved.UpdateProxy.Password!="proxy-secret" {
		t.Fatalf("proxy config not persisted: %#v",saved.UpdateProxy)
	}
	req=httptest.NewRequest("GET","http://192.168.50.1:10005/api/update-proxy",nil)
	req.Host="192.168.50.1:10005"
	resp=httptest.NewRecorder()
	h.ServeHTTP(resp,req)
	if resp.Code!=http.StatusOK {t.Fatalf("get proxy status=%d",resp.Code)}
	var public map[string]any
	if err=json.Unmarshal(resp.Body.Bytes(),&public);err!=nil{t.Fatal(err)}
	for _,secret:=range []string{`"username"`,`"password"`,`proxy-user`,`proxy-secret`} {
		if strings.Contains(resp.Body.String(),secret){t.Fatalf("proxy credentials leaked in GET response: %s",resp.Body.String())}
	}
	if public["type"]!="socks5"||public["host"]!="proxy.example.com"||public["port"]!=float64(1081)||public["auth_configured"]!=true {
		t.Fatalf("proxy state missing: %#v",public)
	}
	body=`{"type":"socks5","clear_auth":true}`
	req=httptest.NewRequest("POST","http://192.168.50.1:10005/api/update-proxy",strings.NewReader(body))
	req.Host="192.168.50.1:10005"
	req.Header.Set("Origin","http://192.168.50.1:10005")
	req.Header.Set("Content-Type","application/json")
	resp=httptest.NewRecorder()
	h.ServeHTTP(resp,req)
	if resp.Code!=http.StatusOK{t.Fatalf("clear auth status=%d body=%s",resp.Code,resp.Body.String())}
	saved,err=loadConfig(path)
	if err!=nil{t.Fatal(err)}
	if saved.UpdateProxy.Host!="proxy.example.com"||saved.UpdateProxy.Port!=1081||saved.UpdateProxy.Username!=""||saved.UpdateProxy.Password!="" {
		t.Fatalf("clear auth damaged endpoint or retained credentials: %#v",saved.UpdateProxy)
	}
}
