// af83proxy.go
// A debug HTTP proxy
// Jérôme Andrieux, 2013
package main

import (
	"fmt"
	eventsource "github.com/antage/eventsource/http"
	"github.com/bmizerany/pat"
	"github.com/elazarl/goproxy"
	//"github.com/vmihailenco/redis"
	"html/template"
	"net/http"
	"net/http/httputil"
	"runtime"
	"strings"
	"time"
)

const (
	printlogs_tpl = `
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Logs</title>
    <script src="https://ajax.googleapis.com/ajax/libs/jquery/1.7.2/jquery.min.js"></script>
    <script type="text/javascript">
      $(function () {
        var evsrc = new EventSource("/events");
        evsrc.onmessage = function (ev) {
			console.log(ev);
          var data = ev.data ;
          $("#log > ul").append("<li>" + data + "</li>");
        }
        evsrc.onerror = function (ev) {
          console.log("readyState = " + ev.currentTarget.readyState)
        }
      })
    </script>
</head>
<body>
  <h1>Logs</h1>
    <div id="log">
      <ul>
      </ul>
    </div>
  <dl>
    {{range .}}
    <dt>{{.OriginatingIP}} : {{.Method}} {{.URL}} at {{.Timestamp}}</dt>
	<dd>{{.Body}}</dd>
    {{end}}
  </ol>
</body>
</html>
`
)

type Log struct {
	// FIXME I should store the whole http.Request and http.Response
	Timestamp     int64
	OriginatingIP string
	Method        string
	URL           string
	Body          string
}

var (
	Logs      []Log
	LogPipe   = make(chan Log, 3)
	EventPipe = make(chan Log, 3)
)

func PrintLogs(w http.ResponseWriter, req *http.Request) {
	t, _ := template.New("printlogs").Parse(printlogs_tpl)
	t.ExecuteTemplate(w, "printlogs", Logs)
}

func CollectLogs() {
	for {
		log := <-LogPipe
		Logs = append(Logs, log)
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// HTTP EventSource
	Es := eventsource.New(nil)
	defer Es.Close()
	http.Handle("/events", Es)

	go func() {
		for {
			log := <-EventPipe
			Es.SendMessage(log.URL, "add", "")
			fmt.Println(log.OriginatingIP)
		}
	}()

	// HTTP interface
	m := pat.New()
	m.Get("/logs", http.HandlerFunc(PrintLogs))
	http.Handle("/", m)
	go http.ListenAndServe("0.0.0.0:8081", nil)
	go CollectLogs()

	// Proxy
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		originatingip := strings.Split(r.RemoteAddr, ":")[0]
		r.Header.Set("X-Forwarded-For", originatingip)
		fmt.Println("Request received")
		return r, nil
	})
	proxy.OnResponse().DoFunc(func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		originatingip := ctx.Req.Header.Get("X-Forwarded-For")
		method := ctx.Req.Method
		url := ctx.Req.URL.String()
		bodybytes, _ := httputil.DumpResponse(r, true)
		body := string(bodybytes)
		log := Log{OriginatingIP: originatingip, Method: method, URL: url, Body: body, Timestamp: time.Now().Unix()}
		LogPipe <- log
		EventPipe <- log
		return r
	})
	fmt.Println("Proxy listening on http://0.0.0.0:8080")
	http.ListenAndServe("0.0.0.0:8080", proxy)
}
