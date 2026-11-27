package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const latestReleaseAPI = "https://api.github.com/repos/xiaobei206/WANIPv4-Watcher/releases/latest"
const latestDownloadBase = "https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download"

type App struct {
	mu sync.RWMutex
	cfg Config
	configPath string
	started time.Time
	lanIP string
	latest Observation
	hasLatest bool
	lastAction string
	reload chan struct{}
	checkMu sync.Mutex
}

func newApp(cfg Config,path string)*App{return &App{cfg:cfg,configPath:path,started:time.Now(),reload:make(chan struct{},1)}}
func(a *App)getConfig()Config{a.mu.RLock();defer a.mu.RUnlock();c:=a.cfg;c.CheckTimes=append([]string(nil),c.CheckTimes...);c.SMTP.To=append([]string(nil),c.SMTP.To...);c.ExternalIPURLs=append([]string(nil),c.ExternalIPURLs...);return c}
func(a *App)setConfig(c Config){a.mu.Lock();a.cfg=c;a.mu.Unlock();select{case a.reload<-struct{}{}:default:}}
func(a *App)setLatest(o Observation,action string){a.mu.Lock();a.latest=o;a.hasLatest=true;a.lastAction=action;a.mu.Unlock()}

func(a *App)performCheck(sendNotice bool)Observation{
	a.checkMu.Lock()
	defer a.checkMu.Unlock()
	cfg:=a.getConfig()
	o:=observe(cfg)
	logObservation(o)
	a.setLatest(o,"检测完成")
	if !sendNotice { return o }
	if err:=validateSMTPConfig(cfg.SMTP);err!=nil {
		log.Printf("检测完成但邮件未发送：SMTP 尚未配置：%v",err)
		a.setLatest(o,"检测完成，邮件未发送（SMTP 未配置）")
		return o
	}
	notice:=makeObservationNotice(cfg,o)
	var sendErr error
	for attempt:=1;attempt<=3;attempt++ {
		sendErr=sendEmail(cfg.SMTP,notice)
		if sendErr==nil { break }
		log.Printf("email send failed (%d/3): %v",attempt,sendErr)
		if attempt<3 { time.Sleep(10*time.Second) }
	}
	if sendErr!=nil {
		log.Printf("本轮检测邮件最终发送失败：%v",sendErr)
		a.setLatest(o,"检测完成，邮件发送失败")
	} else {
		log.Printf("本轮检测邮件已发送：%s",notice.Subject)
		a.setLatest(o,"检测完成，邮件已发送")
	}
	return o
}

func(a *App)serveWeb(){
	ip,iface,err:=discoverLANIPv4();if err!=nil{log.Printf("Web disabled: %v",err);return}
	cfg:=a.getConfig();addr:=net.JoinHostPort(ip.String(),strconv.Itoa(cfg.WebPort))
	ln,err:=net.Listen("tcp4",addr);if err!=nil{log.Printf("Web listen %s failed: %v",addr,err);return}
	a.mu.Lock();a.lanIP=ip.String();a.mu.Unlock()
	mux:=http.NewServeMux()
	mux.HandleFunc("/",a.index)
	mux.HandleFunc("/api/status",a.status)
	mux.HandleFunc("/api/config",a.config)
	mux.HandleFunc("/api/test-email",a.testEmail)
	mux.HandleFunc("/api/test-check",a.testCheck)
	mux.HandleFunc("/api/check-now",a.checkNow)
	mux.HandleFunc("/api/logs",a.logs)
	mux.HandleFunc("/api/update-proxy",a.updateProxyConfig)
	mux.HandleFunc("/api/test-update-proxy",a.testUpdateProxy)
	mux.HandleFunc("/api/update",a.update)
	s:=&http.Server{Handler:a.guard(mux),ReadHeaderTimeout:5*time.Second,ReadTimeout:10*time.Second,WriteTimeout:60*time.Second,IdleTimeout:30*time.Second,MaxHeaderBytes:8<<10}
	log.Printf("Web management at http://%s/ (LAN interface %s; no password; LAN only)",addr,iface)
	if e:=s.Serve(ln);e!=nil&&!errors.Is(e,http.ErrServerClosed){log.Printf("Web server stopped: %v",e)}
}

func discoverLANIPv4()(net.IP,string,error){
	names:=[]string{"br0","br-lan","br-lan0"};ifs,_:=net.Interfaces()
	for _,i:=range ifs{if strings.HasPrefix(i.Name,"br-")&&i.Name!="br-lan"{names=append(names,i.Name)}}
	for _,name:=range names{i,e:=net.InterfaceByName(name);if e!=nil||i.Flags&net.FlagUp==0{continue};as,e:=i.Addrs();if e!=nil{continue};for _,a:=range as{var ip net.IP;switch v:=a.(type){case *net.IPNet:ip=v.IP;case *net.IPAddr:ip=v.IP};ip=ip.To4();if ip!=nil&&ip.IsPrivate(){return ip,name,nil}}}
	return nil,"",errors.New("no private IPv4 found on br0/br-lan LAN bridge")
}

func(a *App)guard(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
	a.mu.RLock();lan,port:=a.lanIP,a.cfg.WebPort;a.mu.RUnlock()
	host:=r.Host;if h,_,e:=net.SplitHostPort(host);e==nil{host=h}
	if lan==""||host!=lan{http.Error(w,"LAN host only",403);return}
	if r.Method!=http.MethodGet&&r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return}
	if r.Method==http.MethodPost{u,e:=url.Parse(r.Header.Get("Origin"));expected:=net.JoinHostPort(lan,strconv.Itoa(port));if e!=nil||u.Scheme!="http"||u.Host!=expected||u.User!=nil{http.Error(w,"same-origin request required",403);return};ct:=strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"),";")[0]));if ct!="application/json"{http.Error(w,"application/json required",415);return}}
	w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("Referrer-Policy","no-referrer");w.Header().Set("X-Frame-Options","DENY");w.Header().Set("Cache-Control","no-store")
	next.ServeHTTP(w,r)
})}

