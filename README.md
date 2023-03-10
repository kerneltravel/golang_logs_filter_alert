*功能*

监控日志文件变化，过滤出感兴趣的内容（不在白名单内的都属于待告警的内容）并发送告警。基于golang实现的，支持正则匹配，在配置文件中自定义匹配规则。

*【编译】*

windows 下编译：
```
go mod init logalert
go mod tidy
go build .
```

[可选]
windows下编译得到 适合linux的可执行文件：
```
$ export CGO_ENABLED=0
$ export GOOS=linux
$ export GOARCH=amd64
```

* 【运行方式】 *

1. 需配置文件 config.ini （日志接收端的验证信息配置、 WhiteListFrom 对应的告警白名单内容的URL 地址准备）
2. 然后将编译得到的 logalert.exe 文件可以命令方式在后台运行。 


* 【功能计划】 *

1. 对指定目录下后续新产生的日志文件，也能被加入监控，而无需重启软件。

*License*

MIT
