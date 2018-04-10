package main

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"net/http"
	"fmt"
	"github.com/go-redis/redis"
	"crypto/sha256"
	"time"
	"encoding/hex"
	"github.com/express-track/lib"
)

func main() {
	//
	router := gin.Default()
	r1 := router.Group("api/express")
	{
		r1.POST("/query", queryRecords)
		r1.POST("/subscribe",subscribe)
	}
	authorized := router.Group("api/secret", BasicAuthStrategy())
	authorized.GET("/", func(c *gin.Context) {
		// get user, it was set by the BasicAuth middleware
		user := c.MustGet(gin.AuthUserKey).(string)
		fmt.Println(user)
		h := sha256.New()
		h.Write([]byte(user+time.Now().String()))
		finalKey := hex.EncodeToString(h.Sum(nil))
		err := rc.Set(user,finalKey,time.Hour * 1).Err()
		if err != nil {
			c.JSON(http.StatusInternalServerError, "internal server error, please try later")
		}
		c.JSON(http.StatusOK,finalKey)
	})



	router.Run(":"+lib.WebPort)
}

var dbConn *gorm.DB
var rc *redis.Client
func init() {
	dbConn = lib.GetDc()
	if !dbConn.HasTable(&lib.TrackRecord{}) {
		panic("model TrackRecord doesn't exits")
	}
	if !dbConn.HasTable(&lib.TrackTask{}) {
		panic("table TrackTask doesn't exist")
	}

	if !dbConn.HasTable(&lib.SubscribeTask{}) {
		panic("table SubscribeTask doesn't exist")
	}

	if !dbConn.HasTable(&lib.Channel{}) {
		panic("table Channel doesn't exist")
	}
	rc = redis.NewClient(&redis.Options{
		Addr:"localhost:6379",
		Password: "",
		DB: 0,
	})

}



func BasicAuthStrategy() gin.HandlerFunc{
	var users []lib.User
	dbConn.Find(&users)
	accounts := make(gin.Accounts)
	if len(users) != 0 {
		for _, user := range users{
			accounts[user.Username] = user.Password
		}
	}
	return gin.BasicAuth(accounts)

}

func queryRecords(c *gin.Context) {
	if !Auth(c) {
		c.JSON(http.StatusUnauthorized,gin.H{"status":http.StatusUnauthorized, "message":"unauthorized"})
		return
	}
	expressNumber := c.PostForm("express_number")
	if expressNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "invalid param express_number"})
		return
	}
	//c.JSON(http.StatusOK,gin.H{"express_number":expressNumber})
	//return
	var trackRecords []lib.TrackRecord
	dbConn.Where("express_number = ?", expressNumber).Find(&trackRecords)
	//fmt.Printf("trackRecords are %v\n and the length is %d",trackRecords, len(trackRecords))
	if len(trackRecords) <= 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "No track records found"})
	}
	var records []lib.Record
	for _, item := range trackRecords {
		records = append(records, lib.Record{Content: item.Content, Status: item.TrackStatus, EventTime: item.EventTime})
	}
	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "records": records, "express_number": expressNumber})
}

func subscribe(c *gin.Context) {
	if !Auth(c)  {
		c.JSON(http.StatusUnauthorized,gin.H{"status":http.StatusUnauthorized, "message":"unauthorized"})
		return
	}
	expressNumber := c.PostForm("express_number")
	channelName := c.PostForm("channel_name")
	url := c.PostForm("url")
	if expressNumber == "" || channelName == "" || url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "invalid param express_number or channel_name or url"})
		return
	}

	var channel lib.Channel
	dbConn.Where("channel_name = ?", channelName).Find(&channel)
	if (lib.Channel{}) == channel {
		//如果没有找到对应的渠道，则返回一个错误
		c.JSON(http.StatusBadRequest, gin.H{"status":http.StatusBadRequest, "message":"unsupported channel"})
		return
	}
	var trackTask lib.TrackTask
	dbConn.Where("express_number = ? and channel_id = ?", expressNumber, channel.ChannelId).Find(&trackTask)
	var subscribeTask lib.SubscribeTask
	if (lib.TrackTask{}) != trackTask {
		//如果找到了对应的渠道,检查下状态,0是初始化,1是已经完成,2是被丢弃
		//只有当状态为初始化的时候才可以继续订阅
		fmt.Println(trackTask)
		if trackTask.TrackStatus != 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status":http.StatusBadRequest, "message":"inactive express_number, subscription failure"})
			return
		}
		//现在只用添加订阅任务就行
		dbConn.Where("track_id = ? and url = ?", trackTask.TrackId, url).Find(&subscribeTask)
		if (lib.SubscribeTask{}) != subscribeTask {
			//找到了重复的订阅任务,提示不要重复订阅
			c.JSON(http.StatusOK, gin.H{"status":http.StatusOK,"message":"you've subscribed, do not repeat"})
			return
		}
		fmt.Println(subscribeTask)
		subscribeTask.TrackId = trackTask.TrackId
		subscribeTask.Url = url
		dbConn.Create(&subscribeTask)
		if subscribeTask.SubscribeId == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "internal server error"})
			return
		} else {
			c.JSON(http.StatusOK, gin.H{"status":http.StatusOK, "message":"subscribed successfully"})
			return
		}
	} else{
		//如果没有找到记录就先创建追踪任务再创建订阅任务，用事务包裹
		ts := dbConn.Begin()
		trackTask.ExpressNumber = expressNumber
		trackTask.ChannelId = channel.ChannelId
		ts.Create(&trackTask)
		if trackTask.TrackId == 0 {
			ts.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status":http.StatusInternalServerError, "message":"can't create track, please try later"})
			return
		}
		//追踪任务未创建时订阅任务也未建立,无需重复检查
		subscribeTask.TrackId = trackTask.TrackId
		subscribeTask.Url = url
		ts.Create(&subscribeTask)
		if subscribeTask.SubscribeId == 0 {
			ts.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status":http.StatusInternalServerError, "message":"can't create subscription, please try later"})
			return
		}
		ts.Commit()
		c.JSON(http.StatusOK, gin.H{"status":http.StatusOK,"message":"create subscription successfully"})
	}


}

func Auth(c *gin.Context) bool{
	authKey := c.PostForm("key")
	user := c.PostForm("user")
	storedKey, err := rc.Get(user).Result()
	//满足所有条件才确认为真
	return !(err != nil || authKey == "" || authKey != storedKey)
}