func(a *App)index(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/"||r.Method!=http.MethodGet{http.NotFound(w,r);return};w.Header().Set("Content-Type","text/html; charset=utf-8");_,_=io.WriteString(w,indexHTML)}
func(a *App)status(w http.ResponseWriter,r *http.Request){
	c:=a.getConfig();a.mu.RLock();v:=map[string]any{"version":version,"uptime":time.Since(a.started).Round(time.Second).String(),"lan_address":a.lanIP,"web_port":c.WebPort,"wan_interface":c.WANInterface,"device_name":deviceTag(c),"check_times":c.CheckTimes,"has_observation":a.hasLatest,"last_action":a.lastAction}
	if a.hasLatest{v["observation"]=a.latest};a.mu.RUnlock();if n,e:=nextScheduledCheck(time.Now(),c.CheckTimes);e==nil{v["next_check"]=n.Format("2006-01-02 15:04:05")};writeJSON(w,v)
}
func(a *App)config(w http.ResponseWriter,r *http.Request){
	if r.Method==http.MethodGet{c:=a.getConfig();smtpOK:=validateSMTPConfig(c.SMTP)==nil;writeJSON(w,map[string]any{"configured":smtpOK,"smtp_configured":smtpOK,"device_name_configured":strings.TrimSpace(c.DeviceName)!="","device_note_configured":strings.TrimSpace(c.DeviceNote)!=""});return}
	fields:=map[string]json.RawMessage{};if e:=decodeJSON(w,r,&fields);e!=nil{writeError(w,400,e.Error());return}
	allowed:=map[string]bool{"device_name":true,"device_note":true,"wan_interface":true,"check_times":true,"http_timeout_seconds":true,"web_port":true,"smtp_host":true,"smtp_port":true,"smtp_username":true,"smtp_password":true,"smtp_from":true,"smtp_to":true};for k:=range fields{if !allowed[k]{writeError(w,400,"unknown setting: "+k);return}}
	c:=a.getConfig()
	str:=func(k string)string{var v string;_ = json.Unmarshal(fields[k],&v);return strings.TrimSpace(v)}
	num:=func(k string,d int)int{var v int;if len(fields[k])==0||json.Unmarshal(fields[k],&v)!=nil||v==0{return d};return v}
	if v:=str("device_name");v!=""{c.DeviceName=v}
	if v:=str("device_note");v!=""{c.DeviceNote=v}
	if v:=str("wan_interface");v!=""{c.WANInterface=v}
	if v:=str("check_times");v!=""{c.CheckTimes=parseCommaList(v)}
	c.HTTPTimeoutSeconds=num("http_timeout_seconds",c.HTTPTimeoutSeconds)
	c.WebPort=10005
	if v:=str("smtp_host");v!=""{c.SMTP.Host=v}
	c.SMTP.Port=num("smtp_port",c.SMTP.Port)
	if v:=str("smtp_username");v!=""{c.SMTP.Username=v}
	if v:=str("smtp_from");v!=""{c.SMTP.From=v}
	if v:=str("smtp_to");v!=""{c.SMTP.To=parseCommaList(v)}
	if p:=str("smtp_password");p!=""{c.SMTP.Password=p}
	c.ExternalIPURLs=domesticExternalIPURLs()
	if e:=validateConfig(c);e!=nil{writeError(w,400,e.Error());return};if e:=saveWebConfig(a.configPath,c);e!=nil{writeError(w,500,e.Error());return};a.setConfig(c);writeJSON(w,map[string]any{"ok":true,"message":"配置已保存"})
}
func(a *App)testEmail(w http.ResponseWriter,r *http.Request){
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	c:=a.getConfig();if e:=validateSMTPConfig(c.SMTP);e!=nil{writeError(w,400,e.Error());return};n:=makeTestNotice(c);if e:=sendEmail(c.SMTP,n);e!=nil{writeError(w,502,e.Error());return};writeJSON(w,map[string]any{"ok":true,"message":"测试邮件已发送"})}
func(a *App)testCheck(w http.ResponseWriter,r *http.Request){
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	o:=a.performCheck(false);writeJSON(w,map[string]any{"ok":o.Status!="unknown","observation":o})}
func(a *App)checkNow(w http.ResponseWriter,r *http.Request){
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if e:=validateSMTPConfig(a.getConfig().SMTP);e!=nil{writeError(w,400,e.Error());return}
	o:=a.performCheck(true);writeJSON(w,map[string]any{"ok":o.Status!="unknown","observation":o})}
func(a *App)logs(w http.ResponseWriter,r *http.Request){b:=memoryLogs.Snapshot();if len(b)>24<<10{b=b[len(b)-(24<<10):]};if b==""{b="暂无日志"};writeJSON(w,map[string]string{"text":b})}
func(a *App)updateProxyConfig(w http.ResponseWriter,r *http.Request){
	if r.Method==http.MethodGet {
		p:=a.getConfig().UpdateProxy
		writeJSON(w,map[string]any{"type":p.Type,"host":p.Host,"port":p.Port,"configured":p.Type!="direct"&&validateUpdateProxyConfig(p)==nil,"auth_configured":p.Username!=""&&p.Password!=""})
		return
	}
	if r.Method!=http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	fields:=map[string]json.RawMessage{}
	if e:=decodeJSON(w,r,&fields);e!=nil { writeError(w,400,e.Error());return }
	str:=func(k string)string{var v string;_ = json.Unmarshal(fields[k],&v);return strings.TrimSpace(v)}
	var port int
	if len(fields["port"])>0 { if e:=json.Unmarshal(fields["port"],&port);e!=nil {writeError(w,400,"代理端口必须是整数");return} }
	var clearAuth bool
	if len(fields["clear_auth"])>0 { if e:=json.Unmarshal(fields["clear_auth"],&clearAuth);e!=nil {writeError(w,400,"clear_auth 必须是布尔值");return} }
	c:=a.getConfig()
	switch mode:=strings.ToLower(str("type"));mode {
	case "direct":
		c.UpdateProxy=UpdateProxyConfig{Type:"direct"}
	case "http","socks5":
		c.UpdateProxy.Type=mode
		if str("host")!="" { c.UpdateProxy.Host=normalizeProxyHost(str("host")) }
		if port!=0 { c.UpdateProxy.Port=port }
		if str("username")!="" { c.UpdateProxy.Username=str("username") }
		var password string
		_ = json.Unmarshal(fields["password"],&password)
		if password!="" { c.UpdateProxy.Password=password }
		if clearAuth { c.UpdateProxy.Username="";c.UpdateProxy.Password="" }
	default:
		writeError(w,400,"代理模式必须是 direct、http 或 socks5");return
	}
	if e:=validateUpdateProxyConfig(c.UpdateProxy);e!=nil { writeError(w,400,e.Error());return }
	if e:=saveWebConfig(a.configPath,c);e!=nil { writeError(w,500,e.Error());return }
	a.setConfig(c)
	writeJSON(w,map[string]any{"ok":true,"message":"更新代理设置已保存"})
}
func(a *App)testUpdateProxy(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	message,e:=checkUpdateProxy(a.getConfig().UpdateProxy)
	if e!=nil { writeError(w,502,e.Error());return }
	writeJSON(w,map[string]any{"ok":true,"message":message})
}
func(a *App)update(w http.ResponseWriter,r *http.Request){
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	tag,changed,e:=performSelfUpdate(a.getConfig().UpdateProxy);if e!=nil{writeError(w,502,e.Error());return};if !changed{writeJSON(w,map[string]any{"ok":true,"updated":false,"message":"当前已是最新版本："+tag});return};writeJSON(w,map[string]any{"ok":true,"updated":true,"message":"已更新到 "+tag+"，正在重启"});go func(){time.Sleep(1500*time.Millisecond);exe,e:=os.Executable();if e!=nil{log.Printf("restart after update: %v",e);return};if e=syscall.Exec(exe,append([]string{exe},os.Args[1:]...),os.Environ());e!=nil{log.Printf("restart after update: %v",e)}}()}

func decodeJSON(w http.ResponseWriter,r *http.Request,dst any)error{r.Body=http.MaxBytesReader(w,r.Body,16<<10);defer r.Body.Close();d:=json.NewDecoder(r.Body);d.DisallowUnknownFields();if e:=d.Decode(dst);e!=nil{return e};if d.Decode(new(any))!=io.EOF{return errors.New("extra JSON data")};return nil}
func saveWebConfig(path string,c Config)error{b,e:=json.MarshalIndent(c,"","  ");if e!=nil{return e};if e=os.MkdirAll(filepath.Dir(path),0700);e!=nil{return e};tmp:=path+".new";if e=os.WriteFile(tmp,append(b,'\n'),0600);e!=nil{return e};if e=os.Chmod(tmp,0600);e!=nil{_ = os.Remove(tmp);return e};if e=os.Rename(tmp,path);e!=nil{_ = os.Remove(tmp);return e};return nil}
func parseCommaList(s string)[]string{var a []string;for _,v:=range strings.FieldsFunc(s,func(r rune)bool{return r==','||r=='\n'||r=='\r'||r==';'}){v=strings.TrimSpace(v);if v!=""{a=append(a,v)}};return a}
func writeJSON(w http.ResponseWriter,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,n int,s string){w.WriteHeader(n);writeJSON(w,map[string]any{"ok":false,"error":s})}

func normalizeProxyHost(host string) string {
	host = strings.TrimSpace(host)
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	return host
}

func validateUpdateProxyConfig(p UpdateProxyConfig) error {
	mode := strings.ToLower(strings.TrimSpace(p.Type))
	if mode == "" || mode == "direct" { return nil }
	if mode != "http" && mode != "socks5" {
		return errors.New("更新代理类型必须是 direct、http 或 socks5")
	}
	host := normalizeProxyHost(p.Host)
	if host == "" { return errors.New("请填写代理服务器地址") }
	if strings.ContainsAny(host, " /\\?#@\t\r\n") {
		return errors.New("代理地址只填写 IP 或域名，不要包含协议、端口或路径")
	}
	if net.ParseIP(host) == nil {
		if len(host) > 253 { return errors.New("代理域名长度无效") }
		domain := strings.TrimSuffix(host, ".")
		if domain == "" { return errors.New("代理服务器地址无效") }
		for _, label := range strings.Split(domain, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return errors.New("代理服务器地址必须是有效 IPv4、IPv6 或域名")
			}
			for _, ch := range label {
				if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-') {
					return errors.New("代理服务器地址必须是有效 IPv4、IPv6 或 ASCII 域名")
				}
			}
		}
	}
	if p.Port < 1 || p.Port > 65535 { return errors.New("代理端口必须是 1–65535 的整数") }
	if (p.Username == "") != (p.Password == "") {
		return errors.New("代理用户名和密码必须同时填写；如需取消认证，请勾选清除认证信息")
	}
	if len([]byte(p.Username)) > 255 || len([]byte(p.Password)) > 255 {
		return errors.New("代理用户名和密码均不能超过 255 字节")
	}
	if strings.ContainsAny(p.Username+p.Password, "\r\n\x00") {
		return errors.New("代理用户名或密码包含无效控制字符")
	}
	return nil
}

