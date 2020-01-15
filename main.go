package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	wg      sync.WaitGroup
	chs     = make(chan int, 20)
	path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	host    string
)

func init() {
	flag.StringVar(&host, "host", "", "主机名 带http/https")
}

func main() {
	flag.Parse()
	_, err := os.Stat(path + "\\temp")
	if err != nil {
		os.Mkdir(path+"\\temp", 0644)
	}
	//--------------------下载部分-----------
	m3u8 := "video.m3u8"
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
			} else {
				log.Println("url不合法")
				panic("url不合法")
			}

		}
		// fmt.Println(string(line))
		chs <- 0 //限制线程数
		wg.Add(1)
		go downloads(url, no)
		no++
	}

	wg.Wait()

	fmt.Println("finishDownload")
	//-----------------结束下载--------------------------

	//------------合并------------------------
	finPath := path + "\\final.ts"
	finobj, _ := os.OpenFile(finPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	finW := bufio.NewWriter(finobj)
	defer finobj.Close()
	rd, _ := ioutil.ReadDir(path + "\\temp\\")

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
		tempPath := path + "\\temp\\" + name
		merge(tempPath, finW)
	}
	os.RemoveAll(path + "\\temp")
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

func downloads(url string, no int) {
	defer func() {
		<-chs
		wg.Done()
	}()
	fileName := path + "\\temp\\" + strconv.Itoa(no)
	fmt.Println("开始下载：" + fileName)
	ff, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ff.Close()
	rep, err := http.Get(url)
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
