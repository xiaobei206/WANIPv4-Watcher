package main

import (
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
	historyMu sync.Mutex
	previousStatus string
	previousDetail string
	pending []Notice
}

func newApp(cfg Config,path string)*App{return &App{cfg:cfg,configPath:path,started:time.Now(),reload:make(chan struct{},1)}}
func(a *App)getConfig()Config{a.mu.RLock();defer a.mu.RUnlock();c:=a.cfg;c.CheckTimes=append([]string(nil),c.CheckTimes...);c.SMTP.To=append([]string(nil),c.SMTP.To...);c.ExternalIPURLs=append([]string(nil),c.ExternalIPURLs...);return c}
func(a *App)setConfig(c Config){a.mu.Lock();a.cfg=c;a.mu.Unlock();select{case a.reload<-struct{}{}:default:}}
func(a *App)setLatest(o Observation,action string){a.mu.Lock();a.latest=o;a.hasLatest=true;a.lastAction=action;a.mu.Unlock()}

func(a *App)performCheck(alert bool)Observation{
	a.checkMu.Lock();defer a.checkMu.Unlock()
	cfg:=a.getConfig();o:=observe(cfg);logObservation(o);a.setLatest(o,"检测完成")
	if !alert||o.Status=="unknown"{return o}
	a.historyMu.Lock();defer a.historyMu.Unlock()
	if a.previousStatus==""{if cfg.NotifyOnStart&&shouldAlert(cfg,o){a.pending=append(a.pending,makeAlertNotice(o))}}else if o.Status!=a.previousStatus{
		if o.Status=="healthy"&&isAbnormalStatus(a.previousStatus){if cfg.NotifyOnRecovery{a.pending=append(a.pending,makeRecoveryNotice(o,a.previousStatus))}}else if isAbnormalStatus(o.Status)&&shouldAlert(cfg,o){a.pending=append(a.pending,makeAlertNotice(o))}
	}else if o.Status!="healthy"&&o.Detail!=a.previousDetail&&shouldAlert(cfg,o){a.pending=append(a.pending,makeAlertNotice(o))}
	a.previousStatus,a.previousDetail=o.Status,o.Detail
	if len(a.pending)>8{a.pending=a.pending[len(a.pending)-8:]}
	if len(a.pending)>0{n:=a.pending[0];var e error;for i:=1;i<=3;i++{e=sendEmail(cfg.SMTP,n);if e==nil{break};log.Printf("email send failed (%d/3): %v",i,e);if i<3{time.Sleep(10*time.Second)}};if e==nil{a.pending=a.pending[1:];log.Printf("email sent: %s",n.Subject)}}
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
	c:=a.getConfig();a.mu.RLock();v:=map[string]any{"version":version,"uptime":time.Since(a.started).Round(time.Second).String(),"lan_address":a.lanIP,"web_port":c.WebPort,"wan_interface":c.WANInterface,"check_times":c.CheckTimes,"has_observation":a.hasLatest,"last_action":a.lastAction}
	if a.hasLatest{v["observation"]=a.latest};a.mu.RUnlock();if n,e:=nextScheduledCheck(time.Now(),c.CheckTimes);e==nil{v["next_check"]=n.Format("2006-01-02 15:04:05")};writeJSON(w,v)
}
func(a *App)config(w http.ResponseWriter,r *http.Request){
	if r.Method==http.MethodGet{c:=a.getConfig();smtpOK:=validateSMTPConfig(c.SMTP)==nil;writeJSON(w,map[string]any{"configured":smtpOK,"smtp_configured":smtpOK});return}
	fields:=map[string]json.RawMessage{};if e:=decodeJSON(w,r,&fields);e!=nil{writeError(w,400,e.Error());return}
	allowed:=map[string]bool{"wan_interface":true,"check_times":true,"http_timeout_seconds":true,"web_port":true,"smtp_host":true,"smtp_port":true,"smtp_username":true,"smtp_password":true,"smtp_from":true,"smtp_to":true};for k:=range fields{if !allowed[k]{writeError(w,400,"unknown setting: "+k);return}}
	c:=a.getConfig()
	str:=func(k string)string{var v string;_ = json.Unmarshal(fields[k],&v);return strings.TrimSpace(v)}
	num:=func(k string,d int)int{var v int;if len(fields[k])==0||json.Unmarshal(fields[k],&v)!=nil||v==0{return d};return v}
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
	c:=a.getConfig();if e:=validateSMTPConfig(c.SMTP);e!=nil{writeError(w,400,e.Error());return};n:=Notice{Subject:"[WANIPv4-Watcher] 邮件测试",Body:fmt.Sprintf("WANIPv4-Watcher 邮件测试成功。\\n版本：%s\\n时间：%s\\n",version,time.Now().Format(time.RFC3339))};if e:=sendEmail(c.SMTP,n);e!=nil{writeError(w,502,e.Error());return};writeJSON(w,map[string]any{"ok":true,"message":"测试邮件已发送"})}