func newUpdateHTTPClient(proxy UpdateProxyConfig, timeout time.Duration) (*http.Client, *http.Transport, error) {
	if err := validateUpdateProxyConfig(proxy); err != nil { return nil,nil,err }
	if timeout <= 0 { timeout = 90*time.Second }
	transport := &http.Transport{
		Proxy: nil, // Never inherit HTTP_PROXY/HTTPS_PROXY implicitly.
		TLSHandshakeTimeout: 15*time.Second,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout: 30*time.Second,
	}
	mode := strings.ToLower(strings.TrimSpace(proxy.Type))
	switch mode {
	case "", "direct":
		// Explicit direct mode, even when shell proxy variables are present.
	case "http":
		u := &url.URL{Scheme:"http",Host:net.JoinHostPort(normalizeProxyHost(proxy.Host),strconv.Itoa(proxy.Port))}
		if proxy.Username!="" { u.User=url.UserPassword(proxy.Username,proxy.Password) }
		transport.Proxy=http.ProxyURL(u)
	case "socks5":
		proxyAddress:=net.JoinHostPort(normalizeProxyHost(proxy.Host),strconv.Itoa(proxy.Port))
		transport.DialContext=socks5DialContext(proxyAddress,proxy.Username,proxy.Password,timeout)
	default:
		return nil,nil,errors.New("unsupported update proxy mode")
	}
	client:=&http.Client{Transport:transport,Timeout:timeout}
	return client,transport,nil
}

