package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gookit/ini"
	"github.com/nxadm/tail"
)

/* 需改进成从外部动态读取最新白名单规则列表，作为 okLogRules的规则内容（只要有这些单词的就认为改日志条目无需告警）
var okLogRules = []string{
	`请登录`,
	`转存成功`,
	`paramsLogs`,
	`add_credit`,
	`testtemp`,
	`newCommCount`,
	`listCount`,
	`unzip`,
	`commCount`,
	`resLogs`,
	`user\/getPrivateSwitch`,
	`ETag`,
	`Allready Exist in COS`,
	`SSL_do_handshake`,
	`php_network_getaddresses`,
	`\.well-known\/`,
}
*/

/*
从 local_config.ini 读取对应日志的配置
*/
type LogFileConfig struct {
	// File-specifc
	LogType        string
	LogPath        string
	ParentPath     string
	DatetimeFormat string
	WhiteListFrom  string
	IsNewFileDaily bool
}

/*
返回指定字符串（当前日志行内容）是否包含特定【日期 时间】 格式的内容，
如果包含则返回 true,日期时间 值，否则返回 false,""
*/
func currentLineContainDatetime(currentLine, DatetimeFormat string) (bool, string) {
	if strings.Trim(currentLine, " ") == "" {
		return false, ""
	}
	//reg1 := regexp.MustCompile(`【\d+\-\d+\-\d+ \d+\:\d+\:\d+】`)
	reg1 := regexp.MustCompile(DatetimeFormat)
	if reg1 == nil {
		fmt.Println("regexp err")
		return false, ""
	}
	//根据规则提取关键信息
	timeStr := reg1.FindString(currentLine)
	if len(timeStr) > 0 { //字符串为【】包裹的日期时间串
		//timeStr = strings.Replace(timeStr, "【", "", -1)
		//timeStr = strings.Replace(timeStr, "】", "", -1)
		//fmt.Println("old line is:", currentLine, "log datetime is：", timeStr)

		return true, timeStr
	} else {
		return false, ""
	}

}

/*
若需要告警，即该字符串未匹配到白名单的任何规则，则返回 true。
否则返回 false
*/
func needAlert(oldlines string, okLogRules *[]string) bool {
	//默认需要告警
	need := true

	for _, value := range *okLogRules {
		reg1 := regexp.MustCompile(value)
		/* TODO: 需要错误处理
		if reg1 == nil {
			fmt.Println("regexp err")
			return false
		}*/
		//根据规则提取关键信息
		timeStr := reg1.FindString(oldlines)

		//若有匹配任意白名单规则，则无需告警
		if len(timeStr) > 0 {
			need = false
			break
		}
	}
	return need
}

/*
对于需要告警的，则发送告警。
返回告警发送状态： true,"" 或false,"发送失败原因"
*/
func doAlert_sendmsg(oldlines, datetimevalue, logPath string) (bool, string) {
	//TODO: 改成发送消息到redis或其他消息存储后端，以集中记录错误日志，进一步可以告警
	fmt.Println("Alert:", logPath, oldlines, datetimevalue)
	return true, ""
}

//日志的log段已读取完整，如果需要告警，则在本函数发起告警
func FinishedOldlines_processLog(oldlines, datetimevalue string, okLogRules *[]string, logPath string) {
	//fmt.Println(oldlines, datetimevalue)
	// 去掉换行， 进行白名单匹配。若不在白名单中，则返回
	//if needAlert(strings.Join(strings.Split(oldlines, string("\n")), " ")) == true {
	if needAlert(oldlines, okLogRules) == true {
		//fmt.Println(logPath)
		doAlert_sendmsg(oldlines, datetimevalue, logPath)
	} else {

	}
}

func parseLogRulesFromUrl(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	strbody := string(body)
	return strings.Split(strbody, "\n"), err
}

/*
从日志文件实时读取最新日志，待分析日志内容是否需要告警
*/
func tailLogFile(logFileConfig *LogFileConfig) { //func tailLogFile(logfile, DatetimeFormat string) {
	// 使用TailFile库从日志文件实时读取最新日志
	logfile := logFileConfig.LogPath
	DatetimeFormat := logFileConfig.DatetimeFormat
	t, err := tail.TailFile(
		//"2022-10-07_logs.txt",
		logfile,
		tail.Config{MustExist: true, Follow: true, ReOpen: true})

	if err != nil {
		panic(err)
	}

	oldlines := ""
	okLogRules, err := parseLogRulesFromUrl(logFileConfig.WhiteListFrom)
	//fmt.Println("网络获取规则内容：", okLogRules)

	//TODO: 网络错误的处理
	if err != nil {
		fmt.Println("获取规则文件URL内容失败", err)
		return
	}

	// 每次追加新日志的时候，会触发下面的日志分析，如果属于异常日志则告警
	for line := range t.Lines {

		//忽略日志中的空行
		if strings.Trim(line.Text, " ") == "" {
			continue
		}

		//fmt.Println(line.Text)

		//根据日志是否以【日期时间】开始，作为判断当前读取的日志属于日志的新段落还是日志的旧段落。若新段落则从空字符串开始拼接为新内容
		if ok, datetimevalue := currentLineContainDatetime(line.Text, DatetimeFormat); ok == true {
			//fmt.Println("datetime is  ", datetimevalue, line.Text)
			FinishedOldlines_processLog(oldlines, datetimevalue, &okLogRules, logFileConfig.LogPath)
			oldlines = ""
			oldlines += fmt.Sprintf(" %s", line.Text) // 注意：这里去掉了原日志的换行
		} else { //若为日志的旧段落，则继续拼接到之前的日志段落内容上
			oldlines += fmt.Sprintf(" %s", line.Text) // 注意：这里去掉了原日志的换行
		}
	}
}

