package main
import("net/http";"net/http/httptest";"strings";"testing")
func TestHostGuardRejectsForeignHostAndOrigin(t *testing.T){
 a:=newApp(Config{WebPort:10005},"/tmp/config.json");a.lanIP="192.168.50.1"
 h:=a.guard(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(204)}))
 r:=httptest.NewRequest("GET","http://192.168.50.1:10005/",nil);r.Host="bad.example:10005";w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=403{t.Fatalf("foreign Host: %d",w.Code)}
 r=httptest.NewRequest("POST","http://192.168.50.1:10005/api/config",strings.NewReader("{}"));r.Host="192.168.50.1:10005";r.Header.Set("Origin","http://bad.example:10005");r.Header.Set("Content-Type","application/json");w=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=403{t.Fatalf("foreign Origin: %d",w.Code)}
}