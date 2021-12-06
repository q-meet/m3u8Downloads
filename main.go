package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	wg        sync.WaitGroup
	path, _   = filepath.Abs(filepath.Dir(os.Args[0]))
	host      string
	chs       chan int
	chsWorker chan int
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
	chsWorker = make(chan int, num)
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

type pathStruct struct {
	F    os.FileInfo
	Path string
}

func main() {
	// 默认当前目录下文件搜索m3u8文件  修改为video文件夹
	path += "\\video"
	fileArr, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatalln(err.Error())
	}

	var pathS []pathStruct
	for _, f := range fileArr {
		p := pathStruct{
			F:    f,
			Path: path,
		}
		pathS = append(pathS, p)
	}

	for len(pathS) > 0 {
		ps := pathS[0]
		pathS = pathS[1:]
		if ps.F.IsDir() {
			_pathInfoArr, err := ioutil.ReadDir(ps.Path + "\\" + ps.F.Name())
			if err != nil {
				stdErr(err.Error())
				continue
			}
			for _, _f := range _pathInfoArr {
				_p := pathStruct{
					F:    _f,
					Path: ps.Path + "\\" + ps.F.Name() + "\\",
				}
				pathS = append(pathS, _p)
			}
		} else {
			check(ps)
		}
	}

	wg.Wait()

	fmt.Println("Done")

}

func check(ps pathStruct) {
	if !strings.Contains(ps.F.Name(), ".m3u8") {
		return
	}
	wg.Add(1)
	chsWorker <- 0
	go work(ps.F.Name(), ps.Path)
}

func work(m3u8, basePath string) {
	defer func() {
		<-chsWorker
		wg.Done()
	}()
	finalName := strings.ReplaceAll(m3u8, ".m3u8", "")

	tempPath := basePath + "\\" + "temp_" + finalName + "\\"

	_, err := os.Stat(tempPath)
	if err != nil {
		os.Mkdir(tempPath, 0644)
	}

	analysis(basePath+"/"+m3u8, tempPath)
	combine(basePath, tempPath, finalName)
}

func analysis(m3u8, tempPath string) {
	downloadsWg := new(sync.WaitGroup)
	m3u8f, err := os.Open(m3u8)
	if err != nil {
		stdErr(err.Error())
		remove(tempPath)
		return
	}
	defer remove(m3u8)
	defer m3u8f.Close()
	m3u8reder := bufio.NewReader(m3u8f)
	no := 1
	key := ""
	for {
		line, _, err := m3u8reder.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			stdErr(err.Error())
			remove(tempPath)
			return
		}

		url := string(line)
		if strings.HasPrefix(url, "#EXT-X-KEY"){
			keyInfo := parseLineParameters(url)
			fmt.Println(keyInfo)
			if keyUrl, ok := keyInfo["URI"];ok && keyUrl != ""{
				if !strings.Contains(keyUrl, "http") {
					keyUrl = host + keyUrl
				}
				fmt.Println(keyUrl)
				keyBody, err := Get(keyUrl)
				if err != nil {
					log.Println("get key error", err)
					continue
				}
				keyBodyStr , err := ioutil.ReadAll(keyBody)
				if err != nil {
					log.Println("get key error", err)
					continue
				}
				key = string(keyBodyStr)
			}
		}
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
		fmt.Println(string(line))
		chs <- 0 //限制线程数
		downloadsWg.Add(1)
		go downloads(httpClient, url, tempPath, no, downloadsWg, key)
		no++
	}
	downloadsWg.Wait()
	allTemp, err := ioutil.ReadDir(tempPath)
	if err != nil {
		stdErr(err.Error())
		defer func() {
			remove(tempPath)
		}()
		return
	}
	if len(allTemp) != (no - 1) {
		stdErr("文件数量不正确")
		defer func() {
			remove(tempPath)
		}()
		return
	}
	return
}

