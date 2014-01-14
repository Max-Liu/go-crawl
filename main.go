package main

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
	"github.com/davecgh/go-spew/spew"

	// "syscall"
	// "io"
	"os"
	"sync"
)

var wg sync.WaitGroup
var linkList []link
var host, full_host_url string
var file *os.File
var finish bool
var throttle <-chan time.Time

var ignoredFileExtention = []string{".css", ".js", ".png", ".jpg", ".ico"}
var badLinkRetryTimes = 5
var requestTimeOut = 5 * time.Second
var target = "http://www.geekpark.net"
var maxConcurrenceQresuet = 1000

type link struct {
	url          string
	status_code  int
	duration     time.Duration
	error_count  int
	hasRequested int
	title        string
}

func main() {
	link_list_chan := make(chan []link)

	throttle = time.Tick(time.Duration(1 * time.Second))

	runtime.GOMAXPROCS(runtime.NumCPU())

	prepare()
	getPageUrls(target)
	// os.Exit(1)
	for finish == false {
		crawl(link_list_chan)
	}
	log.Println("finish!~")
	defer file.Close()
}

func prepare() {
	init_file()
	set_host()
}

func init_file() {
	file, _ = os.OpenFile("log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
}

func set_host() {
	last_dot_index := strings.LastIndex(target, ".")
	url_without_last_dot := target[:last_dot_index]
	last_sec_dot_index := strings.Index(url_without_last_dot, ".")
	host = target[last_sec_dot_index+1:]
	if host[len(host)-1:] == "/" {
		host = host[:len(host)-1]
	}

	full_host_url = target[:strings.Index(target, host)] + host
}

func crawl(link_list_chan chan []link) {
	for {
		for k, v := range linkList {
			if runtime.NumGoroutine() < maxConcurrenceQresuet {
				go request(&linkList[k])
				go getPageUrls(v.url)
			} else {
				log.Println("Max requset,sleep in 2 second")
				time.Sleep(2 * time.Second)
				continue
			}
		}
		if finish == true {
			break
		}
		checkAllRequested()
		<-throttle
	}
}

//check the all list has requested
func checkAllRequested() {
	var requestCount int
	for _, v := range linkList {
		if v.hasRequested == 1 {
			requestCount++
		}
	}

	if requestCount == len(linkList) {
		finish = true
	}
}

func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, time.Duration(requestTimeOut))
}

func request(link *link) {
	if link.hasRequested == 1 {
		return
	}
	//bad request retry
	var breakCounter int
	var t0, t1 time.Time
	transport := http.Transport{
		Dial: dialTimeout,
	}

	client := http.Client{
		Transport: &transport,
	}

	for {
		t0 = time.Now()
		resp, err := client.Get(link.url)
		t1 = time.Now()

		if err != nil {
			breakCounter++
			if breakCounter == badLinkRetryTimes {
				file.WriteString(link.url + " " + strconv.Itoa(link.status_code) + "\n")
				break
			}
		} else {
			link.status_code = resp.StatusCode
			if link.status_code != 200 {
				file.WriteString(link.url + " " + strconv.Itoa(link.status_code) + "\n")
			}
			spew.Dump(link.url)
			get_title(link, resp)
			break
		}
	}
	link.duration = t1.Sub(t0)
	link.error_count = breakCounter
	link.hasRequested = 1
}

func get_title(link *link, resp *http.Response) {
	content, _ := ioutil.ReadAll(resp.Body)
	contentStr := string(content)

	startIndex := strings.Index(contentStr, "<title>")
	endIndex := strings.Index(contentStr, "</title>")

	if startIndex != -1 && endIndex != -1 {
		link.title = contentStr[startIndex+7 : endIndex]
	}
}

func getPageUrls(url string) {
	res, err := http.Get(url)
	if err != nil {
		file.WriteString(err.Error())
		return
	}

	content, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		file.WriteString(err.Error())
		return
	}

	contentStr := string(content)

lookingForLink:
	for {
		startIndex := strings.Index(contentStr, `href="`)
		//check if it has looked entire page for href=
		if startIndex == -1 {
			break
		}

		newStr := contentStr[startIndex+6:]
		newStrEndIndex := strings.Index(newStr, `"`)

		if newStrEndIndex <= 6 {
			contentStr = newStr[2:]
			continue lookingForLink
		}
		linkStr := newStr[:newStrEndIndex]

		//check if linkSts is relative path.if so,change to absolute path.
		if string(linkStr[0]) == "/" {
			linkStr = full_host_url + linkStr
		}

		//check the links in pages blog to targe domain
		//check http/https://xxx.host.com(len(xxx) = 20),
		if index := strings.Index(linkStr, host); index == -1 || index > 20 {
			contentStr = newStr[newStrEndIndex:]
			continue lookingForLink
		}
		for _, v := range ignoredFileExtention {
			if linkStr[len(linkStr)-4:len(linkStr)] == v {
				contentStr = newStr[newStrEndIndex:]
				continue lookingForLink
			}
		}
		// spew.Dump(linkStr)

		linkNew := &link{linkStr, 0, 0, 0, 0, ""}
		linkList = appendIfMissing(linkList, linkNew)
		contentStr = newStr[newStrEndIndex:]
	}
}

func appendIfMissing(list []link, new_link *link) []link {
	for _, v := range list {
		if v.url == new_link.url {
			return list
		}
	}
	return append(list, *new_link)
}
