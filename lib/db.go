package lib

import "github.com/jinzhu/gorm"

type (

	//跟踪任务表
	TrackTask struct {
		TrackId         uint32 `gorm:"primary_key"`
		ExpressNumber   string
		ChannelId       uint8
		CreateTime      string `sql:"-"`
		UpdateTime      string `sql:"-"`
		TrackStatus     uint8
		LatestTrackTime string `sql:"-"`
	}

	//订阅任务表
	SubscribeTask struct {
		SubscribeId     uint32 `gorm:"primary_key"`
		TrackId         uint32
		Url             string
		SubscribeStatus uint8
		CreateTime      string `sql:"-"` //忽略标志
		UpdateTime      string `sql:"-"`
	}
	//追踪记录表
	TrackRecord struct {
		TrackId       uint32
		ExpressNumber string
		Content       string `json:"Content"`
		TrackStatus   uint8  `json:"trackStatus"`
		EventTime     string `json:"evenTime"`
		CreateTime    string `sql:"-"`
	}
	//追踪频道表
	Channel struct {
		ChannelName string
		ChannelId uint8 `gorm:"primary_key"`
	}
	//用户表
	User struct{
		Uid uint32
		Username string
		Password string
		CreateTime string
	}

)

func (TrackTask) TableName() string {
	return "track_task"
}

func (TrackRecord) TableName() string {
	return "track_record"
}

func (SubscribeTask) TableName() string {
	return "subscribe_task"
}

func (Channel) TableName() string {
	return "channel"
}

func (User) TableName() string {
	return "user"
}

//获取数据库的连接
func GetDc() *gorm.DB {
	dc, err := gorm.Open("mysql", "root:194466@/demo?charset=utf8&loc=Local")
	if err != nil {
		panic("can't connect to database")
	}
	return dc
}



const(
	TrackInited = 0
	TrackFinished = 1
	TrackAbandoned = 2
	SubscribeInited = 0
	SubscribeFinished = 1
)