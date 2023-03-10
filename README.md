* Project goals & functions:

1. to monitor multiple logs files on client side and in realtime filter out the valuable logs to upload for log-alerting, by only upload the filter log , making the ETL happen in client side, so the log serverside need fewer space for the dirty-useless logs .

2. client can batch monitor log file name by regex-pattern.(current not support newly created log file)

* How to compile
on windows:
```
go mod init logalert
go mod tidy
go build .
```

[Optional] Cross compile
on windows minGW cross-compile to linux biary by go.
```
$ export CGO_ENABLED=0
$ export GOOS=linux
$ export GOARCH=amd64
```
* Usage
1. edit config.ini file, set WhiteListFrom URL, which contains the filtered pattern list.
2. start the logalert.exe in background by supervisor or other tools.

* Plans
1. to support auto monitor newly generated log file, with out restarting programs.
2. to send log to cloud log-server , e.g. the tencent CLS and aliyun SLS log-server.


* License

[MIT License](https://github.com/duthied/Free-Friendika/blob/master/LICENSE)

