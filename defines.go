package main

/*
从 config.ini 读取对应日志的配置
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

type TencentClsLogSetting struct {
	//Enable          bool   `json:"enable"`
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"accesskeyid"`
	AccessKeySecret string `json:"accesskeysecret"`
	TopicId         string `json:"topicid"`
}
