package main

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestValidateUpdateProxyConfig(t *testing.T) {
	cases:=[]struct{name string;proxy UpdateProxyConfig;wantErr bool}{
		{"direct",UpdateProxyConfig{Type:"direct"},false},
		{"http ipv4",UpdateProxyConfig{Type:"http",Host:"192.168.1.2",Port:1089},false},
		{"socks ipv6",UpdateProxyConfig{Type:"socks5",Host:"2001:db8::1",Port:1081},false},
		{"domain",UpdateProxyConfig{Type:"http",Host:"proxy.example.com",Port:8080},false},
		{"missing host",UpdateProxyConfig{Type:"http",Port:8080},true},
		{"bad port",UpdateProxyConfig{Type:"socks5",Host:"127.0.0.1",Port:65536},true},
		{"bad scheme in host",UpdateProxyConfig{Type:"http",Host:"http://proxy.example.com",Port:8080},true},
		{"half credentials",UpdateProxyConfig{Type:"socks5",Host:"127.0.0.1",Port:1080,Username:"user"},true},
		{"unsupported mode",UpdateProxyConfig{Type:"socks4",Host:"127.0.0.1",Port:1080},true},
	}
	for _,tc:=range cases {
		t.Run(tc.name,func(t *testing.T) {
			err:=validateUpdateProxyConfig(tc.proxy)
			if (err!=nil)!=tc.wantErr {t.Fatalf("validate error=%v wantErr=%v",err,tc.wantErr)}
		})
	}
}

func TestHTTPUpdateProxyUsesAuthentication(t *testing.T) {
	const username,password="proxy-user","proxy-pass"
	wantAuth:="Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	proxy:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
		if got:=r.Header.Get("Proxy-Authorization");got!=wantAuth {http.Error(w,"missing proxy authentication",http.StatusProxyAuthRequired);return}
		if r.URL.Host!="example.invalid" {http.Error(w,"unexpected proxy target",http.StatusBadRequest);return}
		w.WriteHeader(http.StatusOK)
		_,_=io.WriteString(w,"ok")
	}))
	defer proxy.Close()
	u,err:=url.Parse(proxy.URL)
	if err!=nil {t.Fatal(err)}
	host,portText,err:=net.SplitHostPort(u.Host)
	if err!=nil {t.Fatal(err)}
	port,err:=strconv.Atoi(portText)
	if err!=nil {t.Fatal(err)}
	client,transport,err:=newUpdateHTTPClient(UpdateProxyConfig{Type:"http",Host:host,Port:port,Username:username,Password:password},5*time.Second)
	if err!=nil {t.Fatal(err)}
	defer transport.CloseIdleConnections()
	resp,err:=client.Get("http://example.invalid/check")
	if err!=nil {t.Fatal(err)}
	defer resp.Body.Close()
	if resp.StatusCode!=http.StatusOK {t.Fatalf("proxy status=%s",resp.Status)}
}

func TestSOCKS5DialerUsesRemoteDNSAndUsernamePassword(t *testing.T) {
	listener,err:=net.Listen("tcp","127.0.0.1:0")
	if err!=nil {t.Fatal(err)}
	defer listener.Close()
	result:=make(chan error,1)
	go func() {
		conn,e:=listener.Accept()
		if e!=nil {result<-e;return}
		defer conn.Close()
		var greeting [2]byte
		if _,e=io.ReadFull(conn,greeting[:]);e!=nil {result<-e;return}
		if greeting[0]!=5||greeting[1]!=2 {result<-io.ErrUnexpectedEOF;return}
		methods:=make([]byte,int(greeting[1]))
		if _,e=io.ReadFull(conn,methods);e!=nil {result<-e;return}
		if len(methods)!=2||methods[0]!=0||methods[1]!=2 {result<-io.ErrUnexpectedEOF;return}
		if _,e=conn.Write([]byte{5,2});e!=nil {result<-e;return}
		var authHead [2]byte
		if _,e=io.ReadFull(conn,authHead[:]);e!=nil {result<-e;return}
		user:=make([]byte,int(authHead[1]))
		if _,e=io.ReadFull(conn,user);e!=nil {result<-e;return}
		var passLen [1]byte
		if _,e=io.ReadFull(conn,passLen[:]);e!=nil {result<-e;return}
		pass:=make([]byte,int(passLen[0]))
		if _,e=io.ReadFull(conn,pass);e!=nil {result<-e;return}
		if authHead[0]!=1||string(user)!="alice"||string(pass)!="secret" {result<-io.ErrUnexpectedEOF;return}
		if _,e=conn.Write([]byte{1,0});e!=nil {result<-e;return}
		var requestHead [4]byte
		if _,e=io.ReadFull(conn,requestHead[:]);e!=nil {result<-e;return}
		if requestHead[0]!=5||requestHead[1]!=1||requestHead[3]!=3 {result<-io.ErrUnexpectedEOF;return}
		var domainLen [1]byte
		if _,e=io.ReadFull(conn,domainLen[:]);e!=nil {result<-e;return}
		domain:=make([]byte,int(domainLen[0]))
		if _,e=io.ReadFull(conn,domain);e!=nil {result<-e;return}
		var portBytes [2]byte
		if _,e=io.ReadFull(conn,portBytes[:]);e!=nil {result<-e;return}
		port:=int(portBytes[0])<<8|int(portBytes[1])
		if string(domain)!="github.com"||port!=443 {result<-io.ErrUnexpectedEOF;return}
		_,e=conn.Write([]byte{5,0,0,1,0,0,0,0,0,0})
		result<-e
	}()
	host,portText,err:=net.SplitHostPort(listener.Addr().String())
	if err!=nil {t.Fatal(err)}
	port,err:=strconv.Atoi(portText)
	if err!=nil {t.Fatal(err)}
	dial:=socks5DialContext(net.JoinHostPort(host,strconv.Itoa(port)),"alice","secret",5*time.Second)
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
	defer cancel()
	conn,err:=dial(ctx,"tcp","github.com:443")
	if err!=nil {t.Fatal(err)}
	_ = conn.Close()
	if err=<-result;err!=nil {t.Fatal(err)}
}

func TestDirectUpdateClientDoesNotEnableEnvironmentProxy(t *testing.T) {
	_,transport,err:=newUpdateHTTPClient(UpdateProxyConfig{Type:"direct"},time.Second)
	if err!=nil {t.Fatal(err)}
	defer transport.CloseIdleConnections()
	if transport.Proxy!=nil||transport.DialContext!=nil {
		t.Fatalf("direct mode must not inherit environment proxy")
	}
}
