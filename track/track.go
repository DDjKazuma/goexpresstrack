package main

import (
	"fmt"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"time"
	"sync"
	"github.com/express-track/lib"
)

func main() {
	t1 := time.Now()
	var trackTasks []lib.TrackTask
	dbConn := lib.GetDc()
	dbConn.Where("track_status = ?", lib.TrackInited).Find(&trackTasks)
	//
	//dbConn.Debug().Where("track_status = 0").Find(&trackTasks)
	if len(trackTasks) == 0 {
		fmt.Println("no tasks to be tracked")
		return
	}
	//trackParamsList := make([]TrackParams, 0)
	ct := make(chan lib.TrackTask, len(trackTasks))
	cr := make(chan []lib.TrackRecord, len(trackTasks))
	wg := new(sync.WaitGroup)
	println(len(trackTasks))
	threadsCount := len(trackTasks) / 4
	wg.Add(threadsCount)
	for _, trackTask := range trackTasks {
		ct <- trackTask
	}
	close(ct)
	for i := 0; i < threadsCount; i++ {
		//println("start get")
		go trackOne(wg, ct, cr)
	}
	wg.Wait()
	close(cr)
	for {
		trackRecords, isOpen := <- cr
		if !isOpen {
			break
		}
		//fmt.Println(trackRecords)
		parsedLatestEventTime := time.Time{}
		var records []lib.Record
		isSigned := uint8(0)
		for _, trackRecord := range trackRecords{
			//dbConn.Create(trackRecord)
			parsedTmpTime, err := time.Parse("2006-01-02 15:04:05", trackRecord.EventTime)
			if err != nil {
				continue
			}
			if parsedTmpTime.After(parsedLatestEventTime){
				parsedLatestEventTime = parsedTmpTime
			}
			dbConn.Create(&trackRecord)
			if trackRecord.TrackStatus == 1{
				isSigned = 1
			}
			records = append(records, lib.Record{Status:trackRecord.TrackStatus,Content:trackRecord.Content,EventTime:trackRecord.EventTime})
		}
		//curTask := lib.TrackTask{TrackId:trackRecords[0].TrackId}
		fmt.Println("start")
		fmt.Println(parsedLatestEventTime.Format("2006-01-02 15:04:05"))
		fmt.Println("down")
		dbConn.Debug().Table("track_task").Where("track_id = ? ", trackRecords[0].TrackId).Updates(map[string]interface{}{"latest_track_time": parsedLatestEventTime.Format("2006-01-02 15:04:05"), "track_status":isSigned})
		//fmt.Println(curTask)
		//新的记录可以即时发送给url
		//查找所属的任务
		//var subscribeTasks []lib.SubscribeTask
		//dbConn.Where("track_id = ?", curTask.TrackId).Find(&subscribeTasks)
		//pushLatestRecords(records, subscribeTasks)
	}
	interval := time.Since(t1)
	fmt.Println("totalTime is ", interval)

}





//从信道中逐个取出追踪任务抓取结果，并发操作
func trackOne(wg *sync.WaitGroup, channelIn chan lib.TrackTask, channelOut chan []lib.TrackRecord) {
	defer wg.Done()
	for {
		trackTask, isOpen := <-channelIn
		//fmt.Println(trackParams, isOpen)

		if !isOpen {
			//fmt.Println("now the channel is closed")
			break
		}
		tracker, err := dispatchTracker(trackTask)
		if err != nil {
			fmt.Println("can't dispatch tracker with express_number", trackTask.ExpressNumber)
			continue
		}
		res, err := tracker.Track()
		if err != nil {
			fmt.Println("can't get track result with express_number ", trackTask.ExpressNumber)
			continue
		}
		formedRecords := tracker.Parse(&res, trackTask.LatestTrackTime, trackTask.ExpressNumber, trackTask.TrackId)
		if len(formedRecords) > 0 {
			for _, record := range formedRecords {
				fmt.Println(record)
			}
			channelOut <- formedRecords
		} else{
			fmt.Println("no new records")
		}
	}
	fmt.Println("thread Done!")

}


//为指定的任务分配一个追踪接口
func dispatchTracker(trackTask lib.TrackTask) (lib.Tracker, error) {
	switch trackTask.ChannelId {
	case 5:
		gdex := lib.GdexTracker{TrackTask:trackTask}
		return gdex, nil
	case 33:
		abx := lib.AbxTracker{TrackTask:trackTask}
		return abx, nil
	default:
		return nil, fmt.Errorf("unresolved channelId %d", trackTask.ChannelId)
	}
}