func(a *App)testCheck(w http.ResponseWriter,r *http.Request){
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	o:=a.performCheck(false);writeJSON(w,map[string]any{"ok":o.Status!="unknown","observation":o})}
func(a *App)checkNow(w http.ResponseWriter,r *http.Request){
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if e:=validateSMTPConfig(a.getConfig().SMTP);e!=nil{writeError(w,400,e.Error());return}
	o:=a.performCheck(true);writeJSON(w,map[string]any{"ok":o.Status!="unknown","observation":o})}
func(a *App)logs(w http.ResponseWriter,r *http.Request){c:=a.getConfig();b,e:=os.ReadFile(c.LogFile);if e!=nil{writeJSON(w,map[string]string{"text":"暂无日志"});return};if len(b)>24<<10{b=b[len(b)-(24<<10):]};writeJSON(w,map[string]string{"text":string(b)})}
func(a *App)update(w http.ResponseWriter,r *http.Request){
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	tag,changed,e:=performSelfUpdate();if e!=nil{writeError(w,502,e.Error());return};if !changed{writeJSON(w,map[string]any{"ok":true,"updated":false,"message":"当前已是最新版本："+tag});return};writeJSON(w,map[string]any{"ok":true,"updated":true,"message":"已更新到 "+tag+"，正在重启"});go func(){time.Sleep(1500*time.Millisecond);exe,e:=os.Executable();if e!=nil{log.Printf("restart after update: %v",e);return};if e=syscall.Exec(exe,append([]string{exe},os.Args[1:]...),os.Environ());e!=nil{log.Printf("restart after update: %v",e)}}()}

func decodeJSON(w http.ResponseWriter,r *http.Request,dst any)error{r.Body=http.MaxBytesReader(w,r.Body,16<<10);defer r.Body.Close();d:=json.NewDecoder(r.Body);d.DisallowUnknownFields();if e:=d.Decode(dst);e!=nil{return e};if d.Decode(new(any))!=io.EOF{return errors.New("extra JSON data")};return nil}
func saveWebConfig(path string,c Config)error{b,e:=json.MarshalIndent(c,"","  ");if e!=nil{return e};if e=os.MkdirAll(filepath.Dir(path),0700);e!=nil{return e};tmp:=path+".new";if e=os.WriteFile(tmp,append(b,'\n'),0600);e!=nil{return e};if e=os.Chmod(tmp,0600);e!=nil{_ = os.Remove(tmp);return e};if e=os.Rename(tmp,path);e!=nil{_ = os.Remove(tmp);return e};return nil}
func parseCommaList(s string)[]string{var a []string;for _,v:=range strings.FieldsFunc(s,func(r rune)bool{return r==','||r=='\n'||r=='\r'||r==';'}){v=strings.TrimSpace(v);if v!=""{a=append(a,v)}};return a}
func writeJSON(w http.ResponseWriter,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,n int,s string){w.WriteHeader(n);writeJSON(w,map[string]any{"ok":false,"error":s})}