func combine(basePath, tempPath, finalName string) {

	rd, err := ioutil.ReadDir(tempPath)
	if err != nil {
		remove(tempPath)
		stdErr(err.Error())
		return
	}

	//------------合并------------------------
	finPath := basePath + "\\" + finalName + ".ts"
	fmt.Println("合并" + finPath)
	finobj, err := os.OpenFile(finPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		stdErr(err.Error())
		remove(finPath)
	}
	defer finobj.Close()
	finW := bufio.NewWriter(finobj)
	var keys []int
	for _, file := range rd {
		nums := strings.Split(file.Name(), ".")[0]
		inums, err := strconv.Atoi(nums)
		if err != nil {
			stdErr(err.Error())
			defer func() {
				finobj.Close()
				remove(finPath)
			}()
			return
		}
		keys = append(keys, inums)
	}
	sort.Ints(keys)
	for _, v := range keys {
		name := strconv.Itoa(v)
		tempName := tempPath + name
		err = merge(tempName, finW)
		if err != nil {
			defer func() {
				finobj.Close()
				remove(finPath)
			}()
			stdErr(err.Error())
			return
		}
	}
	if debug == "" {
		err := os.RemoveAll(tempPath)
		if err != nil {
			defer func() {
				finobj.Close()
				remove(finPath)
			}()
			stdErr(err.Error())
		}
	}

	fmt.Println("合并完成")
}

func remove(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		stdErr(err.Error() + "\n")
	}
}

func merge(fileName string, f *bufio.Writer) error {
	tempF, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer tempF.Close()
	tempR := bufio.NewReader(tempF)
	_, err = io.Copy(f, tempR)
	if err != nil {
		return err
	}
	f.Flush()
	fmt.Println("合并" + fileName)
	return nil
}

func stdErr(msg string) {
	fmt.Fprintf(os.Stderr, msg)
}

func downloads(httpClient *http.Client, url, tempPath string, no int, downloadsWg *sync.WaitGroup, key string) {
	defer func() {
		<-chs
		downloadsWg.Done()
	}()
	fileName := tempPath + strconv.Itoa(no)
	fmt.Println("开始下载：" + fileName)
	ff, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		defer func() {
			remove(fileName)
		}()
		stdErr(err.Error())
		return
	}
	defer ff.Close()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		defer func() {
			ff.Close()
			remove(fileName)
		}()
		stdErr(err.Error())
		return
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
	// fmt.Printf("url=%s\n", url)
	rep, err := httpClient.Do(req)

	if err != nil {
		defer func() {
			ff.Close()
			remove(fileName)
		}()
		stdErr(err.Error())
		return
	}
	defer rep.Body.Close()
	reader := bufio.NewReader(rep.Body)

	bf := bufio.NewWriter(ff)

	all, _ := ioutil.ReadAll(reader)
	if key != "" {
		all, err = AES128Decrypt(all, []byte(key), []byte(""))
		if err != nil {
			defer func() {
				ff.Close()
				remove(fileName)
			}()
			stdErr(err.Error())
			return
		}
	}
	bf.Write(all)
/*
	_, err = io.Copy(bf, reader)
	if err != nil {
		defer func() {
			ff.Close()
			remove(fileName)
		}()
		stdErr(err.Error())
		return
	}*/
	bf.Flush()

}

func AES128Encrypt(origData, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(iv) == 0 {
		iv = key
	}
	origData = pkcs5Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

func AES128Decrypt(crypted, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(iv) == 0 {
		iv = key
	}
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = pkcs5UnPadding(origData)
	return origData, nil
}

func pkcs5Padding(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipherText, padText...)
}

func pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}


// regex pattern for extracting `key=value` parameters from a line
var linePattern = regexp.MustCompile(`([a-zA-Z-]+)=("[^"]+"|[^",]+)`)

// parseLineParameters extra parameters in string `line`
func parseLineParameters(line string) map[string]string {
	r := linePattern.FindAllStringSubmatch(line, -1)
	params := make(map[string]string)
	for _, arr := range r {
		params[arr[1]] = strings.Trim(arr[2], "\"")
	}
	return params
}
func Get(url string) (io.ReadCloser, error) {
	c := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: time.Duration(60) * time.Second,
	}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error: status code %d", resp.StatusCode)
	}
	return resp.Body, nil
}
