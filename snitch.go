// snitch.go
// A debug HTTP proxy
package main

import (
	"encoding/json"
	"fmt"
	eventsource "github.com/antage/eventsource/http"
	"github.com/bmizerany/pat"
	"github.com/elazarl/goproxy"
	"github.com/mattbaird/elastigo/api"
	"github.com/mattbaird/elastigo/core"
	"github.com/mattbaird/elastigo/indices"
	"github.com/mattbaird/elastigo/search"
	"html/template"
	"net/http"
	//"net/http/httputil" // http.Response.Body byting
	"runtime"
	"strings"
)

type Log struct {
	OriginatingIP string
	Method        string
	URL           string
	Request       http.Request
	Response      http.Response
}

var (
	LogPipe = make(chan Log, 3) //FIXME
	Pouet Log
)

func PrintLogs(w http.ResponseWriter, req *http.Request) {
	t, _ := template.ParseFiles("views/printlogs.html")
	err := t.Execute(w, GetLogs())
	if err != nil {
		fmt.Println("template execution", err)
	}
}

func GetLogs() []Log {
	var logs []Log
	var log Log
	out, _ := search.Search("logs").Type("log").Size("100").Result()
	hits := out.Hits.Hits
	for _, hit := range hits {
		json.Unmarshal(hit.Source, &log)
		logs = append(logs, log)
	}
	return logs
}

func CollectLogs(es eventsource.EventSource) {
	for {
		log := <-LogPipe
		es.SendMessage(, "", "")
		core.Index(true, "logs", "log", "", log)
		indices.Flush()
	}
}

func doTheProxyStuff() {
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		originatingip := strings.Split(r.RemoteAddr, ":")[0]
		r.Header.Set("X-Forwarded-For", originatingip)
		return r, nil
	})
	proxy.OnResponse().DoFunc(func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		originatingip := ctx.Req.Header.Get("X-Forwarded-For")
		method := ctx.Req.Method
		url := ctx.Req.URL.String()
		log := Log{OriginatingIP: originatingip, Method: method, URL: url, Request: *ctx.Req, Response: *r}
		Pouet = log
		LogPipe <- log
		return r
	})
	fmt.Println("Proxy listening on http://0.0.0.0:8080")
	http.ListenAndServe("0.0.0.0:8080", proxy)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Elasticsearch
	api.Domain = "localhost"
	

	// Proxy
	go doTheProxyStuff()

	// HTTP
	es := eventsource.New(nil)
	defer es.Close()
	go CollectLogs(es)
	http.Handle("/events", es)
	
	m := pat.New()
	m.Get("/logs", http.HandlerFunc(PrintLogs))
	//m.Get("/logs/:id", http.HandlerFunc(PrintLog))
	//m.Get("/logs/?filter=:ip", http.HandlerFunc(PrintLogsForIp))
	http.Handle("/", m)
	http.ListenAndServe("0.0.0.0:8081", nil)
}