/*根据query 的文件名模式，找到dirpath 目录下的最新日志文件，
返回： true,目标日志文件名
       若近期无日志变化，则返回false,""
*/
func lookupTargetLogFile(dirpath, query string) (bool, string) {
	var lastTimeFile string
	lastTimeFile = ""
	ok := true

	//2天之前
	d, _ := time.ParseDuration("-48h")
	oldModTime := time.Now().Add(d)

	filefullpath := dirpath + query
	//fmt.Println(filefullpath)
	matches, _ := filepath.Glob(filefullpath)

	for _, vfpath := range matches {
		//fmt.Println(k, vfpath)
		fstat, _ := os.Stat(vfpath)
		fLastModTime := fstat.ModTime()
		if fLastModTime.After(oldModTime) {
			oldModTime = fLastModTime
			lastTimeFile = vfpath
		}
	}
	if lastTimeFile == "" {
		ok = false
	}
	return ok, lastTimeFile
}

/*根据指定的文件名格式和日志参数，开始监控日志文件决定是否对最新的日志告警
 */
func startMonitorLogFile(logFileKey string, logFileInfo map[string]string, waitGroup *sync.WaitGroup) {
	//fmt.Println(logFileKey, logFileInfo)

	defer waitGroup.Done()
	query := logFileInfo["Pattern"]

	_, logPath := lookupTargetLogFile(logFileInfo["ParentPath"], query)

	//fmt.Println("----", logFileKey, logFileInfo, ok, logPath)
	IsNewFileDaily, err := strconv.ParseBool(logFileInfo["IsNewFileDaily"])
	if err != nil {
		panic(err)
	}

	logFileConfig := LogFileConfig{
		LogType:        logFileInfo["LogType"],
		LogPath:        logPath,
		ParentPath:     logFileInfo["ParentPath"],
		DatetimeFormat: logFileInfo["DatetimeFormat"],
		WhiteListFrom:  logFileInfo["WhiteListFrom"],
		IsNewFileDaily: IsNewFileDaily,
	}

	//tailLogFile(logPath, DatetimeFormat)
	tailLogFile(&logFileConfig)
}

func main() {
	configFilePath := "config.ini"
	configFileDefault := ` 配置文件读取失败，未定义LocalServiceList ，参考配置：

	LocalServiceList = runtime_log,error_log
	
	[runtime_log]
	ParentPath=/www/wwwroot/site.domain.com/runtime/log_path/
	Pattern=*_runtime.log
	IsNewFileDaily=true
	LogType=runtimelog
	DatetimeFormat=\d+\-\d+\-\d+ \d+\:\d+\:\d+
	WhiteListFrom=https://rule_list_from.domain.net/whitelist.txt
	
	[error_log]
	ParentPath=/www/wwwlogs/
	Pattern=*.error.log
	IsNewFileDaily=true
	LogType=runtimelog
	DatetimeFormat=\d+\-\d+\-\d+ \d+\:\d+\:\d+
	WhiteListFrom=https://rule_list_from.domain.net/whitelist.txt
	`
	localConfig, err := ini.LoadExists(configFilePath)
	if err != nil {
		panic(err)
	}
	//fmt.Println(localConfig.Data())
	localServiceConfig, ok := localConfig.String("LocalServiceList")
	if !ok {
		//fmt.Println(localServiceConfig, ok)
		panic(localServiceConfig + configFileDefault)
	}

	var wg sync.WaitGroup //WaitGroup 方式实现主线程等待子线程都完成后再退出(甚至永远运行以跟踪新日志文件)

	//localServiceConfig2, ok := localConfig.StringMap("LocalServiceList")
	//fmt.Println(localServiceConfig2, ok)
	for _, configKey := range strings.Split(localServiceConfig, ",") {
		logFileInfo, _ := localConfig.StringMap(configKey)
		//fmt.Println(configKey, logFileInfo, ok)
		wg.Add(1)
		go startMonitorLogFile(configKey, logFileInfo, &wg)
	}
	//测试阶段，需要启用以下代码，以防止go 调用的startMonitorLogFile() 协程 在main()退出之后才启动就无法看到协程的输出信息。
	//time.Sleep(100 * time.Second)
	wg.Wait() // 代替 //time.Sleep(100 * time.Second) 的方式等待协程退出

	//维护说明：可每天 09:00以后和 12:00重启一次服务
}