func performSelfUpdate()(string,bool,error){
	if releaseAssetName==""{return "",false,errors.New("release asset mapping missing")}
	client:=&http.Client{Timeout:90*time.Second};req,_:=http.NewRequest(http.MethodGet,latestReleaseAPI,nil);req.Header.Set("User-Agent","WANIPv4-Watcher")
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

const indexHTML = "<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>WANIPv4-Watcher</title><style>\nbody{font:15px system-ui,\"Microsoft YaHei\",sans-serif;background:#f3f5f7;color:#20252b;max-width:720px;margin:auto;padding:14px}header{display:flex;justify-content:space-between;align-items:center}h1{font-size:21px}.card{background:white;border:1px solid #dce1e6;border-radius:8px;padding:14px;margin:12px 0}.grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}.item{background:#f3f5f7;padding:10px;border-radius:6px;overflow-wrap:anywhere}.item b{display:block;margin-top:4px}small,.muted{color:#68737e}label{display:block;margin:10px 0 5px;font-size:13px}input{box-sizing:border-box;width:100%;padding:9px;border:1px solid #cbd2d9;border-radius:5px;font:inherit}button{padding:9px 12px;border:1px solid #aeb8c2;border-radius:5px;background:#f6f8fa;font:inherit;cursor:pointer}.primary{background:#263746;color:white}.actions{display:flex;gap:8px;flex-wrap:wrap;margin-top:12px}.ok{color:#16834b}.bad{color:#ba3434}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#171c21;color:#e0e7ed;padding:10px;border-radius:6px;max-height:250px;overflow:auto}details summary{cursor:pointer;font-weight:bold}footer{font-size:12px;color:#78818a}@media(max-width:450px){button{flex:1}}</style></head><body>\n<header><div><h1>WANIPv4-Watcher</h1><small id=\"address\">内网管理 · TCP 10005</small></div><b id=\"state\">读取中</b></header>\n<section class=\"card\"><div class=\"grid\"><div class=\"item\"><small>WAN 接口</small><b id=\"wan\">—</b></div><div class=\"item\"><small>监测状态</small><b id=\"health\">—</b></div><div class=\"item\"><small>最近检查</small><b id=\"checked\">—</b></div><div class=\"item\"><small>下次检查</small><b id=\"next\">—</b></div><div class=\"item\"><small>WAN IPv4</small><b id=\"wanip\">—</b></div><div class=\"item\"><small>公网出口 IPv4</small><b id=\"extip\">—</b></div></div><div class=\"actions\"><button onclick=\"refresh()\">刷新状态</button><button onclick=\"logs()\">查看日志</button></div></section>\n<section class=\"card\"><details id=\"settings\"><summary>配置 <span id=\"saved\" class=\"ok\">读取中</span></summary><form id=\"form\">\n<label>WAN 接口（留空保持原值）</label><input id=\"wan_interface\" placeholder=\"默认 auto；例如 ppp0；留空保持原值\"><label>检测时间（留空保持原值）</label><input id=\"times\" placeholder=\"默认 07:00,19:00\"><label>HTTP 超时秒数（留空保持原值）</label><input id=\"timeout\" type=\"number\" min=\"2\" max=\"60\" placeholder=\"默认 8 秒\">\n<h3>邮件</h3><label>SMTP 主机（留空保持原值）</label><input id=\"host\" placeholder=\"smtp.qq.com\"><label>SMTP 端口（留空保持原值）</label><input id=\"port\" type=\"number\" placeholder=\"465\"><label>用户名（留空保持原值）</label><input id=\"user\" placeholder=\"发件邮箱地址\"><label>SMTP 授权码（留空保持原值）</label><input id=\"password\" type=\"password\" autocomplete=\"new-password\" placeholder=\"只输入新授权码，不回显旧值\"><label>发件人（留空保持原值）</label><input id=\"from\" placeholder=\"通常与 SMTP 用户名相同\"><label>收件人（留空保持原值）</label><input id=\"to\" placeholder=\"收件邮箱，可多个，用逗号分隔\">\n<div class=\"actions\"><button class=\"primary\" type=\"submit\">保存</button><button type=\"button\" onclick=\"loadConfig()\">重新载入</button></div></form></details></section>\n<section class=\"card\"><b>测试与操作</b><div class=\"actions\"><button onclick=\"act('/api/test-email')\">测试邮件</button><button onclick=\"act('/api/test-check')\">检测测试（不告警）</button><button class=\"primary\" onclick=\"act('/api/check-now')\">立即正式检测</button></div><div id=\"result\" class=\"muted\" style=\"white-space:pre-wrap;margin-top:10px\"></div></section>\n<section class=\"card\"><b>程序更新</b><p class=\"muted\">当前版本 <b id=\"version\">—</b>。下载最新架构包并校验 SHA-256。</p><button class=\"primary\" onclick=\"act('/api/update')\">检查更新 / 手动更新</button></section>\n<section class=\"card\" id=\"logcard\" hidden><b>日志</b><pre id=\"log\"></pre></section><footer>无网页登录密码 · 仅 LAN 私有地址 · 不要映射到公网</footer>\n<script>\nasync function api(p,b){let o={cache:'no-store'};if(b!==undefined){o.method='POST';o.headers={'Content-Type':'application/json'};o.body=JSON.stringify(b)}let r=await fetch(p,o),j=await r.json();if(!r.ok)throw Error(j.error||r.status);return j}\nfunction msg(s,ok=true){let e=document.getElementById('result');e.textContent=s;e.className=ok?'ok':'bad'}\nasync function refresh(){try{let s=await api('/api/status'),o=s.observation;document.getElementById('version').textContent=s.version;document.getElementById('address').textContent='内网管理 · '+s.lan_address+':'+s.web_port;document.getElementById('state').textContent='运行中';document.getElementById('state').className='ok';document.getElementById('wan').textContent=s.wan_interface;document.getElementById('next').textContent=s.next_check||'—';document.getElementById('health').textContent=o?o.status:'等待首次检测';document.getElementById('checked').textContent=o?o.checked_at:'—';document.getElementById('wanip').textContent=o?o.wan_ip||'—':'—';document.getElementById('extip').textContent=o?o.external_ip||'—':'—'}catch(e){msg(e.message,false)}}\nasync function loadConfig(){\n['wan_interface','times','timeout','host','port','user','password','from','to'].forEach(id=>{document.getElementById(id).value=''});\ntry{\nconst c=await api('/api/config');\ndocument.getElementById('saved').textContent=c.smtp_configured?' · 已保存（内容不回显）':' · 尚未配置';\ndocument.getElementById('saved').className=c.smtp_configured?'ok':'bad';\ndocument.getElementById('host').placeholder=c.smtp_configured?'留空保持已保存主机':'smtp.qq.com';\ndocument.getElementById('port').placeholder=c.smtp_configured?'留空保持已保存端口':'465';\ndocument.getElementById('user').placeholder=c.smtp_configured?'留空保持已保存用户名':'发件邮箱地址';\ndocument.getElementById('password').placeholder=c.smtp_configured?'已保存；留空不修改':'请输入 QQ 邮箱 SMTP 授权码';\ndocument.getElementById('from').placeholder=c.smtp_configured?'留空保持已保存发件人':'通常与 SMTP 用户名相同';\ndocument.getElementById('to').placeholder=c.smtp_configured?'留空保持已保存收件人':'收件邮箱，可多个，用逗号分隔';\n}catch(e){msg(e.message,false)}\n}\ndocument.getElementById('form').onsubmit=async e=>{\ne.preventDefault();\nconst ids=['wan_interface','times','timeout','host','port','user','password','from','to'];\nconst value=id=>document.getElementById(id).value.trim();\nconst b={wan_interface:value('wan_interface'),check_times:value('times'),http_timeout_seconds:value('timeout')?Number(value('timeout')):0,web_port:10005,smtp_host:value('host'),smtp_port:value('port')?Number(value('port')):0,smtp_username:value('user'),smtp_password:value('password'),smtp_from:value('from'),smtp_to:value('to')};\ntry{\nconst x=await api('/api/config',b);\nids.forEach(id=>{document.getElementById(id).value=''});\ndocument.getElementById('saved').textContent=' · 已保存';\ndocument.getElementById('saved').className='ok';\ndocument.getElementById('settings').open=false;\nmsg(x.message);refresh();\n}catch(err){msg(err.message,false)}\n}\nasync function act(p){try{msg('正在执行…');let x=await api(p,{});if(x.observation){let o=x.observation;msg('状态：'+o.status+'\\nWAN：'+o.wan_ip+'\\n出口：'+o.external_ip+'\\n说明：'+o.detail+'\\n来源：\\n'+JSON.stringify(o.provider_results,null,2),x.ok)}else{msg(x.message||'完成',x.ok!==false);if(x.updated)setTimeout(()=>location.reload(),5000)}refresh()}catch(e){msg(e.message,false)}}\nasync function logs(){let x=await api('/api/logs');logcard.hidden=false;log.textContent=x.text}\nrefresh();loadConfig();document.getElementById(\"settings\").open=false;\n</script></body></html>"
