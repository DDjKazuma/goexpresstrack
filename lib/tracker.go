package lib

import (
	"io/ioutil"
	"regexp"
	"time"
	"fmt"
	"net/http"
	"net/url"
	"bytes"

)
type (

	Record struct {
		Status    uint8  `json:"status"`
		Content   string `json:"content"`
		EventTime string `json:"eventTime"`
	}

	GdexTracker struct {
		TrackTask TrackTask

	}

	AbxTracker struct {
		TrackTask TrackTask
	}

	Tracker interface {
		Track() ([]byte, error)
		Parse(*[]byte, string, string, uint32) ([]TrackRecord)
		//FilterResponse()
		//checkIfSigned()
	}
)


func (gdex GdexTracker) Track() ([]byte, error) {
	client := &http.Client{
		//CheckRedirect:redirectPolicyFunc,
	}

	//resp, err := client.Get("http://baidu.com")
	req, err := http.NewRequest("GET", "http://hk.kerryexpress.com/track?track="+gdex.TrackTask.ExpressNumber, nil)
	if err != nil {
		//panic("can't init the request")
		return nil, err
	}

	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
	resp, err := client.Do(req)
	//defer req.Body.Close()
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	return body, err
	//fmt.Printf("%s", body)
}

func (gdex GdexTracker) Parse(respBodyPt *[]byte, deadTime string, expressNumber string, trackId uint32) ([]TrackRecord) {
	re, err := regexp.Compile(`(?s)<li>\s*?<time.*?<span>(?P<time>[\d/]+?)</span>[\n\s]+?<span>(?P<attach>.+?)</span></time>.*?<h4>(?P<status>.*?)</h4>.*?</li>`)
	if err != nil {
		return nil
	}
	result := re.FindAllStringSubmatch(string(*respBodyPt), -1)
	if result == nil {
		return nil
	}
	var parsedDeadTime time.Time
	if deadTime == "" {
		parsedDeadTime = time.Time{}
	} else {
		parsedDeadTime, err = time.Parse("2006-01-02 15:04:05", deadTime)
		if err != nil {
			fmt.Println(err)
		}
	}
	trackRecords := make([]TrackRecord, 0)
	isSigned := uint8(0)
	for _, record := range result {
		//htmlTagReg, err := regexp.Compile(`<[/]?\w+?>`)
		//if err != nil {
		//	Record[1] := htmlTagReg.ReplaceAll(Record[1], ``)
		//}
		//fmt.Println(Record[2], Record[1])
		accurateTime, err := time.Parse("02/1/2006 " + record[2], record[1] + " " + record[2])
		if err != nil {
			fmt.Println(err)
			continue
		}
		if !parsedDeadTime.Before(accurateTime){
			continue
		}
		if isSigned == 0{
			re, err := regexp.Compile(`(?i)delivered`)
			if err != nil{
				continue
			}
			if re.MatchString(record[3]) {
				isSigned = 1//1表示已经签收
			}
		}
		formattedTime := accurateTime.Format("2006-01-02 15:04:05")
		if isSigned == 1 {
			trackRecords = append(trackRecords, TrackRecord{EventTime: formattedTime, Content: record[3], ExpressNumber: expressNumber, TrackId: trackId, TrackStatus: 1})
		}else{
			trackRecords = append(trackRecords, TrackRecord{EventTime: formattedTime, Content: record[3], ExpressNumber: expressNumber, TrackId: trackId, TrackStatus: 0})
		}
		if isSigned == 1{
			isSigned = 2//状态为2表示签收且被修改，不需再检查
		}

	}
	return trackRecords
}

//func (gdex GdexTracker) FilterResponse(){
//
//}



func (abx AbxTracker) Track() ([]byte, error) {
	client := &http.Client{

	}
	form := url.Values{
		"capture":    {abx.TrackTask.ExpressNumber},
		"redoc_gdex": {"cnGdex"},
		"Submit":     {"Track"},
	}

	requestBody := bytes.NewBufferString(form.Encode())
	req, err := http.NewRequest("POST", "http://web2.gdexpress.com/official/iframe/etracking_v2.php", requestBody)
	if err != nil {
		return nil, err
	}
	//defer req.Body.Close()
	req.Header.Add("content-type", "application/x-www-form-urlencoded;charset=UTF-8")
	resp, err := client.Do(req)
	//defer req.Body.Close()
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	return body, err
}

func (abx AbxTracker) Parse(respBodyPt *[]byte, deadTime string, expressNumber string, trackId uint32) ([]TrackRecord) {
	re, err := regexp.Compile(`(?s)<tr bgcolor=.*?<td>.*?</td>\s*?<td>(?P<time>[\d\W]+?)</td>\s*?<td>(?P<status>.+?)</td>\s*?<td>.*?</td>\s*?</tr>`)
	if err != nil {
		//panic("wrong regular expression")
		return nil
	}
	result := re.FindAllStringSubmatch(string(*respBodyPt), -1)
	if result == nil {
		return nil
	}
	trackRecords := make([]TrackRecord, 0)
	var parsedDeadTime time.Time
	if deadTime != "" {
		parsedDeadTime, err = time.Parse("2006-01-02 15:04:05", deadTime)
		if err != nil {
			fmt.Println(err)
		}
	}else {
		parsedDeadTime = time.Time{}
	}
	isSigned := 0
	for _, record := range result {
		accurateTime, err := time.Parse("02/01/2006 15:04:05", record[1])
		if err != nil {
			fmt.Println(err)
			continue
		}
		if !parsedDeadTime.Before(accurateTime) {
			continue
		}
		if isSigned == 0{
			re, err := regexp.Compile(`(?i)delivered`)
			if err != nil{
				continue
			}
			if re.MatchString(record[2]) {
				isSigned = 1
			}
		}
		formattedTime := accurateTime.Format("2006-01-02 15:04:05")
		if isSigned == 1 {
			trackRecords = append(trackRecords, TrackRecord{EventTime: formattedTime, Content: record[2], ExpressNumber: expressNumber, TrackId: trackId, TrackStatus:1})
		}else{
			trackRecords = append(trackRecords, TrackRecord{EventTime: formattedTime, Content: record[2], ExpressNumber: expressNumber, TrackId: trackId})
		}
		if isSigned == 1{
			isSigned = 2
		}
	}
	return trackRecords
}