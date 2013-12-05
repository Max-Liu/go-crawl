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
	"math/rand"
	// "syscall"
	// "io"
	// spew "github.com/davecgh/go-spew/spew"
	"os"
	"sync"
)

var wg sync.WaitGroup
var link_list []link
var host, full_host_url string
var ok, no int
var target string
var file *os.File
var finish bool
var throttle <-chan time.Time

type link struct {
	url           string
	status_code   int
	duration      time.Duration
	error_count   int
	has_requested int
	title string
}

func main() {
	link_list_chan := make(chan []link, 1000)


	throttle = time.Tick(time.Duration(30 * time.Second))

	runtime.GOMAXPROCS(runtime.NumCPU())
	target = "http://www.geekpark.net"
	prepare()
	get_page_urls(target)

	for finish == false {
		crawl(link_list_chan)
	}
	log.Println("finish!~")
	defer file.Close()
}

func prepare() {
	init_file()
	set_host(target)
}

func init_file() {
	file, _ = os.OpenFile("log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
}

func set_host(target string) {
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
		for k, v := range link_list {
			go request(&link_list[k])
			go get_page_urls(v.url)
			go func() {
				log.Println(runtime.NumGoroutine())
			}()
		}
		go check_pending()
		if finish == true {
			break
		}
		<-throttle
	}
}

func check_pending() {
	var request_count int
	for _, v := range link_list {
		if v.has_requested == 1 {
			request_count++
		}
	}
	if request_count == len(link_list) {
		finish = true
	}
}

func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, time.Duration(15*time.Second))
}

func request(link *link) {
	if link.has_requested == 1 {
		return
	}

	var break_counter int
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
			log.Println(err)
			link.status_code = 0
			time.Sleep(time.Duration(rand.Int63n(10000)) * time.Millisecond)
			break_counter++
			if break_counter == 50 {
				no++
				break
			}
		} else {
			link.status_code = resp.StatusCode
			get_title(link,resp)
			file.WriteString(link.url + " " + t1.Sub(t0).String() + " " + strconv.Itoa(link.status_code) + " ok:" + strconv.Itoa(ok) + " no:" + strconv.Itoa(no) +link.title+"\n")
			ok++
			break
		}
	}


	link.duration = t1.Sub(t0)
	link.error_count = break_counter
	link.has_requested = 1
}

func get_title(link *link,resp *http.Response) {
	content, _ := ioutil.ReadAll(resp.Body)
	content_str := string(content)

	start_index :=strings.Index(content_str, "<title>")
	end_index := strings.Index(content_str, "</title>")

	if(start_index != -1 && end_index != -1){
		link.title = content_str[start_index+7:end_index]
	}
}

func get_page_urls(url string) {

	res, err := http.Get(url)
	if err != nil {
		return
	}
	content, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return
	}
	content_str := string(content)

	for {
		start_index := strings.Index(content_str, "href=\"")
		if start_index == -1 {
			break
		}
		new_str := content_str[start_index+6:]

		new_str_end_index := strings.Index(new_str, "\"")

		if new_str_end_index <= 6 {
			content_str = new_str[2:]
			continue
		}

		link_str := new_str[:new_str_end_index]
		if string(link_str[0]) == "/" {
			link_str = full_host_url + link_str
		}

		//check the links in pages blog to targe domain
		//check http/https://xxx.host.com(len(xxx) = 20),
		if index :=strings.Index(link_str,host); index == -1 || index > 20 {
			content_str = new_str[new_str_end_index:]
			continue
		}

		link_new := link{link_str, 0, 0, 0, 0,""}
		link_list = append_if_missing(link_list, link_new)
		content_str = new_str[new_str_end_index:]
	}
}

func append_if_missing(list []link, new_link link) []link {
	for _, v := range list {
		if v.url == new_link.url {
			return list
		}
	}
	return append(list, new_link)
}
