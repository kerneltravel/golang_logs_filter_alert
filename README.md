*功能*

监控日志文件变化，过滤感兴趣的内容并发送告警。基于golang实现的，支持正则匹配，在配置文件中自定义匹配规则。

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

*【运行方式】*

编译得到的 logalert.exe 文件可以命令方式运行
需配置文件 config.ini


*License*

MIT
