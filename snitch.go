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
	Id            string
	OriginatingIP string
	Method        string
	URL           string
	Request       http.Request
	Response      http.Response
}

var (
	LogPipe = make(chan Log, 3) //FIXME, why do I need a buffer ?
)

// PrintLogs writes the list of log lines GetLogs() returns
func PrintLogs(w http.ResponseWriter, req *http.Request) {
	t, err := template.ParseFiles("views/printlogs.html")
	if err != nil {
		fmt.Println("Error: ", err)
		panic(err)
	}
	err = t.Execute(w, GetLogs())
	if err != nil {
		fmt.Println("Error: ", err)
		panic(err)
	}
}

// PrintLog writes the whole request and response for a given ES ":logid"
func PrintLog(w http.ResponseWriter, req *http.Request) {
	t, err := template.ParseFiles("views/printlog.html")
	if err != nil {
		fmt.Println("Error: ", err)
		panic(err)
	}
	logid := req.URL.Query().Get(":logid")
	err = t.Execute(w, GetLog(logid))
	if err != nil {
		fmt.Println("Error: ", err)
		panic(err)
	}
}

// GetLogs returns an array of log lines []Log from ES, limited to the last 100
func GetLogs() []Log {
	var logs []Log
	var log Log
	out, err := search.Search("logs").Type("log").Size("100").Result()
	if err != nil {
		fmt.Println("Error: ", err)
		panic(err)
	}
	hits := out.Hits.Hits
	for _, hit := range hits {
		json.Unmarshal(hit.Source, &log)
		log.Id = hit.Id
		logs = append(logs, log)
	}
	return logs
}

// GetLog returns a whole Log line corresponding to ES :logid
func GetLog(logid string) Log {
	var log Log
	out, err := core.SearchUri("logs", "log", fmt.Sprintf("_id:%s", logid), "")
	if err != nil {
		fmt.Println("Error: ", err)
		panic(err)
	}
	hits := out.Hits.Hits
	for _, hit := range hits {
		json.Unmarshal(hit.Source, &log)
		log.Id = hit.Id
	}
	return log
}

func DispatchLogs(es eventsource.EventSource) {
	for {
		log := <-LogPipe
		resp, err := core.Index(true, "logs", "log", "", log)
		if err != nil {
			fmt.Println("EINDEXING: ", err)
		} else {
			es.SendMessage(log.URL, "", resp.Id) // I should add a type
			indices.Flush()
		}
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
	go DispatchLogs(es)
	http.Handle("/events", es)

	m := pat.New()
	m.Get("/logs", http.HandlerFunc(PrintLogs))
	m.Get("/logs/:logid", http.HandlerFunc(PrintLog))
	//m.Get("/logs/?filter=:ip", http.HandlerFunc(PrintLogsForIp))
	http.Handle("/", m)
	http.ListenAndServe("0.0.0.0:8081", nil)
}
