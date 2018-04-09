package main

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"time"
	"net/http"
	"fmt"
	"bytes"
	"encoding/json"
	"../lib"
)

func main() {
	//遍历每一个处于持续订阅状态的订阅任务
	//先根据订阅任务的更新时间来发送新的追踪记录,然后刷新任务的更新时间
	//发送完成之后更新本追踪任务的状态，如果有完成的或者是很久没有更新的任务则关闭订阅任务
	var subscribeTasks []lib.SubscribeTask
	dc.Where("subscribe_status = 0 ").Limit(100).Find(&subscribeTasks)
	fmt.Println(len(subscribeTasks))
	if len(subscribeTasks) == 0 {
		//没有活跃的订阅任务，直接返回
		fmt.Println("no new records to notify subscribers")
		return
	}
	for _, subscribeTask := range subscribeTasks {
		var trackTask lib.TrackTask
		dc.Where("track_id = ? ", subscribeTask.TrackId).Find(&trackTask)
		tTime, err := time.Parse("2006-01-02 15:04:05", trackTask.LatestTrackTime)
		if err != nil {
			continue
		}
		var sTime time.Time
		needNotify := false
		if subscribeTask.UpdateTime == "" {
			needNotify = true
		} else {
			sTime, err = time.Parse("2006-01-02 15:04:05", subscribeTask.UpdateTime)
			if err != nil {
				fmt.Println("can't parse update time of subscription")
				continue
			}
			if tTime.After(sTime) {
				needNotify = true
			}
		}
		if !needNotify {
			continue
		}
		//有新的追踪记录，开始分发
		var trackRecords []lib.TrackRecord
		dc.Where("event_time > ? ", subscribeTask.UpdateTime).Find(&trackRecords)
		pushRecords(trackRecords, subscribeTask.Url)
		fmt.Println(trackTask)
		if trackTask.TrackStatus == lib.TrackFinished || trackTask.TrackStatus == lib.TrackAbandoned {
			//追踪结束或者追踪被遗弃，更改追踪任务状态
			dc.Debug().Table("subscribe_task").Where("subscribe_id = ? ", subscribeTask.SubscribeId).Updates(map[string]interface{}{"update_time": trackTask.LatestTrackTime, "subscribe_status": lib.SubscribeFinished})
		} else {
			fmt.Println("not finished")
			dc.Debug().Table("subscribe_task").Where("subscribe_id = ? ", subscribeTask.SubscribeId).UpdateColumn("update_time" ,trackTask.LatestTrackTime)
		}
	}
}

var dc *gorm.DB

func init() {
	dc = lib.GetDc()

}

//推送最新的追踪记录给url
func pushRecords(trackRecords []lib.TrackRecord, url string) {
	client := &http.Client{
	}
	jsonedRecords, err := json.Marshal(trackRecords)
	if err != nil {
		return
		fmt.Println(err)
	}
	fmt.Println(url)
	fmt.Println(trackRecords)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonedRecords))
	req.Header.Add("content-type", "application/json;charset=utf8")
	if err != nil {
		return
		fmt.Println(err)
	}
	client.Do(req)
	fmt.Println("Done")
}