// socks5DialContext implements RFC 1928 and optional RFC 1929 authentication
// using only the Go standard library. Target hostnames are sent to the proxy
// for resolution, avoiding a local DNS lookup for GitHub destinations.
func socks5DialContext(proxyAddress, username, password string, timeout time.Duration) func(context.Context,string,string)(net.Conn,error) {
	return func(ctx context.Context, network, address string)(net.Conn,error) {
		if !strings.HasPrefix(network,"tcp") { return nil,fmt.Errorf("SOCKS5 only supports TCP, got %q",network) }
		dialer:=net.Dialer{Timeout:timeout,KeepAlive:15*time.Second}
		conn,err:=dialer.DialContext(ctx,"tcp",proxyAddress)
		if err!=nil { return nil,fmt.Errorf("connect to SOCKS5 proxy: %w",err) }
		closeOnError:=true
		defer func(){if closeOnError {_=conn.Close()}}()
		deadline:=time.Now().Add(timeout)
		if d,ok:=ctx.Deadline();ok&&d.Before(deadline){deadline=d}
		if err=conn.SetDeadline(deadline);err!=nil{return nil,err}
		stopCancel:=context.AfterFunc(ctx,func(){_=conn.Close()})
		defer stopCancel()
		writeAll:=func(data []byte)error{
			n,e:=conn.Write(data);if e!=nil{return e};if n!=len(data){return io.ErrShortWrite};return nil
		}
		methods:=[]byte{0}
		if username!="" { methods=[]byte{0,2} }
		greeting:=[]byte{5,byte(len(methods))}
		greeting=append(greeting,methods...)
		if err=writeAll(greeting);err!=nil{return nil,fmt.Errorf("SOCKS5 greeting: %w",err)}
		var methodReply [2]byte
		if _,err=io.ReadFull(conn,methodReply[:]);err!=nil{return nil,fmt.Errorf("SOCKS5 method reply: %w",err)}
		if methodReply[0]!=5{return nil,errors.New("invalid SOCKS5 proxy response")}
		switch methodReply[1] {
		case 0:
		case 2:
			if username=="" { return nil,errors.New("SOCKS5 proxy requires username/password authentication") }
			auth:=[]byte{1,byte(len(username))}
			auth=append(auth,[]byte(username)...)
			auth=append(auth,byte(len(password)))
			auth=append(auth,[]byte(password)...)
			if err=writeAll(auth);err!=nil{return nil,fmt.Errorf("SOCKS5 authentication: %w",err)}
			var authReply [2]byte
			if _,err=io.ReadFull(conn,authReply[:]);err!=nil{return nil,fmt.Errorf("SOCKS5 authentication reply: %w",err)}
			if authReply[0]!=1||authReply[1]!=0{return nil,errors.New("SOCKS5 proxy authentication failed")}
		case 255:
			return nil,errors.New("SOCKS5 proxy offers no supported authentication method")
		default:
			return nil,fmt.Errorf("SOCKS5 proxy selected unsupported authentication method %d",methodReply[1])
		}
		targetHost,targetPortText,err:=net.SplitHostPort(address)
		if err!=nil{return nil,fmt.Errorf("invalid SOCKS5 target address %q: %w",address,err)}
		targetHost=normalizeProxyHost(targetHost)
		targetPort,err:=strconv.Atoi(targetPortText)
		if err!=nil||targetPort<1||targetPort>65535{return nil,errors.New("invalid SOCKS5 target port")}
		request:=[]byte{5,1,0}
		if ip:=net.ParseIP(targetHost);ip!=nil {
			if v4:=ip.To4();v4!=nil { request=append(request,1);request=append(request,v4...) } else { request=append(request,4);request=append(request,ip.To16()...) }
		} else {
			targetBytes:=[]byte(targetHost)
			if len(targetBytes)==0||len(targetBytes)>255{return nil,errors.New("SOCKS5 target domain must be 1–255 bytes")}
			request=append(request,3,byte(len(targetBytes)))
			request=append(request,targetBytes...)
		}
		request=append(request,byte(targetPort>>8),byte(targetPort))
		if err=writeAll(request);err!=nil{return nil,fmt.Errorf("SOCKS5 CONNECT request: %w",err)}
		var reply [4]byte
		if _,err=io.ReadFull(conn,reply[:]);err!=nil{return nil,fmt.Errorf("SOCKS5 CONNECT reply: %w",err)}
		if reply[0]!=5||reply[2]!=0{return nil,errors.New("invalid SOCKS5 CONNECT response")}
		if reply[1]!=0 {
			reasons:=map[byte]string{1:"general failure",2:"connection not allowed",3:"network unreachable",4:"host unreachable",5:"connection refused",6:"TTL expired",7:"command unsupported",8:"address type unsupported"}
			if reason:=reasons[reply[1]];reason!="" { return nil,fmt.Errorf("SOCKS5 CONNECT failed: %s",reason) }
			return nil,fmt.Errorf("SOCKS5 CONNECT failed with code %d",reply[1])
		}
		var boundLength int
		switch reply[3] {
		case 1: boundLength=4
		case 4: boundLength=16
		case 3:
			var length [1]byte
			if _,err=io.ReadFull(conn,length[:]);err!=nil{return nil,err}
			boundLength=int(length[0])
		default: return nil,errors.New("invalid SOCKS5 bound address type")
		}
		if _,err=io.CopyN(io.Discard,conn,int64(boundLength+2));err!=nil{return nil,fmt.Errorf("read SOCKS5 bound address: %w",err)}
		if err=ctx.Err();err!=nil{return nil,err}
		stopCancel()
		if err=ctx.Err();err!=nil{return nil,err}
		if err=conn.SetDeadline(time.Time{});err!=nil{return nil,err}
		closeOnError=false
		return conn,nil
	}
}

