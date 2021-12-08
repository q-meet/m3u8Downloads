# M3U8 视频下载

### 启动步骤

首先复制m3u8内容到 video 文件夹目录(程序会自动判断video下的m3u8文件然后执行下载过程) m3u8内容绝对路径可多文件多任务

如果m3u8内容里面为绝对路径则不用设置host参数

参数指定下载方式 输出在video目录下

```shell
go run main.go -u="http://example.com/xxxx/index.m3u8"
```


### 执行

```shell
go build
 ```

Linux

```shell
./m3u8Downloader
```

Windows PowerShell

```shell
.\m3u8Downloader.exe
```

MacOS

```shell
./m3u8Downloader
```



### 参数说明

```shell
./m3u8Downloader -h
 ```
