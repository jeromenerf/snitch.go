# snitch.go

snitch.go should be a simple HTTP proxy that helps everyone to test, debug and
reverse HTTP based applications by sitting in the middle and giving users
structured informations about 

## Goals

- MUST help to reverse engineer mobile apps 
- SHOULD NOT be necessary on install on the local system
- SHOULD display results in the browser

## References 

* goproxy : https://github.com/elazarl/goproxy a good lib to build a proxy in
  golang
* fiddler2 : http://fiddler2.com is a source of inspiration, though it's a evil
  .NET application that doesn't provide a network service

## TODO

* make a real daemon
* clean / split HTML and proxy
* forward more informations to SSE
* add ID to log to retrieve them individually 
* add TTL to log entries in the index