func checkUpdateProxy(proxy UpdateProxyConfig) (string,error) {
	client,transport,err:=newUpdateHTTPClient(proxy,30*time.Second)
	if err!=nil{return "",err}
	defer transport.CloseIdleConnections()
	req,err:=http.NewRequest(http.MethodGet,latestReleaseAPI,nil)
	if err!=nil{return "",err}
	req.Header.Set("User-Agent","WANIPv4-Watcher")
	resp,err:=client.Do(req)
	if err!=nil{return "",fmt.Errorf("连接 GitHub API 失败：%w",err)}
	defer resp.Body.Close()
	if resp.StatusCode!=http.StatusOK{return "",fmt.Errorf("GitHub API 返回 %s",resp.Status)}
	var info map[string]any
	if err=json.NewDecoder(io.LimitReader(resp.Body,256<<10)).Decode(&info);err!=nil{return "",fmt.Errorf("读取 GitHub Release 信息失败：%w",err)}
	tag,_:=info["tag_name"].(string)
	if !strings.HasPrefix(tag,"v"){return "",errors.New("GitHub 返回的 Release 版本号无效")}
	assets,_:=info["assets"].([]any)
	checksumURL:=""
	for _,raw:=range assets {
		asset,ok:=raw.(map[string]any);if !ok{continue}
		if name,_:=asset["name"].(string);name=="checksums.txt" { checksumURL,_=asset["browser_download_url"].(string);break }
	}
	if checksumURL==""{return "",errors.New("最新 Release 中没有 checksums.txt 资产")}
	parsed,err:=url.Parse(checksumURL)
	if err!=nil||parsed.Scheme!="https"||parsed.Host!="github.com"{return "",errors.New("GitHub Release 校验文件地址无效")}
	checkReq,err:=http.NewRequest(http.MethodGet,checksumURL,nil)
	if err!=nil{return "",err}
	checkReq.Header.Set("User-Agent","WANIPv4-Watcher")
	checkResp,err:=client.Do(checkReq)
	if err!=nil{return "",fmt.Errorf("连接 GitHub Release 下载站点失败：%w",err)}
	defer checkResp.Body.Close()
	if checkResp.StatusCode!=http.StatusOK{return "",fmt.Errorf("GitHub Release 下载站点返回 %s",checkResp.Status)}
	checks,err:=io.ReadAll(io.LimitReader(checkResp.Body,(128<<10)+1))
	if err!=nil{return "",fmt.Errorf("读取 Release 校验文件失败：%w",err)}
	if len(checks)==0||len(checks)>128<<10{return "",errors.New("Release 校验文件为空或超过 128 KiB")}
	found:=false
	for _,line:=range strings.Split(string(checks),"\n"){fields:=strings.Fields(line);if len(fields)>=2&&fields[1]==releaseAssetName&&len(fields[0])==64{found=true;break}}
	if !found{return "",fmt.Errorf("Release 校验文件中没有当前架构资产 %s",releaseAssetName)}
	return fmt.Sprintf("代理测试成功：GitHub API 与 Release 下载站点均可访问，最新版本 %s",tag),nil
}

