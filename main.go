package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	wg      sync.WaitGroup
	path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	host    string
	chs     chan int
	//ProxyURL 代理地址
	ProxyURL   string
	num        int
	otherParam string
	referer    string
	origin     string
	userAgent  string
	httpClient *http.Client
	debug      string
)

func init() {
	flag.StringVar(&host, "host", "", "主机名 带http/https")
	flag.StringVar(&ProxyURL, "proxy", "", "代理")
	flag.IntVar(&num, "num", 20, "并发数")
	flag.StringVar(&otherParam, "otherParam", "", "url剩余部分")
	flag.StringVar(&referer, "referer", "", "referer")
	flag.StringVar(&origin, "origin", "", "origin")
	flag.StringVar(&userAgent, "userAgent", "", "userAgent")
	flag.StringVar(&debug, "debug", "", "debug")
	flag.Parse()
	chs = make(chan int, num)
	if ProxyURL != "" {
		proxy, err := url.Parse(ProxyURL)
		log.Println(ProxyURL)
		if err != nil {
			panic(err)
		}
		netTransport := &http.Transport{
			Proxy:                 http.ProxyURL(proxy),
			MaxIdleConnsPerHost:   20,
			ResponseHeaderTimeout: time.Second * time.Duration(5),
		}
		httpClient = &http.Client{
			Timeout:   time.Second * 20,
			Transport: netTransport,
		}
	} else {
		httpClient = new(http.Client)
	}
}

func main() {

	fileArr, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatalln(err.Error())
	}
	for _, f := range fileArr {
		if f.IsDir() {
			continue
		}
		if !strings.Contains(f.Name(), ".m3u8") {
			continue
		}
		wg.Add(1)
		go work(f.Name())
	}

	wg.Wait()

	fmt.Println("finishDownload")

}

func work(m3u8 string) {
	finalName := strings.ReplaceAll(m3u8, ".m3u8", "")

	tempPath := path + "\\" + "temp_" + finalName + "\\"
	_, err := os.Stat(tempPath)
	if err != nil {
		os.Mkdir(tempPath, 0644)
	}
	analysis(m3u8, tempPath)
	combine(tempPath, finalName)
	wg.Done()
}

func analysis(m3u8, tempPath string) {

	m3u8f, err := os.Open(m3u8)
	defer m3u8f.Close()
	if err != nil {
		fmt.Println(err)
	}
	m3u8reder := bufio.NewReader(m3u8f)
	no := 1
	for {
		line, _, err := m3u8reder.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			break
		}

		url := string(line)
		if strings.Contains(url, "#") {
			log.Println(url + "非url 执行跳过")
			continue
		}
		if !strings.Contains(url, "http") {
			if host != "" {

				url = host + url
				if otherParam != "" {
					url += otherParam
				}
			} else {
				log.Println("url不合法")
				panic("url不合法")
			}

		}
		// fmt.Println(string(line))
		chs <- 0 //限制线程数

		go downloads(httpClient, url, tempPath, no)
		no++
	}
}

func combine(tempPath, finalName string) {
	//------------合并------------------------
	finPath := path + "\\" + finalName + ".ts"
	finobj, _ := os.OpenFile(finPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	finW := bufio.NewWriter(finobj)
	defer finobj.Close()
	rd, _ := ioutil.ReadDir(tempPath)

	var keys []int
	for _, file := range rd {
		nums := strings.Split(file.Name(), ".")[0]
		inums, err := strconv.Atoi(nums)
		if err != nil {
			fmt.Println(err)
			break
		}
		keys = append(keys, inums)
	}
	sort.Ints(keys)
	for _, v := range keys {
		name := strconv.Itoa(v)
		tempName := tempPath + name
		merge(tempName, finW)
	}
	if debug == "" {
		os.RemoveAll(tempPath)
	}

	fmt.Println("合并完成")
}

func merge(fileName string, f *bufio.Writer) {
	tempF, err := os.Open(fileName)
	if err != nil {
		fmt.Println(err)
	}
	tempR := bufio.NewReader(tempF)
	io.Copy(f, tempR)
	tempF.Close()
	f.Flush()
	fmt.Println("合并" + fileName)
}

func downloads(httpClient *http.Client, url, tempPath string, no int) {
	defer func() {
		<-chs
	}()
	fileName := tempPath + strconv.Itoa(no)
	fmt.Println("开始下载：" + fileName)
	ff, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ff.Close()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(url)
		fmt.Println(err)
	}
	if origin != "" {
		req.Header.Add("Origin", origin)
	}
	if referer != "" {
		req.Header.Add("Referer", referer)
	}
	if referer != "" {
		req.Header.Add("Referer", referer)
	}
	if userAgent != "" {
		req.Header.Add("User-Agent", userAgent)
	}
	fmt.Printf("url=%s\n", url)
	rep, err := httpClient.Do(req)

	if err != nil {
		fmt.Println(url)
		fmt.Println(err)
	}
	defer rep.Body.Close()
	reader := bufio.NewReader(rep.Body)

	bf := bufio.NewWriter(ff)
	_, err = io.Copy(bf, reader)
	if err != nil {
		log.Printf("第%d个文件拷贝出错 error message:%s", no, err.Error())
		return
	}
	bf.Flush()

}
