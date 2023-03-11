package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-ini/ini"
	//_ "github.com/mkevac/debugcharts"
	"github.com/nxadm/tail"
	cls "github.com/tencentcloud/tencentcloud-cls-sdk-go"
	//"net/http/pprof"
	//pprof "runtime/pprof"
	//"syscall"
)

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
	if len(timeStr) > 0 {
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
	send_alert_to_tencent_cls_log(oldlines, datetimevalue, logPath, true)

	fmt.Println("Alert:", logPath, oldlines, datetimevalue)
	return true, ""
}

//日志的log段已读取完整，如果需要告警，则在本函数发起告警
func FinishedOldlines_processLog(oldlines, datetimevalue string, okLogRules *[]string, logPath string) {

	//fmt.Println(oldlines, datetimevalue)
	// 去掉换行， 进行白名单匹配。若不在白名单中，则返回
	//if needAlert(strings.Join(strings.Split(oldlines, string("\n")), " ")) == true {
	if needAlert(oldlines, okLogRules) == true {
		//ch := getLogChan()
		logsListChannel <- LogBody{Mesg: oldlines, Datetime: datetimevalue, LogPath: logPath}
		//fmt.Println("检测到新日志要发送：", logPath, datetimevalue, oldlines, "队列长度", len(logsListChannel))
		/*
			fmt.Println("当前队列中的日志", <-ch)
		*/
		//doAlert_sendmsg(oldlines, datetimevalue, logPath)
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
func tailErrLogFile(logFileConfig *LogFileConfig) { //func tailLogFile(logfile, DatetimeFormat string) {
	// 使用TailFile库从日志文件实时读取最新日志
	logfile := logFileConfig.LogPath
	DatetimeFormat := logFileConfig.DatetimeFormat
	tailfile, err := tail.TailFile(
		//"2022-10-07_logs.txt",
		logfile,
		//如果在windows系统，只能用Poll=true
		tail.Config{MustExist: true, Follow: true, ReOpen: true, Poll: false, Location: &tail.SeekInfo{Offset: 2, Whence: io.SeekEnd}})

	if err != nil {
		fmt.Println("error logfile :"+logfile, err)
		//panic(err)
		log.Println(err, logfile, "日志文件读取失败")
		return
	}
	defer tailfile.Cleanup()

	oldlines := ""
	okLogRules, err := parseLogRulesFromUrl(logFileConfig.WhiteListFrom)
	//fmt.Println("网络获取规则内容：", okLogRules)

	//TODO: 网络错误的处理
	if err != nil {
		fmt.Println("获取规则文件URL内容失败", err)
		return
	}

	// 每次追加新日志的时候，会触发下面的日志分析，如果属于异常日志则告警
	for line := range tailfile.Lines {

		//忽略日志中的空行
		if strings.Trim(line.Text, " ") == "" {
			continue
		}

		//fmt.Println(line.Text)

		//根据日志是否以【日期时间】开始，作为判断当前读取的日志属于日志的新段落还是日志的旧段落。若新段落则从空字符串开始拼接为新内容
		if ok, datetimevalue := currentLineContainDatetime(line.Text, DatetimeFormat); ok == true {
			//fmt.Println("datetime is  ", datetimevalue, line.Text)
			//如果在windows系统，只能用Poll=true
			FinishedOldlines_processLog(oldlines, datetimevalue, &okLogRules, logFileConfig.LogPath)
			oldlines = ""
			oldlines += fmt.Sprintf(" %s", line.Text) // 注意：这里去掉了原日志的换行
		} else { //若为日志的旧段落，则继续拼接到之前的日志段落内容上
			oldlines += fmt.Sprintf(" %s", line.Text) // 注意：这里去掉了原日志的换行
		}
	}
}

/*
从日志文件实时读取最新日志，待分析日志内容是否需要告警
*/
func tailNewestLogFile(logFileConfig *LogFileConfig) { //func tailLogFile(logfile, DatetimeFormat string) {
	// 使用TailFile库从日志文件实时读取最新日志
	logfile := logFileConfig.LogPath
	DatetimeFormat := logFileConfig.DatetimeFormat
	t, err := tail.TailFile(
		logfile,
		//tail.Config{MustExist: true, Follow: true, ReOpen: true})
		tail.Config{MustExist: true, Follow: true, ReOpen: true, Poll: false, Location: &tail.SeekInfo{Offset: 2, Whence: io.SeekEnd}})

	if err != nil {
		fmt.Println("TailFile error, logfile：", logfile)
		//TODO 改成return
		//panic(err)
		log.Println(err, logfile, "日志文件读取失败")
		return
	}
	defer t.Cleanup()

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

/*根据query 的文件名模式，找到dirpath 目录下的指定名称的错误日志文件，
返回： true,目标日志文件名
       若近期无日志变化，或目录/文件不存在，则返回false,""
*/
func lookupTargetErrLogFile(dirpath, query string) (bool, string) {
	ok := true

	filefullpath := dirpath + query
	if !IsDir(dirpath) || !IsFile(filefullpath) {
		fmt.Println("指定目录或文件不存在：", dirpath, filefullpath)
		return false, ""
	}

	/*
		var lastTimeFile string
		lastTimeFile = ""
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
	*/
	return ok, filefullpath
}

/*根据query 的文件名模式，找到dirpath 目录下的最新日志文件，
返回： true,目标日志文件名
       若近期无日志变化，则返回false,""
*/
func lookupNewestLogFile(dirpath, query string) (bool, string) {
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

	var logPath string
	var logType = strings.Trim(strings.ToLower(logFileInfo["LogType"]), " ")
	switch {
	case "runtimelog" == logType:
		{
			_, logPath = lookupNewestLogFile(logFileInfo["ParentPath"], query)
		}
	case "errorlog" == logType:
		{
			_, logPath = lookupTargetErrLogFile(logFileInfo["ParentPath"], query)
		}
	}

	if strings.Trim(logPath, " ") == "" {
		switch {
		case "runtimelog" == logType:
			{
				fmt.Println("未找到匹配的日志文件。", "目录：", logFileInfo["ParentPath"], "匹配规则：", query, "时间范围：近2天内")
			}
		case "errorlog" == logType:
			{
				fmt.Println("未找到匹配的日志文件。", "目录：", logFileInfo["ParentPath"], "匹配规则：", query)
			}

		}
		return
	}
	//fmt.Println("----", logFileKey, logFileInfo, ok, logPath)
	IsNewFileDaily, err := strconv.ParseBool(logFileInfo["IsNewFileDaily"])
	if err != nil {
		//panic(err)
		log.Println(err, logFileInfo, "配置文件读取失败，IsNewFileDaily 配置项读取失败")
		return
	}

	logFileConfig := LogFileConfig{
		LogType:        logFileInfo["LogType"],
		LogPath:        logPath,
		ParentPath:     logFileInfo["ParentPath"],
		DatetimeFormat: logFileInfo["DatetimeFormat"],
		WhiteListFrom:  logFileInfo["WhiteListFrom"],
		IsNewFileDaily: IsNewFileDaily,
	}

	if IsNewFileDaily == true { //这种模式会要求主程序每天重启一次（以后可以改成根据时间做判断）
		tailNewestLogFile(&logFileConfig)
	} else { //这种模式只需要关注匹配的文件的最新末尾内容。
		tailErrLogFile(&logFileConfig)
	}
}

type LogBody struct {
	Mesg     string
	Datetime string
	LogPath  string
}

var clslogset TencentClsLogSetting
var clsProducerConfig *cls.AsyncProducerClientConfig
var clientName string
var clsLog_enable, clsLog_isDebug bool
var logsListChannel = make(chan LogBody, 100000)
var error_log_output_file = "./error_log_output.txt"

func getClsConfig() (*cls.AsyncProducerClientConfig, *TencentClsLogSetting) {
	return clsProducerConfig, &clslogset
}

func getClientName() string {
	return clientName
}

func getclsLog_enable() bool {
	return clsLog_enable
}

func getLogChan() chan LogBody {
	return logsListChannel
}

func consumeLog() {
	//ch :=
	for {
		select {
		//case logBody := <-getLogChan():
		case logBody := <-logsListChannel:
			{
				//for s := range logBody {
				fmt.Printf("收到消息 %v\n", logBody)
				doAlert_sendmsg(logBody.Mesg, logBody.Datetime, logBody.LogPath)
			}

			//default:  // bug fix: 如果select { } 一直不匹配case logBody := <-logsListChannel，则每次都 continue，会导致cpu占用高。下面default: continue 注释掉即可。
			//continue
			//fmt.Println("等待日志消息……", len(logsListChannel))
		}
	}
}

func init() {
	errorlogfile, err := os.Create(error_log_output_file)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(errorlogfile)
}

/*
// for tracing CPU load-high bugs
func handleCtrlC(c chan os.Signal) {
	sig := <-c
	// handle ctrl+c event here
	// for example, close database
	fmt.Println("\nsignal: ", sig)
	pprof.StopCPUProfile()
	time.Sleep(time.Second * 5)
	os.Exit(0)
}
*/

func main() {
	/*
		f, err := os.OpenFile("profile", os.O_CREATE|os.O_RDWR, 0644)
		if nil != err {
			fmt.Println("pprofile 创建失败", err)
		}

		pprof.StartCPUProfile(f)
		time.Sleep(time.Second * 10)
		pprof.StopCPUProfile()
		//defer pprof.StopCPUProfile()
	*/
	/*if err := http.ListenAndServe(":6060", nil); err != nil {
		log.Fatal(err)
	}
	*/

	/*
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		profile, err := os.Create("cpuProfile")
		defer profile.Close()
		if err := pprof.StartCPUProfile(profile); err != nil {
			log.Fatal("could not start cpu profile", err)
		}
		go handleCtrlC(c)
		//defer pprof.StopCPUProfile()
	*/

	configFilePath := "config.ini"

	localConfig, err := ini.Load(configFilePath)
	if err != nil {
		//panic(err)
		log.Fatal(err, "配置文件config.ini读取失败")
	}
	//fmt.Println(localConfig.Data())
	localServiceConfig := localConfig.Section("app").Key("LocalServiceList").String()
	clientName = localConfig.Section("app").Key("ClientName").String()

	clsLog_enable, err = localConfig.Section("tencent_cls_log").Key("enable").Bool()
	if nil != err {
		clsLog_enable = false
		fmt.Println(configFilePath + "配置项错误：[tencent_cls_log]enable")
		log.Println(err, configFilePath, configFilePath+"配置项错误：[tencent_cls_log]enable")
		return
	}
	clsLog_isDebug, err = localConfig.Section("tencent_cls_log").Key("debug").Bool()
	if nil != err {
		clsLog_isDebug = false
		fmt.Println(configFilePath + "配置项错误：[tencent_cls_log]debug")
		log.Println(err, configFilePath, configFilePath+"配置项错误：[tencent_cls_log]enable")
		return
	}

	clsProducerConfig = cls.GetDefaultAsyncProducerClientConfig()
	clslogset.Endpoint, clslogset.AccessKeyID, clslogset.AccessKeySecret, clslogset.TopicId = localConfig.Section("tencent_cls_log").Key("Endpoint").String(), localConfig.Section("tencent_cls_log").Key("AccessKeyID").String(), localConfig.Section("tencent_cls_log").Key("AccessKeySecret").String(), localConfig.Section("tencent_cls_log").Key("TopicId").String()

	clsProducerConfig.Endpoint = clslogset.Endpoint
	clsProducerConfig.AccessKeyID = clslogset.AccessKeyID
	clsProducerConfig.AccessKeySecret = clslogset.AccessKeySecret

	go consumeLog()

	var wg sync.WaitGroup //WaitGroup 方式实现主线程等待子线程都完成后再退出(甚至永远运行以跟踪新日志文件)

	for _, configKey := range strings.Split(localServiceConfig, ",") {
		logFileInfo := map[string]string{}
		keynames := localConfig.Section(configKey).KeyStrings()
		for _, name := range keynames {
			logFileInfo[name] = localConfig.Section(configKey).Key(name).String()
		}

		//fmt.Println(configKey, logFileInfo, ok)
		wg.Add(1)
		go startMonitorLogFile(configKey, logFileInfo, &wg)
	}
	//测试阶段，需要启用以下代码，以防止go 调用的startMonitorLogFile() 协程 在main()退出之后才启动就无法看到协程的输出信息。
	//time.Sleep(100 * time.Second)
	wg.Wait() // 代替 //time.Sleep(100 * time.Second) 的方式等待协程退出
	//维护说明：可每天 09:00以后和 12:00重启一次服务
}
