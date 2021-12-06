# M3U8 视频下载 v1.0.0

### 启动步骤

首先复制m3u8内容到 video 文件夹目录(程序会自动判断video下的m3u8文件然后执行下载过程) m3u8内容绝对路径可多文件多任务

如果m3u8内容里面为绝对路径则不用设置host参数

### 执行

```go build ```

http://example.com = m3u8指向域名

Linux

```
./m3u8Downloader -host="http://example.com"
```

Windows PowerShell

```
.\m3u8Downloader.exe -host="http://example.com" 
```

MacOS

```
./m3u8Downloader -host=http://example.com
```