func performSelfUpdate(proxy UpdateProxyConfig)(string,bool,error){
	if releaseAssetName==""{return "",false,errors.New("release asset mapping missing")}
	client,transport,e:=newUpdateHTTPClient(proxy,90*time.Second)
	if e!=nil{return "",false,e}
	defer transport.CloseIdleConnections()
	req,_:=http.NewRequest(http.MethodGet,latestReleaseAPI,nil);req.Header.Set("User-Agent","WANIPv4-Watcher")
	resp,e:=client.Do(req);if e!=nil{return "",false,e};defer resp.Body.Close();if resp.StatusCode!=200{return "",false,fmt.Errorf("GitHub latest Release: %s",resp.Status)}
	var info map[string]any;if e=json.NewDecoder(io.LimitReader(resp.Body,256<<10)).Decode(&info);e!=nil{return "",false,e};tag,_:=info["tag_name"].(string);if !strings.HasPrefix(tag,"v"){return "",false,errors.New("invalid GitHub version tag")};if strings.TrimPrefix(tag,"v")==version{return tag,false,nil}
	get:=func(u string,max int64)([]byte,error){q,_:=http.NewRequest(http.MethodGet,u,nil);q.Header.Set("User-Agent","WANIPv4-Watcher");x,e:=client.Do(q);if e!=nil{return nil,e};defer x.Body.Close();if x.StatusCode!=200{return nil,fmt.Errorf("download: %s",x.Status)};return io.ReadAll(io.LimitReader(x.Body,max+1))}
	bin,e:=get(latestDownloadBase+"/"+releaseAssetName,32<<20);if e!=nil{return "",false,e};if len(bin)==0||len(bin)>32<<20{return "",false,errors.New("binary size invalid")}
	checks,e:=get(latestDownloadBase+"/checksums.txt",128<<10);if e!=nil{return "",false,e};want:="";for _,line:=range strings.Split(string(checks),"\n"){f:=strings.Fields(line);if len(f)>=2&&f[1]==releaseAssetName{want=f[0];break}}
	sum:=sha256.Sum256(bin);if len(want)!=64||hex.EncodeToString(sum[:])!=strings.ToLower(want){return "",false,errors.New("SHA-256 mismatch; current binary kept")}
	exe,e:=os.Executable();if e!=nil{return "",false,e};exe,e=filepath.EvalSymlinks(exe);if e!=nil{return "",false,e};var st syscall.Statfs_t
	if e=syscall.Statfs(filepath.Dir(exe),&st);e==nil{free:=uint64(st.Bavail)*uint64(st.Bsize);need:=uint64(len(bin))+(2<<20);if free<need{return "",false,fmt.Errorf("JFFS free space too low: %.1f MiB needed, %.1f MiB free",float64(need)/(1<<20),float64(free)/(1<<20))}}
	tmp,old:=exe+".update-new",exe+".update-old";_ = os.Remove(tmp);_ = os.Remove(old);if e=os.WriteFile(tmp,bin,0700);e!=nil{return "",false,e};if e=os.Chmod(tmp,0700);e!=nil{_ = os.Remove(tmp);return "",false,e};if e=os.Rename(exe,old);e!=nil{_ = os.Remove(tmp);return "",false,e};if e=os.Rename(tmp,exe);e!=nil{_ = os.Rename(old,exe);_ = os.Remove(tmp);return "",false,e}
	return tag,true,nil
}
func cleanupStaleUpdateBackup(){exe,e:=os.Executable();if e!=nil{return};exe,e=filepath.EvalSymlinks(exe);if e==nil{_ = os.Remove(exe+".update-old");_ = os.Remove(exe+".update-new")}}

