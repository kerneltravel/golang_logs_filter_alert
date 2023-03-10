package main

import (
	"fmt"
	"sync"
	"time"

	cls "github.com/tencentcloud/tencentcloud-cls-sdk-go"
)

type Callback struct {
}

func (callback *Callback) Success(result *cls.Result) {
	fmt.Println(result.IsSuccessful())
	fmt.Println(result.GetErrorCode())
	fmt.Println(result.GetErrorMessage())
	fmt.Println(result.GetReservedAttempts())
	fmt.Println(result.GetRequestId())
	fmt.Println(result.GetTimeStampMs())

	attemptList := result.GetReservedAttempts()
	for _, attempt := range attemptList {
		fmt.Printf("debug Tencent CLS log sent success value %+v \n", attempt)
	}
}

func (callback *Callback) Fail(result *cls.Result) {
	fmt.Println(result.IsSuccessful())
	fmt.Println(result.GetErrorCode())
	fmt.Println(result.GetErrorMessage())
	fmt.Println(result.GetReservedAttempts())
	fmt.Println(result.GetRequestId())
	fmt.Println(result.GetTimeStampMs())
}

/*
发送成功则返回nil，否则返回具体错误信息
*/
func send_alert_to_tencent_cls_log(alertData, datetimevalue, logPath string, isDebug bool) error {

	clsclient, clsset := getClsConfig()
	clientName := getClientName()

	producerInstance, err := cls.NewAsyncProducerClient(clsclient)
	if err != nil {
		return err
	}
	//alertMap, err := JsonToMap(alertData)
	alertMap := map[string]string{
		"from":       clientName,
		"logtime":    datetimevalue,
		"logpath":    logPath,
		"logcontent": alertData,
	}
	// 异步发送程序，需要启动
	producerInstance.Start()

	var m sync.WaitGroup
	callBack := &Callback{}

	m.Add(1)
	go func() {
		defer m.Done()
		//for i := 0; i < 1; i++ {

		log := cls.NewCLSLog(time.Now().Unix(), alertMap)
		if isDebug == true {
			err := producerInstance.SendLog(clsset.TopicId, log, callBack) //debug 模式下可以开启callBack
			//批量发送的接口：func (producer *AsyncProducerClient) SendLogList(topicId string, logList []*Log, callback CallBack) (err error) {
			if err != nil {
				fmt.Println(err)
				//return err
			}
		} else {
			err := producerInstance.SendLog(clsset.TopicId, log, nil) // 非debug模式下可以不打印callBack 信息
			if err != nil {
				fmt.Println(err)
				//return err
			}
		}
	}()

	m.Wait()
	producerInstance.Close(30000)

	return nil
}