const indexHTML = "<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>WANIPv4-Watcher</title><style>\nbody{font:15px system-ui,\"Microsoft YaHei\",sans-serif;background:#f3f5f7;color:#20252b;max-width:720px;margin:auto;padding:14px}header{display:flex;justify-content:space-between;align-items:center}h1{font-size:21px}.card{background:white;border:1px solid #dce1e6;border-radius:8px;padding:14px;margin:12px 0}.grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}.item{background:#f3f5f7;padding:10px;border-radius:6px;overflow-wrap:anywhere}.item b{display:block;margin-top:4px}small,.muted{color:#68737e}label{display:block;margin:10px 0 5px;font-size:13px}input,select{box-sizing:border-box;width:100%;padding:9px;border:1px solid #cbd2d9;border-radius:5px;font:inherit;background:white}.proxy-grid{display:grid;grid-template-columns:minmax(0,3fr) minmax(90px,1fr);gap:8px}.checkline{display:flex;align-items:center;gap:8px}.checkline input{width:auto}button{padding:9px 12px;border:1px solid #aeb8c2;border-radius:5px;background:#f6f8fa;font:inherit;cursor:pointer}.primary{background:#263746;color:white}.actions{display:flex;gap:8px;flex-wrap:wrap;margin-top:12px}.ok{color:#16834b}.bad{color:#ba3434}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#171c21;color:#e0e7ed;padding:10px;border-radius:6px;max-height:250px;overflow:auto}details summary{cursor:pointer;font-weight:bold}footer{font-size:12px;color:#78818a}@media(max-width:450px){button{flex:1}}</style></head><body>\n<header><div><h1>WANIPv4-Watcher</h1><small id=\"address\">内网管理 · TCP 10005</small></div><b id=\"state\">读取中</b></header>\n<section class=\"card\"><div class=\"grid\"><div class=\"item\"><small>WAN 接口</small><b id=\"wan\">—</b></div><div class=\"item\"><small>监测状态</small><b id=\"health\">—</b></div><div class=\"item\"><small>最近检查</small><b id=\"checked\">—</b></div><div class=\"item\"><small>下次检查</small><b id=\"next\">—</b></div><div class=\"item\"><small>WAN IPv4</small><b id=\"wanip\">—</b></div><div class=\"item\"><small>公网出口 IPv4</small><b id=\"extip\">—</b></div></div><div class=\"actions\"><button onclick=\"refresh()\">刷新状态</button><button onclick=\"logs()\">查看日志</button></div></section>\n<section class=\"card\"><details id=\"settings\"><summary>配置 <span id=\"saved\" class=\"ok\">读取中</span></summary><form id=\"form\">\n<label>WAN 接口（留空保持原值）</label><input id=\"wan_interface\" placeholder=\"默认 auto；例如 ppp0；留空保持原值\"><label>检测时间（留空保持原值）</label><input id=\"times\" placeholder=\"默认 07:00,19:00\"><label>HTTP 超时秒数（留空保持原值）</label><input id=\"timeout\" type=\"number\" min=\"2\" max=\"60\" placeholder=\"默认 8 秒\">\n<h3>设备身份（用于区分多台路由器）</h3><label>设备昵称 / 邮件 TAG（留空保持原值）</label><input id=\"device_name\" maxlength=\"48\" placeholder=\"例如：苏州主路由\"><label>设备备注（显示在邮件正文；留空保持原值）</label><input id=\"device_note\" maxlength=\"120\" placeholder=\"例如：电信宽带 · 客厅弱电箱\"><h3>邮件</h3><label>SMTP 主机（留空保持原值）</label><input id=\"host\" placeholder=\"smtp.qq.com\"><label>SMTP 端口（留空保持原值）</label><input id=\"port\" type=\"number\" placeholder=\"465\"><label>用户名（留空保持原值）</label><input id=\"user\" placeholder=\"发件邮箱地址\"><label>SMTP 授权码（留空保持原值）</label><input id=\"password\" type=\"password\" autocomplete=\"new-password\" placeholder=\"只输入新授权码，不回显旧值\"><label>发件人（留空保持原值）</label><input id=\"from\" placeholder=\"通常与 SMTP 用户名相同\"><label>收件人（留空保持原值）</label><input id=\"to\" placeholder=\"收件邮箱，可多个，用逗号分隔\">\n<div class=\"actions\"><button class=\"primary\" type=\"submit\">保存</button><button type=\"button\" onclick=\"loadConfig()\">重新载入</button></div></form></details></section>\n<section class=\"card\"><b>测试与操作</b><div class=\"actions\"><button onclick=\"act('/api/test-email')\">测试邮件</button><button onclick=\"act('/api/test-check')\">检测测试（不告警）</button><button class=\"primary\" onclick=\"act('/api/check-now')\">立即正式检测</button></div><div id=\"result\" class=\"muted\" style=\"white-space:pre-wrap;margin-top:10px\"></div></section>\n<section class=\"card\"><b>程序更新</b><p class=\"muted\">当前版本 <b id=\"version\">—</b>。下载最新架构包并校验 SHA-256。</p><h3>更新代理</h3><label>代理模式</label><select id=\"update_proxy_type\"><option value=\"direct\">直连（不使用代理）</option><option value=\"http\">HTTP 代理</option><option value=\"socks5\">SOCKS5 代理</option></select><div class=\"proxy-grid\"><div><label>服务器地址（IP 或域名）</label><input id=\"update_proxy_host\" placeholder=\"例如 192.168.1.10 或 proxy.example.com\"></div><div><label>端口</label><input id=\"update_proxy_port\" type=\"number\" min=\"1\" max=\"65535\" placeholder=\"1089\"></div></div><label>用户名（可选）</label><input id=\"update_proxy_username\" autocomplete=\"username\" placeholder=\"留空保持已保存用户名\"><label>密码（可选）</label><input id=\"update_proxy_password\" type=\"password\" autocomplete=\"new-password\" placeholder=\"留空保持已保存密码\"><label class=\"checkline\"><input id=\"update_proxy_clear_auth\" type=\"checkbox\">清除已保存的代理认证信息</label><small id=\"proxy_saved\" class=\"muted\">读取代理设置中…</small><p class=\"muted\">代理仅用于 GitHub 版本查询、二进制和 SHA-256 文件下载。WAN 检测与邮件发送不使用此代理；代理连接失败时不会自动改为直连。</p><div class=\"actions\"><button onclick=\"saveUpdateProxy()\">保存代理设置</button><button onclick=\"testUpdateProxy()\">测试代理</button><button class=\"primary\" onclick=\"act('/api/update')\">一键检查更新 / 更新程序</button></div></section>\n<section class=\"card\" id=\"logcard\" hidden><b>日志</b><pre id=\"log\"></pre></section><footer>无网页登录密码 · 仅 LAN 私有地址 · 不要映射到公网</footer>\n<script>\nasync function api(p,b){let o={cache:'no-store'};if(b!==undefined){o.method='POST';o.headers={'Content-Type':'application/json'};o.body=JSON.stringify(b)}let r=await fetch(p,o),j=await r.json();if(!r.ok)throw Error(j.error||r.status);return j}\nfunction msg(s,ok=true){let e=document.getElementById('result');e.textContent=s;e.className=ok?'ok':'bad'}\nasync function refresh(){try{let s=await api('/api/status'),o=s.observation;document.getElementById('version').textContent=s.version;document.getElementById('address').textContent='设备：'+(s.device_name||'未命名设备')+' · 内网管理 · '+s.lan_address+':'+s.web_port;document.getElementById('state').textContent='运行中';document.getElementById('state').className='ok';document.getElementById('wan').textContent=s.wan_interface;document.getElementById('next').textContent=s.next_check||'—';document.getElementById('health').textContent=o?o.status:'等待首次检测';document.getElementById('checked').textContent=o?o.checked_at:'—';document.getElementById('wanip').textContent=o?o.wan_ip||'—':'—';document.getElementById('extip').textContent=o?o.external_ip||'—':'—'}catch(e){msg(e.message,false)}}\nasync function loadConfig(){\n['device_name','device_note','wan_interface','times','timeout','host','port','user','password','from','to'].forEach(id=>{document.getElementById(id).value=''});\ntry{\nconst c=await api('/api/config');\ndocument.getElementById('saved').textContent=c.smtp_configured?' · 已保存（内容不回显）':' · 尚未配置';\ndocument.getElementById('saved').className=c.smtp_configured?'ok':'bad';\ndocument.getElementById('device_name').placeholder=c.device_name_configured?'已保存；留空保持原昵称':'例如：苏州主路由';\ndocument.getElementById('device_note').placeholder=c.device_note_configured?'已保存；留空保持原备注':'例如：电信宽带 · 客厅弱电箱';\ndocument.getElementById('host').placeholder=c.smtp_configured?'留空保持已保存主机':'smtp.qq.com';\ndocument.getElementById('port').placeholder=c.smtp_configured?'留空保持已保存端口':'465';\ndocument.getElementById('user').placeholder=c.smtp_configured?'留空保持已保存用户名':'发件邮箱地址';\ndocument.getElementById('password').placeholder=c.smtp_configured?'已保存；留空不修改':'请输入 QQ 邮箱 SMTP 授权码';\ndocument.getElementById('from').placeholder=c.smtp_configured?'留空保持已保存发件人':'通常与 SMTP 用户名相同';\ndocument.getElementById('to').placeholder=c.smtp_configured?'留空保持已保存收件人':'收件邮箱，可多个，用逗号分隔';\n}catch(e){msg(e.message,false)}\n}\ndocument.getElementById('form').onsubmit=async e=>{\ne.preventDefault();\nconst ids=['device_name','device_note','wan_interface','times','timeout','host','port','user','password','from','to'];\nconst value=id=>document.getElementById(id).value.trim();\nconst b={device_name:value('device_name'),device_note:value('device_note'),wan_interface:value('wan_interface'),check_times:value('times'),http_timeout_seconds:value('timeout')?Number(value('timeout')):0,web_port:10005,smtp_host:value('host'),smtp_port:value('port')?Number(value('port')):0,smtp_username:value('user'),smtp_password:value('password'),smtp_from:value('from'),smtp_to:value('to')};\ntry{\nconst x=await api('/api/config',b);\nids.forEach(id=>{document.getElementById(id).value=''});\ndocument.getElementById('saved').textContent=' · 已保存';\ndocument.getElementById('saved').className='ok';\ndocument.getElementById('settings').open=false;\nmsg(x.message);refresh();\n}catch(err){msg(err.message,false)}\n}\nasync function act(p){try{msg('正在执行…');let x=await api(p,{});if(x.observation){let o=x.observation;msg('状态：'+o.status+'\\nWAN：'+o.wan_ip+'\\n出口：'+o.external_ip+'\\n说明：'+o.detail+'\\n来源：\\n'+JSON.stringify(o.provider_results,null,2),x.ok)}else{msg(x.message||'完成',x.ok!==false);if(x.updated)setTimeout(()=>location.reload(),5000)}refresh()}catch(e){msg(e.message,false)}}\nasync function loadUpdateProxy(){try{const p=await api('/api/update-proxy');document.getElementById('update_proxy_type').value=p.type||'direct';document.getElementById('update_proxy_host').value=p.host||'';document.getElementById('update_proxy_port').value=p.port||'';document.getElementById('update_proxy_username').value='';document.getElementById('update_proxy_password').value='';document.getElementById('update_proxy_username').placeholder=p.auth_configured?'已保存；留空保持原用户名':'可选；留空表示不修改';document.getElementById('update_proxy_password').placeholder=p.auth_configured?'已保存；留空保持原密码':'可选；留空表示不修改';document.getElementById('proxy_saved').textContent=p.type==='direct'?'当前使用直连':(p.configured?'代理设置已保存'+(p.auth_configured?'（已启用认证）':''):'代理设置尚未完成');document.getElementById('proxy_saved').className=p.configured||p.type==='direct'?'ok':'bad'}catch(e){document.getElementById('proxy_saved').textContent=e.message;document.getElementById('proxy_saved').className='bad'}}
async function saveUpdateProxy(){const type=document.getElementById('update_proxy_type').value;const host=document.getElementById('update_proxy_host').value.trim();const portText=document.getElementById('update_proxy_port').value.trim();const username=document.getElementById('update_proxy_username').value.trim();const password=document.getElementById('update_proxy_password').value;const clearAuth=document.getElementById('update_proxy_clear_auth').checked;try{msg('正在保存更新代理…');const x=await api('/api/update-proxy',{type,host,port:portText?Number(portText):0,username,password,clear_auth:clearAuth});document.getElementById('update_proxy_username').value='';document.getElementById('update_proxy_password').value='';document.getElementById('update_proxy_clear_auth').checked=false;await loadUpdateProxy();msg(x.message,true)}catch(e){msg(e.message,false)}}
async function testUpdateProxy(){try{msg('正在测试 GitHub API 和 Release 下载站点…');const x=await api('/api/test-update-proxy',{});msg(x.message,true)}catch(e){msg(e.message,false)}}
async function logs(){let x=await api('/api/logs');logcard.hidden=false;log.textContent=x.text}\nrefresh();loadConfig();loadUpdateProxy();document.getElementById(\"settings\").open=false;\n</script></body></html>"
