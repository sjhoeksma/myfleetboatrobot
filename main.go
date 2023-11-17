package main

//lint:file-ignore ST1005 Ignore message error should not be started with capital

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mdp/qrterminal"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var AppVersion = "0.7.1"                      //The version of application
var AppName = "MyFleetRobot"                  //The Application name
var myFleetVersion = "R1B34"                  //The software version of myFleet
var clubId = "rvs"                            //The club code
const dbPath = "db/"                          //The location where datafiles are stored
const bookingFile = dbPath + "booking.json"   //The json file to store bookings in
const boatNameFile = dbPath + "boats.json"    //The json file to store boats
const boatFile = dbPath + "boatdata.json"     //The json file to store boats
const userFile = dbPath + "users.json"        //The json file to store users
const whatsAppFile = dbPath + "whatsapp.json" //The json file to store whatsapp info
const teamFile = dbPath + "teams.json"        //The json file to store group info
const dbFile = dbPath + "myfleetrobot.db"     //The db file to store whatsapp sessions
const versionFile = dbPath + "version.json"   //The  file to store version info
var timeZoneLoc = "Europe/Amsterdam"          //The time zone location for the club
var timeZone = ""                             //The time zone in hour, is also calculated
var minDuration = 60                          //The minimal duration required to book
var maxDuration = 120                         //The maximal duration allowed to book
var bookWindow = 48                           //The number of hours allowed to book
var confirmTime = 0                           //Time in Min before before starting time to confirm booking, 0=Disabled
var maxRetry int = 100                        //The maximum numbers of retry before we give up, 0=disabled
var refreshInterval int = 1                   //We do a check of the database every 1 minute
var logLevel string = "Info"                  //Default loglevel is info
var logFile string                            //Should we log to file
var whatsApp bool = true                      //Should we enable watchapp
var planner bool = true                       //Should we enable planner

// Used to map specify when to send a whatsapp message
var sendWhatsAppMsg = map[string]bool{
	"Finished":  true,
	"Blocked":   true,
	"Failed":    true,
	"Confirmed": true,
	/*
		"Canceled": false,
		"Retry":    false,
		"Delete":   false,
		"Waiting":  false,
		"Cancel":   false,
		"Moving":   false,
	*/
}

type RepeatType int

// Repeat 0=None, 1=Daily, 2=Weekly, 3=Monthly, 4=Yearly
const (
	None RepeatType = iota
	Daily
	Weekly
	Monthly
	Yearly
)

// Struc used to store user info
type UserInterface struct {
	Id       int64  `db:"id" json:"id"`
	Team     string `db:"team" json:"team"`
	Username string `db:"user" json:"user"`
	Password string `db:"password" json:"password"`
	Name     string `db:"name" json:"name"`
	LastUsed int64  `db:"lastused" json:"lastused"`
}

type LoginInterface struct {
	Team     string `db:"team" json:"team"`
	Password string `db:"password" json:"password"`
	Status   string `db:"-" json:"status,omitempty"`
}

// Struc used to store user info
type WhatsAppToInterface struct {
	Team     string `db:"team" json:"team"`
	To       string `db:"msgto" json:"to"`
	LastUsed int64  `db:"lastused" json:"lastused"`
}

type TeamInterface struct {
	Id         int64  `db:"id" json:"id"`
	Team       string `db:"team" json:"team"`
	Admin      bool   `db:"admin" json:"admin"`
	Password   string `db:"password" json:"password"`
	Title      string `db:"title" json:"title"`
	AddTime    bool   `db:"addtime" json:"addtime"`
	WhatsApp   bool   `db:"whatsapp" json:"whatsapp"`
	WhatsAppId string `db:"whatsappid" json:"whatsappid"`
	WhatsAppTo string `db:"whatsappto" json:"whatsappto"`
	QRCode     string `db:"-" json:"qrcode"`
	Prefix     string `db:"prefix" json:"prefix"`
	Planner    bool   `db:"planner" json:"planner"`
}

// Used to store version info
type VersionInterface struct {
	Version string `db:"version" json:"version"`
}

type ActivityInterface struct {
	Id        int64      `db:"id" json:"id"`
	Team      string     `db:"team" json:"team"`
	StartDate string     `db:"startdate" json:"startdate"`
	EndDate   string     `db:"enddate" json:"enddate"`
	Time      string     `db:"time" json:"time"`
	Duration  int64      `db:"duration" json:"duration"`
	Repeat    RepeatType `db:"repeat" json:"repeat"`
}

type BoatElementBookingStruct struct {
	Type        string `json:"type"`     // =R.s
	EpochStart  int64  `json:"start"`    //Start + (x/12)*15
	EpochEnd    int64  `json:"end"`      //Start +(x+w)/12*15
	Duration    int64  `json:"duration"` //(x+w)/12 in min
	BookingId   string `json:"bookingId,omitempty"`
	BookingInfo string `json:"bookingInfo,omitempty"`
}

type BoatElementStruct struct {
	Id          int                        `json:"id"`          // BoatId
	Name        string                     `json:"name"`        //BoatName BoatInfo[0]
	Type        string                     `json:"type"`        //BoatType BoatInfo[1]
	Location    string                     `json:"location"`    //BoatLocation BoatInfo[2]
	WeigthClass string                     `json:"weightclass"` //WeigthClass BoatInfo[3]
	Permission  string                     `json:"permission"`  //Permission BoatInfo[5]
	Bookings    []BoatElementBookingStruct `json:"bookings"`
}
type BoatListStruct []BoatElementStruct

type LogStruct struct {
	Date  int64  `json:"date"`  // Date
	State string `json:"state"` // State
	Log   string `json:"log"`   //Log
}
type LogListStruct []LogStruct

// Struc used to store boat and session info
type BookingInterface struct {
	Id            int64           `db:"id" json:"id"`
	Team          string          `db:"team" json:"team"`
	Name          string          `db:"boat" json:"boat"`
	Fallback      string          `db:"fallback" json:"fallback,omitempty"`
	Date          string          `db:"date" json:"date"`
	Time          string          `db:"time" json:"time"`
	Duration      int64           `db:"duration" json:"duration"`
	Username      string          `db:"user" json:"user"`
	Password      string          `db:"password" json:"password"`
	Comment       string          `db:"comment" json:"comment"`
	Repeat        RepeatType      `db:"repeat" json:"repeat,omitempty"`
	State         string          `db:"state" json:"state,omitempty"`
	BookingId     string          `db:"bookingid" json:"bookingid,omitempty"`
	BoatId        string          `db:"boatid" json:"boatid,omitempty"`
	Message       string          `db:"message" json:"message,omitempty"`
	EpochNext     int64           `db:"epochnext" json:"next,omitempty"`
	Retry         int             `db:"retrycounter" json:"retry,omitempty"`
	UserComment   bool            `db:"usercomment" json:"usercomment"`
	WhatsAppTo    string          `db:"whatsapp" json:"whatsapp,omitempty"`
	BookStart     int64           `db:"bookstart" json:"bookstart,omitempty"`
	BookDur       int64           `db:"bookdur" json:"bookdur,omitempty"`
	Logs          LogListStruct   `db:"logs" json:"logs,omitempty"`
	TimeZone      string          `db:"-" json:"-"`
	Boats         *BoatListStruct `db:"-" json:"-"`
	GuiEpochStart int64           `db:"-" json:"-"`
	GuiFleetId    string          `db:"-" json:"-"`
	Cookies       []*http.Cookie  `db:"-" json:"-"`
	EpochDate     int64           `db:"-" json:"-"`
	EpochStart    int64           `db:"-" json:"-"`
	EpochEnd      int64           `db:"-" json:"-"`
	Changed       bool            `db:"-" json:"-"`
	UserId        int64           `db:"-" json:"-"`
}

// The list of bookings
type BookingSlice []BookingInterface

var singleRun bool = false                //Should we do a single runonly = nowebserver
var commentPrefix string = ""             //The prefix we use as a comment indicator the booking is ours
var bindAddress string = ":1323"          //The default bind port of web server
var jsonTeam string                       //The Basic Auth team of webserer
var jsonPwd string                        //The Basic Auth password of webserver
var jsonProtect bool                      //Should the web server use Basic Auth
var textUrl string                        //Default backend url
var guiUrl string                         //The gui url towards the fleet.eu backend
var authUrl string                        //Backend url towards authentication
var test string = ""                      //The test we should be running, means allways single ru
var title string = ""                     //The title string
var mutex *sync.Mutex = &sync.Mutex{}     //The lock used where writing files
var whatsAppLog waLog.Logger              //WhatsApp logger
var teams []TeamInterface                 //The security teams
var db *sql.DB                            //The Database
var whatsAppContainer *sqlstore.Container //The whatsapp datacontainer
var dbType string = "sqlite3"             //The Database type

// Simple iif function to select value
func iif(a, b string) string {
	if a == "" {
		return b
	} else {
		return a
	}
}

// Simple conditional if function to select value
func cif(c bool, a, b string) string {
	if c {
		return a
	} else {
		return b
	}
}

// Find min of 2 int64 values
func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// Find max of 2 int64 values
func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func sliceVersion(version string) [3]uint32 {
	sa := strings.Split(version, ".")
	var si [3]uint32
	for j, a := range sa {
		i, err := strconv.Atoi(a)
		if err == nil {
			si[j] = uint32(i)
		}
		if j == 2 {
			break
		}
	}
	return si
}

// Function to get an ENV variable and put it into a string
func setEnvValue(key string, item *string) {
	s := os.Getenv(key)
	if s != "" {
		*item = s
	}
}

// Function to get an ENV variable and put it into a string
func setEnvBoolValue(key string, item *bool) {
	s := os.Getenv(key)
	if s != "" {
		*item, _ = strconv.ParseBool(s)
	}
}

// Make from a long date string a short one
func shortDate(date string) string {
	return strings.Split(date, "T")[0]
}

// Make from a long data string as short time
func shortTime(timeS string) string {
	if strings.Contains(timeS, "T") {
		thetime, _ := time.Parse(time.RFC3339, timeS)
		loc, _ := time.LoadLocation(timeZoneLoc)
		return thetime.Round(15 * time.Minute).In(loc).Format("15:04")
	}
	thetime, _ := time.Parse(time.RFC3339, "2001-01-01"+"T"+timeS+":00+00:00")
	return thetime.Round(15 * time.Minute).Format("15:04")
}

// Function to filter out the valid teams from array
func TeamFilter(arr interface{}, teamName string) interface{} {
	contentType := reflect.TypeOf(arr)
	contentValue := reflect.ValueOf(arr)
	newContent := reflect.MakeSlice(contentType, 0, 0)
	if contentValue.Len() != 0 {
		f := 0
		t := contentValue.Index(0).Type()
		for ; f < t.NumField(); f++ {
			if t.Field(f).Name == "Team" {
				break
			}
		}
		if f <= t.NumField() {
			for i := 0; i < contentValue.Len(); i++ {
				if content := contentValue.Index(i); content.Field(f).Interface() == teamName {
					newContent = reflect.Append(newContent, content)
				}
			}
		}
	}
	return newContent.Interface()
}

func Upgrade() {
	var v VersionInterface = VersionInterface{Version: AppVersion}
	if _, err := os.Stat(versionFile); !errors.Is(err, os.ErrNotExist) {
		file, _ := os.ReadFile(bookingFile)
		json.Unmarshal(file, &v)
	}
	if v.Version != AppVersion {
		//Add code here to do a upgrade, step by step for each version

		//Upgrade finished Write the new version number
		v.Version = AppVersion
		json_to_file, _ := json.Marshal(v)
		os.WriteFile(versionFile, json_to_file, 0755)
	}
}

func updateTimeZone() {
	//Get the local time zone
	loc, err := time.LoadLocation(timeZoneLoc)
	if err != nil {
		log.Fatal(err)
	}
	tz := time.Now().In(loc).Format("-07:00")
	if tz != timeZone {
		log.Info("TimeZone: " + tz)
		timeZone = tz
	}
}

// Read and set settings
func Init() {
	setEnvValue("JSONTEAM", &jsonTeam)
	setEnvValue("JSONPWD", &jsonPwd)
	setEnvValue("PREFIX", &commentPrefix)
	setEnvValue("TIMEZONE", &timeZone)
	setEnvValue("CLUBID", &clubId)
	setEnvValue("FLEETVERSION", &myFleetVersion)
	setEnvValue("LOGLEVEL", &logLevel)
	setEnvValue("TITLE", &title)
	setEnvBoolValue("WHATSAPP", &whatsApp)
	setEnvBoolValue("PLANNER", &planner)

	version := flag.Bool("version", false, "Prints current version ("+AppVersion+")")
	flag.BoolVar(&singleRun, "singleRun", singleRun, "Should we only do one run")
	flag.StringVar(&commentPrefix, "prefix", commentPrefix, "Comment prefix")
	flag.StringVar(&timeZoneLoc, "timezone", timeZoneLoc, "The timezone location used by user")
	flag.IntVar(&refreshInterval, "refresh", refreshInterval, "The iterval in seconds used for refeshing")
	flag.IntVar(&bookWindow, "bookWindow", bookWindow, "The interval in hours for allowed bookings")
	flag.IntVar(&maxRetry, "maxRetry", maxRetry, "The maximum retry's before failing, 0=disabled")
	flag.IntVar(&confirmTime, "confirmTime", confirmTime, "The time before confirming, 0=disabled")
	flag.StringVar(&bindAddress, "bind", bindAddress, "The bind address to be used for webserver")
	flag.StringVar(&jsonTeam, "jsonTeam", jsonTeam, "The team name to protect jsondata")
	flag.StringVar(&jsonPwd, "jsonPwd", jsonPwd, "The password to protect jsondata")
	flag.StringVar(&clubId, "clubId", clubId, "The clubId used")
	flag.StringVar(&myFleetVersion, "fleetVersion", myFleetVersion, "The version of the myFleet software to use")
	flag.StringVar(&logLevel, "logLevel", logLevel, "The log level to use")
	flag.StringVar(&title, "title", title, "The title to use in app")
	flag.StringVar(&logFile, "logFile", logFile, "The logFile where we should write log information to")
	flag.BoolVar(&whatsApp, "whatsApp", whatsApp, "Should we use WhatsApp to send a message")
	flag.BoolVar(&planner, "planner", planner, "Should we use planner")
	flag.StringVar(&test, "test", test, "The test action to perform")

	flag.Parse() // after declaring flags we need to call it
	if *version {
		log.Info("Version ", AppVersion)
		os.Exit(0)
	}
	//When test action is specified we are allways in singlerun
	if test != "" {
		singleRun = true
	}

	//Setup the logging
	if logFile != "" {
		file, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		log.SetOutput(file)
	}
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}
	log.SetFormatter(&log.TextFormatter{DisableColors: false, FullTimestamp: true})
	//log.SetFormatter(&log.JSONFormatter{DisableColors: false, FullTimestamp: true,})

	//Only enable jsonProtection if we have a username and password
	jsonProtect = (jsonTeam != "" && jsonPwd != "")
	guiUrl = "https://my-fleet.eu/" + myFleetVersion + "/gui/index.php"
	textUrl = "https://my-fleet.eu/" + myFleetVersion + "/text/index.php"
	authUrl = "https://my-fleet.eu/" + myFleetVersion + "/text/authenticate.php"

	//Get the local time zone
	updateTimeZone()

	//Create the db path if it does not exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(dbPath), 0755)
		if err != nil {
			log.Fatal(err)
		}
	}
	//Read the teams once, are also read on every login
	teams = readTeamJson()
	//If we have a teams file enable jsonProtect
	if len(teams) != 0 {
		jsonProtect = true
	}

	//Log the version
	log.Info(AppName + " v" + AppVersion)
}

// Logout for the specified booking
func logout(booking *BookingInterface) error {
	//Just check if we have a session
	if booking.Cookies != nil {
		var random string = fmt.Sprint(time.Now().Nanosecond())
		//Calling auth with new random will kill the sessie
		request, _ := http.NewRequest(http.MethodGet, authUrl+"?random="+random, nil)
		for _, o := range booking.Cookies {
			request.AddCookie(o)
		}
		client := &http.Client{}
		client.Do(request)
	}
	booking.Cookies = nil
	return nil
}

func session(booking *BookingInterface) error {
	//We use the textUrl and guiUrl to get the session cookie
	response, err := http.Get(textUrl + "?clubname=" + clubId + "&variant=")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	booking.Cookies = response.Cookies()
	response, err = http.Get(guiUrl + "?clubname=" + clubId + "&variant=")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	booking.Cookies = append(booking.Cookies, response.Cookies()...)

	//Get the GuiStartEpoch and GuiFleetId
	request, _ := http.NewRequest(http.MethodGet, guiUrl+"?clubname="+clubId, nil)
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	client := &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	b, _ := io.ReadAll(response.Body)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(string(b)))
	// Find the sunrise, sunset, min and max times allowed
	doc.Find("form").Each(func(base int, basehtml *goquery.Selection) {
		basehtml.Find("input").Each(func(baseint int, basein *goquery.Selection) {
			val, exists := basein.Attr("name")
			if exists && val == "start" {
				val, exists = basein.Attr("value")
				if exists {
					theTime, err := time.Parse(time.RFC3339, (strings.Fields(val)[0])+"T"+(strings.Fields(val)[1])+":00"+timeZone)
					if err == nil {
						booking.GuiEpochStart = theTime.Unix()
					}
				}
			}
		})
	})

	doc.Find("link").Each(func(base int, basehtml *goquery.Selection) {
		_, exists := basehtml.Attr("media")
		val, exists2 := basehtml.Attr("href")
		if exists && exists2 { //href=index.php?a=i&uniq=myfleet62e7e8ea838ba
			re := regexp.MustCompile(`.*uniq=(.*)$`)
			booking.GuiFleetId = re.FindStringSubmatch(val)[1]
		}
	})
	if booking.GuiFleetId == "" || booking.GuiEpochStart == 0 {
		return errors.New("GuiFleetId or GuiEpochStart not found")
	}
	//log.Info("GuiEpochStart:", booking.GuiEpochStart)
	return nil
}

// Login for the specified booking and save the required cookie
func login(booking *BookingInterface) error {
	random := fmt.Sprint(time.Now().Nanosecond())
	if err := session(booking); err != nil {
		return err
	}

	//First get authentication killling old session
	request, _ := http.NewRequest(http.MethodGet, authUrl+"?random="+random, nil)
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}
	//Now post the our info to login
	data := url.Values{}
	data.Set("un", booking.Username)
	data.Set("pw", booking.Password)
	request, _ = http.NewRequest(http.MethodPost, authUrl+"?random="+random,
		strings.NewReader(data.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		log.Error(err)
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}
	b, _ := io.ReadAll(response.Body)
	//Correct loging
	if !strings.Contains(string(b), "Exit Page") {
		return errors.New("Login response invalid:" + string(b))
	}

	//Now get the user ID
	request, _ = http.NewRequest(http.MethodGet, textUrl+"?clubname="+clubId+"&variant=", nil)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		log.Error(err)
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}
	b, _ = io.ReadAll(response.Body)
	re := regexp.MustCompile(`brsuser=(.*)"`)
	rem := re.FindStringSubmatch(string(b))
	booking.UserId = 0
	if len(rem) > 0 {
		booking.UserId, _ = strconv.ParseInt(rem[1], 10, 64)
	}
	if booking.UserId == 0 {
		return errors.New("User id not found")
	}

	//Get the new GuiFleedId
	request, _ = http.NewRequest(http.MethodGet, guiUrl+"?language=NL&brsuser="+strconv.FormatInt(booking.UserId, 10)+"&clubname="+clubId, nil)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		log.Error(err)
		return err
	}
	defer response.Body.Close()
	bs, _ := io.ReadAll(response.Body)
	booking.Cookies = append(booking.Cookies, response.Cookies()...)

	re = regexp.MustCompile(`&uniq=(.*)"`)
	rem = re.FindStringSubmatch(string(bs))
	if len(rem) == 0 {
		return errors.New("GuiFleedId not found")
	}
	booking.GuiFleetId = rem[1]

	return nil

}

// Cancel a booking
func boatCancel(booking *BookingInterface) error {
	var err error
	//STEP: Create Reference to the booking
	request, err := http.NewRequest(http.MethodGet, guiUrl, nil)
	values := request.URL.Query()
	values.Set("a", "e")
	values.Set("menu", "Omenu")
	values.Set("extrainfo", "mid="+booking.BoatId+
		"&co=0&rid="+booking.BookingId+
		"&from="+strconv.FormatInt(int64((booking.BookStart-booking.GuiEpochStart)/(15*60)), 10)+
		"&dur="+strconv.FormatInt(int64(booking.BookDur/15), 10)+"&rec=0&user="+strconv.FormatInt(booking.UserId, 10))
	request.URL.RawQuery = values.Encode()
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	if err != nil {
		return err
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}

	//Step 2: Login to the reference
	data := url.Values{}
	data.Set("newStart", strconv.FormatInt(int64((booking.BookStart-booking.GuiEpochStart)/(15*60)), 10))
	data.Set("newEnd", strconv.FormatInt(int64((booking.BookStart-booking.GuiEpochStart+booking.BookDur*60)/(15*60)), 10))
	//	data.Set("clubcode", "")
	//	data.Set("username", booking.Username)
	//	data.Set("password", booking.Password)
	request, _ = http.NewRequest(http.MethodPost, guiUrl+"?a=e&menu=Rmenu&page=1_cancel", strings.NewReader(data.Encode()))
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}

	booking.State = "Canceled"
	booking.BookDur = 0
	booking.BookStart = 0
	return err
}

// Confirm a booking
func confirmBoat(booking *BookingInterface) error {
	log.Error("Confirm Boat not implemented")
	return nil
}

// Update a boat booking start and end time
func boatBook(booking *BookingInterface, startTime int64, endTime int64) error {
	//STEP: Session
	booking.BookingId = ""

	//STEP: Create Reference to the booking
	request, err := http.NewRequest(http.MethodGet, guiUrl, nil)
	values := request.URL.Query()
	values.Set("a", "e")
	values.Set("menu", "Amenu")
	values.Set("extrainfo", "mid="+booking.BoatId+
		"&from="+strconv.FormatInt(int64((startTime-booking.GuiEpochStart)/(15*60)), 10)+
		"&dur="+strconv.FormatInt(int64(((endTime-startTime)/60)/15), 10))
	request.URL.RawQuery = values.Encode()
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	if err != nil {
		return err
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}

	//Step 2: Login to the reference
	data := url.Values{}
	data.Set("newStart", strconv.FormatInt(int64((startTime-booking.GuiEpochStart)/(15*60)), 10))
	data.Set("newEnd", strconv.FormatInt(int64((endTime-booking.GuiEpochStart)/(15*60)), 10))
	team, err := getTeamByName(booking.Team)
	c := cif(err == nil, iif(team.Prefix, commentPrefix), commentPrefix) + booking.Comment
	if c != "" {
		data.Set("comment", c)
	}
	data.Set("act", "Verder\n>>")
	request, _ = http.NewRequest(http.MethodPost, guiUrl+"?a=e&menu=Amenu&page=1_single", strings.NewReader(data.Encode()))
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	//request.Header.Set("Referer", guiUrl+"?"+values.Encode())
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}
	//Check if the boat is the boat we where looking for
	//Read the booking id form the reponse
	b, _ := io.ReadAll(response.Body)
	re := regexp.MustCompile(`ReservationId = (.*) `)
	rem := re.FindStringSubmatch(string(b))
	if len(rem) > 0 {
		booking.BookingId = strings.Trim(rem[1], " ")
	}
	if booking.BookingId == "" {
		return errors.New("Failed to create reservation")
	}
	//Save the real booking start and duration
	booking.BookDur = (endTime - startTime) / 60
	booking.BookStart = startTime
	return nil
}

// Update a boat booking start and end time
func boatUpdate(booking *BookingInterface, startTime int64, endTime int64) error {

	//STEP: Create Reference to the booking
	request, err := http.NewRequest(http.MethodGet, guiUrl, nil)
	values := request.URL.Query()
	values.Set("a", "e")
	values.Set("menu", "Rmenu")
	values.Set("extrainfo", "mid="+booking.BoatId+
		"&co=0&rid="+booking.BookingId+
		"&from="+strconv.FormatInt(int64((startTime-booking.GuiEpochStart)/(15*60)), 10)+
		"&dur="+strconv.FormatInt(int64(((startTime-endTime)/60)/15), 10)+"&rec=0")
	request.URL.RawQuery = values.Encode()
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	if err != nil {
		return err
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}

	//Step 2: Login to the reference
	data := url.Values{}
	data.Set("newStart", strconv.FormatInt(int64((startTime-booking.GuiEpochStart)/(15*60)), 10))
	data.Set("newEnd", strconv.FormatInt(int64((endTime-booking.GuiEpochStart)/(15*60)), 10))
	data.Set("clubcode", "")
	data.Set("username", booking.Username)
	data.Set("password", booking.Password)
	request, _ = http.NewRequest(http.MethodPost, guiUrl+"?a=e&menu=Rmenu&page=1_modifylogbook", strings.NewReader(data.Encode()))
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}
	//Check if the boat is the boat we where looking for

	//STEP: Update the booking
	data = url.Values{}
	data.Set("newStart", strconv.FormatInt(int64((startTime-booking.GuiEpochStart)/(15*60)), 10))
	data.Set("newEnd", strconv.FormatInt(int64((endTime-booking.GuiEpochStart)/(15*60)), 10))
	team, err := getTeamByName(booking.Team)
	c := cif(err == nil, iif(team.Prefix, commentPrefix), commentPrefix) + booking.Comment
	if c != "" {
		data.Set("comment", c)
	}
	data.Set("page", "3_commit")
	data.Set("act", "Ok")
	request, _ = http.NewRequest(http.MethodPost, guiUrl+"?a=e&menu=Amenu", strings.NewReader(data.Encode()))
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}
	//Save the real booking start and duration
	booking.BookDur = (endTime - startTime) / 60
	booking.BookStart = startTime
	return nil
}

func guiAction(booking *BookingInterface, action string) (string, error) {
	request, _ := http.NewRequest(http.MethodGet, guiUrl, nil)
	values := request.URL.Query()
	values.Set("a", action)
	values.Set("uniq", booking.GuiFleetId)
	request.URL.RawQuery = values.Encode()
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return "", err
	}
	bd, _ := io.ReadAll(response.Body)
	return string(bd), nil
}

// Read the boat list and create it if not found
func readBoatJson(book *BookingInterface, maxAge int) ([]string, BoatListStruct) {
	var blist []string
	var fs os.FileInfo
	var err error
	var booking *BookingInterface
	blist = append(blist, "No Boats")
	boats := BoatListStruct{}
	if book == nil {
		booking = &BookingInterface{}
		if err := session(booking); err != nil {
			return blist, boats
		}
		fs, err = os.Stat(boatNameFile)
	} else {
		booking = book
		fs, err = os.Stat(boatFile)
	}
	//We need to check if we have the boat file, load it for the first authorized
	if errors.Is(err, os.ErrNotExist) || fs.ModTime().Before(time.Now().Add(-time.Duration(maxAge)*time.Second)) {
		//Get the unix start time of screen
		str, err := guiAction(booking, "b")
		if err != nil {
			log.Error("GuiSession Failed", err)
			return blist, boats
		}
		epochStart := int64(0)
		re := regexp.MustCompile(`var starttime_unix = "(.*)";`)
		rem := re.FindStringSubmatch(str)
		if len(rem) > 0 {
			epochStart, _ = strconv.ParseInt(rem[1], 10, 64)
		}

		//Get the content of screen
		str, err = guiAction(booking, "c")
		if err != nil {
			log.Error("GuiSession Failed", err)
			return blist, boats
		}
		re = regexp.MustCompile(`var info=(.*);`)
		rem = re.FindStringSubmatch(str)
		if len(rem) > 0 {
			/* Parse the boat list*/
			type BoatStruct []struct {
				M struct {
					P        string   `json:"p"`
					BoatId   int      `json:"i"`
					R        string   `json:"r"`
					A        string   `json:"a"`
					B        string   `json:"b"`
					BoatInfo []string `json:"c"` //  [name,type,location,weigth,spacer,permision]
				} `json:"m"`
				R []struct {
					P  string  `json:"p"`
					S  string  `json:"s"`
					Y  int     `json:"y"`
					X  float64 `json:"x"`
					I  int     `json:"i"`
					R  string  `json:"r"`
					W  float64 `json:"w"`
					U  string  `json:"u"`
					O  string  `json:"o"`
					C  string  `json:"c"`
					ID string  `json:"id"`
				} `json:"r"`
			}
			webboats := BoatStruct{}
			err := json.Unmarshal([]byte(rem[1]), &webboats)
			if err != nil {
				blist = []string{}
				re = regexp.MustCompile(`var grid_width = (.*);`)
				rem = re.FindStringSubmatch(str)
				pixelToMin := float64(12)
				if len(rem) > 0 {
					pixelToMin, _ = strconv.ParseFloat(rem[1], 64)
				}
				for _, b := range webboats {
					//Create a new boat
					bname := strings.Split(b.M.BoatInfo[0], "\u0026")[0]
					bc := BoatElementStruct{Id: b.M.BoatId, Name: bname, Type: b.M.BoatInfo[1],
						Location: b.M.BoatInfo[2], WeigthClass: b.M.BoatInfo[3],
						Permission: strings.Split(b.M.BoatInfo[4], "\u0026")[0]}
					//Add all bookings
					for _, bb := range b.R {
						if (bb.S == "B" || bb.S == "R") && bb.W > 0 {
							//The color code indicaties a window not available
							bbb := BoatElementBookingStruct{
								Type:        cif(bb.C == "#404040", "N", cif(bb.I == -1, "S", bb.S)),
								EpochStart:  epochStart + int64((bb.X/pixelToMin)*(15*60)),
								EpochEnd:    epochStart + int64((bb.X+bb.W)/pixelToMin)*(15*60),
								Duration:    int64(bb.W/pixelToMin) * 15,
								BookingId:   bb.ID,
								BookingInfo: bb.U,
							}
							//Add the booking
							bc.Bookings = append(bc.Bookings, bbb)
						}
					}
					//Save it to the boats list
					boats = append(boats, bc)
					blist = append(blist, bname)
				}
				json_to_file, _ := json.Marshal(boats)
				mutex.Lock()
				if book != nil {
					os.WriteFile(boatFile, json_to_file, 0755)
				}
				json_to_file, _ = json.Marshal(blist)
				os.WriteFile(boatNameFile, json_to_file, 0755)
				mutex.Unlock()

			}
			if err != nil {
				log.Error("Boat list error", err)
			}
		}
		return blist, boats
	}

	file, err := os.ReadFile(boatNameFile)
	if err != nil {
		log.Error(err)
	} else {
		err = json.Unmarshal(file, &blist)
		if err != nil {
			log.Error(err)
		}
	}
	file, err = os.ReadFile(boatFile)
	if err == nil {
		err = json.Unmarshal(file, &boats)
		if err != nil {
			log.Error(err)
		}
	}
	return blist, boats
}

// Read the  user info
func readWhatsAppJson() []WhatsAppToInterface {
	var b []WhatsAppToInterface
	var u WhatsAppToInterface = WhatsAppToInterface{To: "?"}
	b = append(b, u)
	if _, err := os.Stat(whatsAppFile); errors.Is(err, os.ErrNotExist) {
		return b
	}
	file, err := os.ReadFile(whatsAppFile)
	if err == nil {
		err = json.Unmarshal(file, &b)
		if err != nil {
			log.Error(err)
		}
	}
	return b
}

// Write the user info to file
func writeWhatsAppJson(data []WhatsAppToInterface) {
	for i := len(data) - 1; i >= 0; i-- {
		if data[i].To == "?" || data[i].LastUsed < time.Now().Add(-30*24*time.Hour).Unix() {
			data = append(data[:i], data[i+1:]...)
		}
	}
	json_to_file, _ := json.Marshal(data)
	mutex.Lock()
	err := os.WriteFile(whatsAppFile, json_to_file, 0755)
	mutex.Unlock()
	if err != nil {
		log.Error(err)
	}
}

// Read the  group info
func readTeamJson() []TeamInterface {
	var b []TeamInterface
	if jsonProtect {
		b = append(b, TeamInterface{Id: 0, Admin: true, Team: jsonTeam, Password: jsonPwd, Title: title, Prefix: commentPrefix, Planner: false})
	}
	if _, err := os.Stat(teamFile); errors.Is(err, os.ErrNotExist) {
		return b
	}
	file, err := os.ReadFile(teamFile)
	if err == nil {
		err = json.Unmarshal(file, &b)
		if err != nil {
			log.Error(err)
		}
	}
	return b
}

// Write the group info to file
func writeTeamJson(data []TeamInterface) {
	json_to_file, _ := json.Marshal(data)
	mutex.Lock()
	err := os.WriteFile(teamFile, json_to_file, 0755)
	mutex.Unlock()
	if err != nil {
		log.Error(err)
	}
}

// Read the  user info
func readUsersJson() []UserInterface {
	var b []UserInterface
	var u UserInterface = UserInterface{Username: "?", Password: "?"}
	b = append(b, u)
	if _, err := os.Stat(userFile); errors.Is(err, os.ErrNotExist) {
		return b
	}
	file, err := os.ReadFile(userFile)
	if err == nil {
		err = json.Unmarshal(file, &b)
		if err != nil {
			log.Error(err)
		}
	}
	return b
}

// Write the user info to file
func writeUsersJson(data []UserInterface) {
	for i := len(data) - 1; i >= 0; i-- {
		if data[i].Username == "?" || data[i].LastUsed < time.Now().Add(-30*24*time.Hour).Unix() {
			data = append(data[:i], data[i+1:]...)
		}
	}
	json_to_file, _ := json.Marshal(data)
	mutex.Lock()
	err := os.WriteFile(userFile, json_to_file, 0755)
	mutex.Unlock()
	if err != nil {
		log.Fatal(err)
	}
}

// Read the booking informatio
func readBookingJson() BookingSlice {
	b := BookingSlice{}
	if _, err := os.Stat(bookingFile); errors.Is(err, os.ErrNotExist) {
		return b
	}
	mutex.Lock()
	file, err := os.ReadFile(bookingFile)
	mutex.Unlock()
	if err != nil {
		log.Error(err)
	} else {
		err = json.Unmarshal(file, &b)
		if err != nil {
			//We have a error try to recover the backup file
			log.Error(err)
			if _, err := os.Stat(bookingFile + ".bak"); errors.Is(err, os.ErrNotExist) {
				writeBookingJson(b)
			} else {
				mutex.Lock()
				file, _ = os.ReadFile(bookingFile + ".bak")
				mutex.Unlock()
				err = json.Unmarshal(file, &b)
				if err != nil {
					writeBookingJson(b)
				}
			}
		}
	}
	return b
}

// Write the data to the booking file, removing expired data
func writeBookingJson(data BookingSlice) {
	for i := len(data) - 1; i >= 0; i-- {
		if data[i].State == "Delete" {
			log.WithFields(log.Fields{
				"state":    data[i].State,
				"boat":     data[i].Name,
				"fallback": data[i].Fallback,
				"user":     data[i].Username,
				"at":       shortDate(data[i].Date),
				"from":     shortTime(data[i].Time),
			}).Info("Deleting")
			data = append(data[:i], data[i+1:]...)
		}
	}
	json_to_file, _ := json.Marshal(data)
	mutex.Lock()
	os.Rename(bookingFile, bookingFile+".bak")
	err := os.WriteFile(bookingFile, json_to_file, 0755)
	mutex.Unlock()
	if err != nil {
		log.Fatal(err)
	}
}

// Function where al checks are done for a single booking and make the booking
func doBooking(b *BookingInterface) (changed bool, err error) {

	//Check thif booking should be canceled
	if b.State == "Cancel" {
		return true, boatCancel(b)
	}

	//Check if we should mark record for removal, after 12 hours
	if b.EpochEnd < time.Now().Add(-time.Hour*24).Unix() {
		log.Debug("Delete", b.EpochEnd, "<", time.Now().Add(-time.Hour*12).Unix())
		b.State = "Delete"
		b.Message = "Booking marked for Delete"
		return true, nil
	}

	//Check fail a booking in the past
	if b.EpochStart < time.Now().Unix() {
		b.State = "Failed"
		b.Message = "Booking in the past"
		return true, nil
	}

	//Check if we have a blocked item and a fallback is set
	if b.State == "Blocked" {
		if b.Fallback != "" {
			b.Message = "using fallback " + b.Fallback + " for boat " + b.Name
			b.Name = b.Fallback
			b.Fallback = ""
			b.State = "Retry"
			return true, nil
		} else {
			b.State = "Failed"
			return true, err
		}
	}

	//Create local time zone for printing
	loc, _ := time.LoadLocation(timeZoneLoc)

	//Check if we have a booking for the requested boat date and time
	for _, bs := range *b.Boats { //Find the boat in the BoatsList
		if strings.Contains(strings.ToLower(bs.Name), strings.ToLower(b.Name)) {
			var sunrise int64 = 0            //EpochData is good enough for Sunrise
			var sunset int64 = math.MaxInt64 //EpochEnd is good enough for Sunset
			var sunsetWindow int64 = sunset
			//Find the allowed sunset and sunrise for the given date of booking by going through de blocks
			for _, bb := range bs.Bookings { //Find the bookings of the boot
				//Check all sunset blocked area's for booking data to find allowed bookings window
				if bb.Type == "S" {
					if bb.EpochStart > b.EpochStart {
						sunset = MinInt64(sunset, bb.EpochStart)
						sunsetWindow = sunset
					}
					if bb.EpochEnd < b.EpochStart {
						sunrise = MaxInt64(sunrise, bb.EpochEnd)
					}
					//log.Info("S ", b.EpochDate, " SunRise ", sunrise, " SunSet ", sunset, " Start ", bb.EpochStart, " End ", bb.EpochEnd)
				}
				//Not Available area, if sunrise
				if bb.Type == "N" {
					if bb.EpochStart < sunset {
						sunset = bb.EpochStart
					}
					if bb.EpochStart < sunrise {
						sunrise = 0
					}
					//log.Info("N ", b.EpochDate, " SunRise ", sunrise, " SunSet ", sunset, " Start ", bb.EpochStart, " End ", bb.EpochEnd)
				}
			}
			log.Debug("Date ", b.EpochDate, " SunRise ", sunrise, " SunSet ", sunset)

			//Check if there is a reservation the could block us
			for _, bb := range bs.Bookings { //Find the bookings of the boot
				//Check if the booking contains the booking id we created
				if b.BookingId != bb.BookingId && bb.Type == "R" {
					//Check if there is a blockage
					if (b.EpochStart >= bb.EpochStart && b.EpochStart < bb.EpochEnd) ||
						(b.EpochEnd >= bb.EpochStart && b.EpochEnd < bb.EpochEnd) {
						if b.State == "Moving" {
							log.WithFields(log.Fields{
								"state": b.State,
								"boat":  b.Name,
								"user":  b.Username,
								"at":    shortDate(b.Date),
								"from":  shortTime(b.Time),
							}).Info("Canceled because of blocked by " + bb.BookingInfo)
							err = boatCancel(b)
						}
						b.State = "Blocked"
						b.Message = "booking blocked by " + bb.BookingInfo
						b.BookingId = ""
						return true, err
					}
					//Skip to next boat because we are not looking for this one
					continue
				}
			}

			//Calculate the minimal start and end time
			endtime := MinInt64(sunset, b.EpochEnd)
			starttime := MinInt64(b.EpochStart, MinInt64(b.EpochStart, endtime-int64(minDuration)*60))
			starttime = MaxInt64(starttime, sunrise)

			//Check if there is a timeslot for book
			if sunrise == 0 || sunset == math.MaxInt64 {
				b.Message = "Date not valid yet"
				b.State = "Waiting"
				b.EpochNext = time.Unix(b.EpochDate, 0).Add(-time.Duration(bookWindow) * time.Hour).Truncate(15 * time.Minute).Unix()
				return true, nil
			}

			//Check if we are not trying to book after sunset
			if b.EpochEnd > sunsetWindow {
				b.Message = "Booking beyond sunset not allowed"
				b.State = "Failed"
				return true, nil
			}

			//Check if we would be allowed booking, we need to be after Sunrise
			if time.Unix(sunset, 0).Add(-time.Duration(minDuration)*time.Minute).Unix() < sunrise {
				b.Message = "Starttime not valid yet"
				b.State = "Waiting"
				b.EpochNext = time.Unix(sunrise, 0).Add(-time.Duration(bookWindow) * time.Hour).Add(time.Duration(minDuration) * time.Minute).Truncate(15 * time.Minute).Unix()
				return true, nil
			}

			//Check if we are allowed to book this based on minimal duration
			if endtime-starttime < int64(minDuration*60) {
				b.Message = "Available duration, <" + strconv.FormatInt(int64(minDuration), 10) + "min"
				b.State = "Waiting"
				b.EpochNext = time.Unix(sunrise, 0).Add(-time.Duration(bookWindow) * time.Hour).Add(time.Duration(minDuration) * time.Minute).Truncate(15 * time.Minute).Unix()
				return true, nil
			}

			//Check if there is exiting booking from us we should move
			for _, bb := range bs.Bookings { //Find the bookings of the boot
				//This should be our boat booking update it
				if b.BookingId != "" && b.BookingId == bb.BookingId && bb.Type == "R" {
					if b.EpochStart == bb.EpochStart && b.EpochEnd == bb.EpochEnd {
						//Boat is on correct time and duration skip it
						return false, nil
					}

					//Check if their is a reason to update the booking
					if starttime > bb.EpochStart || endtime > bb.EpochEnd {
						err = boatUpdate(b, starttime, endtime)
						if err != nil {
							b.State = "Blocked" //Try fallback to do
						} else {
							if b.EpochStart == starttime && b.EpochEnd == endtime {
								b.State = "Finished"
							} else {
								b.State = "Moving"
							}
							b.Message = b.State + ":" + time.Unix(starttime, 0).In(loc).Format("15:04") + " - " + time.Unix(endtime, 0).In(loc).Format("15:04")
							b.Retry = 0
						}
						return true, err
					}
					//We found the boat but not updated it on it so we exit
					return false, nil
				}
			}

			//Get the boat ID and start the booking process
			b.BoatId = strconv.Itoa(bs.Id)
			err := boatBook(b, starttime, endtime)
			if err == nil { //We found the boat and could book it
				loc, _ := time.LoadLocation(timeZoneLoc)
				if b.EpochStart == starttime && b.EpochEnd == endtime {
					b.State = "Finished"
				} else {
					b.State = "Moving"
				}
				b.Message = b.State + ":" + time.Unix(starttime, 0).In(loc).Format("15:04") + " - " + time.Unix(endtime, 0).In(loc).Format("15:04")
				b.Retry = 0
			} else {
				b.State = "Blocked"
				b.Message = "Boat not bookable " + b.Name
			}
			return true, err
		} //If boat found
	} //Loop all Boats

	//Boat not found in the list
	log.WithFields(log.Fields{
		"state": b.State,
		"boat":  b.Name,
		"user":  b.Username,
		"at":    shortDate(b.Date),
		"from":  shortTime(b.Time),
		"boats": b.Boats,
	}).Info("Boat not found")
	b.State = "Blocked"
	b.Message = "Boat not found " + b.Name
	return true, err

}

// The main loop in which we do all the booking processing
func bookLoop() {
	log.Info("Start processing")
	var changed bool = false
	//Timing loop
	for {
		//Read de bookings
		bookingSlice := readBookingJson()
		wg := sync.WaitGroup{}
		for i := range bookingSlice {
			wg.Add(1)
			//We process a booking in parallel
			go func(booking *BookingInterface, changed *bool, wg *sync.WaitGroup) {
				var err error
				//At return of parallel task we changed decrease WaitGroup and log if changed
				defer func() {
					//If data has been changed update the booking array
					if booking.Changed {
						*changed = true
						//Sleep the booking for at least 15 min, if we are not instructed to skip
						if booking.State == "Blocked" || booking.State == "Retry" {
							booking.EpochNext = 0 //On blocked or Retry Item we will not wait
						} else if booking.EpochNext <= time.Now().Unix() {
							booking.EpochNext = MaxInt64(booking.EpochNext, time.Now().Add(15*time.Minute).Truncate(15*time.Minute).Unix())
						}
						loc, _ := time.LoadLocation(timeZoneLoc)
						nextStr := time.Unix(booking.EpochNext, 0).In(loc).Format("15:04")
						if err != nil {
							log.WithFields(log.Fields{
								"state": booking.State,
								"boat":  booking.Name,
								"user":  booking.Username,
								"at":    shortDate(booking.Date),
								"from":  shortTime(booking.Time),
								"next":  shortTime(nextStr),
								"unix":  time.Now().Unix(),
							}).Error(err)
						} else {
							log.WithFields(log.Fields{
								"state": booking.State,
								"boat":  booking.Name,
								"user":  booking.Username,
								"at":    shortDate(booking.Date),
								"from":  shortTime(booking.Time),
								"next":  shortTime(nextStr),
								"unix":  time.Now().Unix(),
							}).Info(booking.Message)
						}
					}
					wg.Done()
				}()
				//Set the timezone
				booking.TimeZone = timeZone

				//Correct the duration automaticly
				//Set the minimal duration
				if booking.Duration < int64(minDuration) {
					booking.Duration = int64(minDuration)
				}
				//Set the maximal duration
				if booking.Duration > int64(maxDuration) {
					booking.Duration = int64(maxDuration)
				}

				//Set the correct EpochDatas
				thetime, err := time.Parse(time.RFC3339, shortDate(booking.Date)+"T00:00:00+00:00")
				if err != nil {
					log.Error("date not valid yyyy-MM-dd")
					booking.State = "Failed"
					booking.Message = "date not valid yyyy-MM-dd"
					booking.Changed = true
					return
				}
				booking.EpochDate = thetime.Unix()
				thetime, err = time.Parse(time.RFC3339, shortDate(booking.Date)+"T"+shortTime(booking.Time)+":00"+booking.TimeZone)
				if err != nil {
					log.Error("time not valid hh:mm")
					booking.State = "Failed"
					booking.Message = "time not valid hh:mm"
					booking.Changed = true
					return
				}

				//Save the real start time and end time of booking
				booking.EpochStart = thetime.Unix()
				thetime = thetime.Add(time.Minute * time.Duration(booking.Duration))
				booking.EpochEnd = thetime.Unix()

				//Check if we should confirm the booking
				if booking.State == "Finished" && confirmTime != 0 &&
					time.Now().Unix() >= time.Unix(booking.EpochStart, 0).Add(-time.Duration(confirmTime)*time.Minute).Unix() && time.Now().Unix() <= time.Unix(booking.EpochStart, 0).Unix() {
					err = confirmBoat(booking)
					if err == nil {
						booking.State = "Confirmed"
						booking.Message = "Booking confirmed"
						booking.Changed = true
						return
					}
				}

				//Check if have allready processed the booking, if so skip it
				if booking.State == "Finished" || booking.State == "Confirmed" || booking.State == "Canceled" ||
					booking.State == "Failed" || booking.State == "Blocked" || booking.EpochNext > time.Now().Unix() {
					//Check if we should repeat this item
					if booking.Repeat != None && booking.EpochEnd < time.Now().Unix() {
						booking.State = "Repeat"
						booking.Message = "Booking is repeated"
						booking.Changed = true
						booking.BookingId = ""
						rs := func(c bool) int {
							if c {
								return 1
							}
							return 0
						}
						booking.Date = time.Unix(booking.EpochStart, 0).AddDate(rs(booking.Repeat == Yearly), rs(booking.Repeat == Monthly), rs(booking.Repeat == Daily)+(7*rs(booking.Repeat == Weekly))).Format("2006-01-02")

					} else
					//log.Println(booking.State, booking.Name, booking.Username, booking.Date, booking.Time)
					//Check if we should mark record for removal, after 24 hours
					if booking.EpochEnd < time.Now().Add(-time.Hour*24).Unix() {
						log.Debug("Delete", booking.EpochEnd, "<", time.Now().Add(-time.Hour*12).Unix())
						booking.State = "Delete"
						booking.Message = "Booking marked for Delete"
						booking.Changed = true
					}
					//Nothing to do more with this booking
					return
				}

				//Check if comment is set, if not fill default
				team, err := getTeamByName(booking.Team)
				if !booking.UserComment && team.AddTime {
					booking.Comment = shortTime(booking.Time) + " - " + thetime.Format("15:04")
				}

				// doBooking

				//Step 1: Login
				err = login(booking)
				if err != nil {
					log.Error(err)
					return
				}

				//Step 2: Read all boat data if older than 60 seconds
				_, boats := readBoatJson(booking, 60)
				booking.Boats = &boats

				//Step 3: Do the real Booking
				booking.Changed, err = doBooking(booking)
				if err != nil {
					booking.State = "Failed"
					booking.Message = err.Error()
					booking.Changed = true
				}

				//Step 4: Logout
				logout(booking)

				//Step 5: On Changed append the message to the log
				if booking.Changed {
					booking.Logs = append(booking.Logs, LogStruct{Date: time.Now().Unix(), State: booking.State, Log: booking.Message})
				}

			}(&bookingSlice[i], &changed, &wg)
		}

		//Wait for all bookings to have finished
		wg.Wait()
		//Save the change to the bookingFile on changed data
		if changed {

			writeBookingJson(bookingSlice)
			//Check if we should send a whatsapp message
			if whatsApp {
				//Send a message for all bookings with same to
				var list = map[string][]BookingInterface{}
				for _, b := range bookingSlice {
					if b.Changed && b.WhatsAppTo != "" {
						list[b.State+":"+b.Team+"-"+b.WhatsAppTo] =
							append(list[b.State+":"+b.Team+"-"+b.WhatsAppTo], b)
					}
				}
				//Finished: booking Amalthea, Argus, Artemis and Lynx at 9:30.
				for k, v := range list {
					var msg string
					var ks = strings.Split(k, ":") //Get the state, team-whatsappto
					if len(v) == 1 {
						msg = v[0].Name
					} else {
						for i, b := range v {
							if i == len(v)-1 {
								msg = msg + " and "
							} else if i > 0 {
								msg = msg + ", "
							}
							msg = msg + b.Name
						}
					}
					msg = "Booking " + strings.ToLower(ks[0]) + " for " + msg + " at " + shortDate(v[0].Date) + " " + shortTime(v[0].Time) + " hour."
					if sendWhatsAppMsg[ks[0]] { //Check for which states we should send message
						sendWhatsApp(v[0].Team, v[0].WhatsAppTo, msg)
					}
				}
			}
		}

		//Exit if we are in single run mode
		if singleRun {
			break
		}
		//Get the local time zone
		updateTimeZone()
		//We sleep before we restart, where we align as close as possible to interval
		time.Sleep(time.Duration(time.Now().Add(time.Duration(refreshInterval)*time.Second).Round(time.Duration(refreshInterval)*time.Second).Unix()-time.Now().Unix()) * time.Second)
		//log.Println("Awake from Sleep", refreshInterval)
	}
}

// Indicate which CORS sites are allowed
func allowOrigin(origin string) (bool, error) {
	// In this example we use a regular expression but we can imagine various
	// kind of custom logic. For example, an external datasource could be used
	// to maintain the list of allowed origins.
	return true, nil //regexp.MatchString(`^https:\/\/spaarne\.(\w).(\w)$`, origin)
}

// Function to create log entry
func makeLogEntry(c echo.Context) *log.Entry {
	if c == nil {
		return log.WithFields(log.Fields{
			"at": time.Now().Format("2006-01-02 15:04:05"),
		})
	}

	return log.WithFields(log.Fields{
		"at":     time.Now().Format("2006-01-02 15:04:05"),
		"method": c.Request().Method,
		"uri":    c.Request().URL.String(),
		"ip":     c.Request().RemoteAddr,
	})
}

// Middleware logging services
func middlewareLogging(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		makeLogEntry(c).Debug("incoming request")
		return next(c)
	}
}

// Error handler for JsonSer er
func errorHandler(err error, c echo.Context) {
	report, ok := err.(*echo.HTTPError)
	if ok {
		report.Message = fmt.Sprintf("http error %d - %v", report.Code, report.Message)
	} else {
		report = echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if report.Code == 401 || report.Code == 404 {
		makeLogEntry(c).Debug(report.Message)
	} else {
		makeLogEntry(c).Error(report.Message)
	}
	c.HTML(report.Code, report.Message.(string))
}

func getTeamByName(teamName string) (*TeamInterface, error) {
	for _, t := range teams {
		if t.Team == teamName {
			return &t, nil
		}
	}
	return nil, errors.New("team not found")
}

func getTeamByContext(c echo.Context) (*TeamInterface, error) {
	var gt TeamInterface
	auth := c.Request().Header["Authorization"]
	if jsonProtect && len(auth) != 0 {
		s := strings.Fields(auth[0])
		if s[0] == "Basic" {
			//log.Debug("Basic",  base64.StdEncoding.DecodeToString((c.Request().Header["Authorization"]))
			k, err := base64.StdEncoding.DecodeString(s[1])
			if err != nil {
				return &gt, err
			}
			a := strings.Split(string(k), ":")
			for i, t := range teams {
				if t.Team == a[0] && t.Password == a[1] {
					return &teams[i], nil
				}
			}
		}
	} else if !jsonProtect {
		for i, t := range teams {
			if t.Id == 0 {
				return &teams[i], nil
			}
		}
	}
	return &gt, errors.New("invalid Authorization Header")
}

// The basic web server
func jsonServer() error {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middlewareLogging)
	e.HTTPErrorHandler = errorHandler
	g := e.Group("/data")
	if jsonProtect {
		g.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			for _, g := range teams {
				if g.Team == username && g.Password == password {
					return true, nil
				}
			}
			return false, nil
		}))
	}

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: allowOrigin,
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete,
			http.MethodPatch},
	}))

	e.GET("data/config", func(c echo.Context) error {
		//For config we allways want to have the latest team info
		teams = readTeamJson()
		g, _ := getTeamByContext(c)
		configData := map[string]interface{}{
			"version":        AppVersion,
			"name":           AppName,
			"team":           g.Team,
			"interval":       refreshInterval,
			"prefix":         iif(g.Prefix, commentPrefix),
			"clubid":         clubId,
			"admin":          g.Admin,
			"myfleetVersion": myFleetVersion,
			"timezone":       timeZoneLoc,
			"title":          iif(g.Title, iif(g.Team, title)),
			"whatsapp":       g.WhatsApp && whatsApp,
			"whatsappid":     g.WhatsAppId,
			"whatsappto":     g.WhatsAppTo,
			"authRequired":   jsonProtect,
			"planner":        g.Planner && planner,
		}
		return c.JSON(http.StatusOK, configData)
	})

	//Protected requests
	g.GET("/boat", func(c echo.Context) error {
		boatNames, _ := readBoatJson(nil, 24*60*60)
		return c.JSON(http.StatusOK, boatNames)
	})

	//Protected requests
	g.GET("/teams", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		if team.Admin {
			return c.JSON(http.StatusOK, teams)
		} else {
			return c.JSON(http.StatusOK, TeamFilter(teams, team.Team))
		}
	})

	g.GET("/teams/:id", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		t := readTeamJson()
		for _, tt := range t {
			if c.Param("id") == strconv.FormatInt(tt.Id, 10) && (team.Admin || tt.Team == team.Team) {
				return c.JSON(http.StatusOK, tt)
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	g.POST("/teams", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		teams = readTeamJson()
		//Autoincrement booking id
		var id int64 = 0
		for _, t := range teams {
			id = MaxInt64(id, t.Id+1)
		}
		new_team := new(TeamInterface)
		err = c.Bind(new_team)
		new_team.Id = id
		if err != nil {
			return c.String(http.StatusBadRequest, "Bad request.")
		}

		teams = append(teams, *new_team)
		writeTeamJson(teams)
		log.WithFields(log.Fields{
			"team":  new_team.Team,
			"title": new_team.Title,
		}).Info("Added team")

		if team.Admin {
			return c.JSON(http.StatusOK, teams)
		} else {
			return c.JSON(http.StatusOK, TeamFilter(teams, team.Team))
		}
	})

	g.PUT("/teams/:id", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		teams = readTeamJson()

		updated_team := new(TeamInterface)
		err = c.Bind(updated_team)
		if err != nil {
			log.Error(err, updated_team)
			return c.String(http.StatusBadRequest, "Bad request.")
		}
		log.WithFields(log.Fields{
			"team":  updated_team.Team,
			"title": updated_team.Title,
		}).Info("Updated team")

		for i, t := range teams {
			if strconv.FormatInt(t.Id, 10) == c.Param("id") && (team.Admin || t.Team == team.Team) {
				teams = append(teams[:i], teams[i+1:]...)
				teams = append(teams, *updated_team)
				writeTeamJson(teams)
				if team.Admin {
					return c.JSON(http.StatusOK, teams)
				} else {
					return c.JSON(http.StatusOK, TeamFilter(teams, team.Team))
				}
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	g.DELETE("/teams/:id", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		teams = readTeamJson()

		for i, t := range teams {
			if strconv.FormatInt(t.Id, 10) == c.Param("id") && (team.Admin || t.Team == team.Team) {
				teams = append(teams[:i], teams[i+1:]...)
				//Disconnect the whatsapp if set
				if team.WhatsAppId != "" {
					devices, err := whatsAppContainer.GetAllDevices()
					if err != nil {
						return c.JSON(http.StatusInternalServerError, err)
					}
					for _, dd := range devices {
						if dd.ID.String() == team.WhatsAppId {
							client := whatsmeow.NewClient(dd, whatsAppLog)
							if client.Store.ID != nil {
								client.Connect()
								client.Logout()
							}
							break
						}
					}
				}
				writeTeamJson(teams)
				//Delete all users of team
				users := readUsersJson()
				for i, u := range users {
					if u.Team == team.Team {
						users = append(users[:i], users[i+1:]...)
					}
				}
				writeUsersJson(users)

				//Delete all whatsappto of team
				whatsappto := readWhatsAppJson()
				for i, w := range whatsappto {
					if w.Team == team.Team {
						whatsappto = append(whatsappto[:i], whatsappto[i+1:]...)
					}
				}
				writeWhatsAppJson(whatsappto)

				//Delete all bookings of
				bookings := readBookingJson()
				for i, b := range bookings {
					if b.Team == team.Team {
						bookings = append(bookings[:i], bookings[i+1:]...)
					}
				}
				writeBookingJson(bookings)

				if team.Admin {
					return c.JSON(http.StatusOK, teams)
				} else {
					return c.JSON(http.StatusOK, TeamFilter(teams, team.Team))
				}
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	g.GET("/booking", func(c echo.Context) error {
		bookings := readBookingJson()
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		if team.Admin {
			return c.JSON(http.StatusOK, bookings)
		} else {
			return c.JSON(http.StatusOK, TeamFilter(bookings, team.Team))
		}
	})

	g.GET("/booking/:id", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		bookings := readBookingJson()
		for _, booking := range bookings {
			if c.Param("id") == strconv.FormatInt(booking.Id, 10) && (team.Admin || team.Team == booking.Team) {
				return c.JSON(http.StatusOK, booking)
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	g.POST("/booking", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		bookings := readBookingJson()
		//Autoincrement booking id
		var id int64 = 0
		for _, booking := range bookings {
			id = MaxInt64(id, booking.Id+1)
		}
		new_booking := new(BookingInterface)
		err = c.Bind(new_booking)
		new_booking.Id = id
		new_booking.State = ""
		new_booking.Message = ""
		new_booking.EpochNext = -1
		new_booking.Team = cif(team.Admin, iif(new_booking.Team, team.Team), team.Team)
		new_booking.UserComment = strings.Trim(new_booking.Comment, " ") != ""
		new_booking.Logs = append(new_booking.Logs, LogStruct{Date: time.Now().Unix(), State: new_booking.State, Log: "Created by " + team.Title})

		if err != nil {
			return c.String(http.StatusBadRequest, "Bad request.")
		}

		//Round the time to the closed one
		if strings.Contains(new_booking.Time, "T") {
			thetime, _ := time.Parse(time.RFC3339, new_booking.Time)
			new_booking.Time = thetime.Round(15 * time.Minute).Format(time.RFC3339)
		}

		bookings = append(bookings, *new_booking)
		writeBookingJson(bookings)
		log.WithFields(log.Fields{
			"boat": new_booking.Name,
			"user": new_booking.Username,
			"at":   shortDate(new_booking.Date),
			"from": shortTime(new_booking.Time),
		}).Info("Added boat")

		//Add password to user file
		users := readUsersJson()
		var found bool = false
		for i, usr := range users {
			if strings.EqualFold(usr.Username, new_booking.Username) && usr.Team == team.Team {
				users[i].Password = new_booking.Password
				users[i].LastUsed = time.Now().Unix()
				found = true
				break
			}
		}
		if !found {
			var id int64 = 0
			for _, t := range users {
				id = MaxInt64(id, t.Id+1)
			}
			users = append(users, UserInterface{Id: id, Name: new_booking.Username, Username: new_booking.Username, Password: new_booking.Password, LastUsed: time.Now().Unix(), Team: new_booking.Team})
		}
		writeUsersJson(users)

		//Add whatsapp to whatsapp file
		whatsappData := readWhatsAppJson()
		found = false
		for i, d := range whatsappData {
			if strings.EqualFold(d.To, new_booking.WhatsAppTo) && d.Team == team.Team {
				whatsappData[i].LastUsed = time.Now().Unix()
				found = true
				break
			}
		}

		if !found && new_booking.WhatsAppTo != "" {
			whatsappData = append(whatsappData, WhatsAppToInterface{To: new_booking.WhatsAppTo, LastUsed: time.Now().Unix(), Team: new_booking.Team})
		}
		writeWhatsAppJson(whatsappData)
		if team.Admin {
			return c.JSON(http.StatusOK, bookings)
		} else {
			return c.JSON(http.StatusOK, TeamFilter(bookings, team.Team))
		}
	})

	g.PUT("/booking/:id", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		bookings := readBookingJson()

		updated_booking := new(BookingInterface)
		err = c.Bind(updated_booking)
		if err != nil {
			log.Error(err, updated_booking)
			return c.String(http.StatusBadRequest, "Bad request.")
		}
		updated_booking.EpochNext = 0
		updated_booking.State = ""
		updated_booking.Message = ""
		updated_booking.Team = cif(team.Admin, iif(updated_booking.Team, team.Team), team.Team)
		//Round the time to the closed one
		if strings.Contains(updated_booking.Time, "T") {
			thetime, _ := time.Parse(time.RFC3339, updated_booking.Time)
			updated_booking.Time = thetime.Round(15 * time.Minute).Format(time.RFC3339)
		}
		log.WithFields(log.Fields{
			"boat": updated_booking.Name,
			"user": updated_booking.Username,
			"at":   shortDate(updated_booking.Date),
			"from": shortTime(updated_booking.Time),
		}).Info("Updated boat")

		//Add password to user file
		users := readUsersJson()
		var found bool = false
		for i, usr := range users {
			if strings.EqualFold(usr.Username, updated_booking.Username) && usr.Team == team.Team {
				users[i].Password = updated_booking.Password
				users[i].LastUsed = time.Now().Unix()
				found = true
				break
			}
		}
		if !found {
			var id int64 = 0
			for _, t := range users {
				id = MaxInt64(id, t.Id+1)
			}
			users = append(users, UserInterface{Id: id, Name: iif(updated_booking.Name, updated_booking.Username), Username: updated_booking.Username, Password: updated_booking.Password, LastUsed: time.Now().Unix(), Team: updated_booking.Team})
		}
		writeUsersJson(users)

		//Add whatsapp to whatsapp file
		whatsappData := readWhatsAppJson()
		found = false
		for i, d := range whatsappData {
			if strings.EqualFold(d.To, updated_booking.WhatsAppTo) && d.Team == team.Team {
				whatsappData[i].LastUsed = time.Now().Unix()
				found = true
				break
			}
		}

		if !found && updated_booking.WhatsAppTo != "" {
			whatsappData = append(whatsappData, WhatsAppToInterface{To: updated_booking.WhatsAppTo, LastUsed: time.Now().Unix(), Team: updated_booking.Team})
		}
		writeWhatsAppJson(whatsappData)

		for i, booking := range bookings {
			if strconv.FormatInt(booking.Id, 10) == c.Param("id") && (team.Admin || booking.Team == team.Team) {
				bookings = append(bookings[:i], bookings[i+1:]...)
				//Do whe have a updated using user comment
				updated_booking.UserComment = booking.UserComment ||
					booking.Comment != updated_booking.Comment

				//Cancel a Boat when you update it, while it is finished
				if (booking.State == "Finished" || booking.State == "Confirmed") &&
					(shortDate(booking.Date) != shortDate(updated_booking.Date) ||
						booking.Duration != updated_booking.Duration ||
						booking.Name != updated_booking.Name) {
					boatCancel(&booking)
					updated_booking.Logs = append(booking.Logs, LogStruct{Date: time.Now().Unix(), State: booking.State, Log: "Canceled to update by " + team.Title})
				} else {
					updated_booking.Logs = append(booking.Logs, LogStruct{Date: time.Now().Unix(), Log: "Updated by " + team.Title})
				}
				bookings = append(bookings, *updated_booking)
				writeBookingJson(bookings)
				if team.Admin {
					return c.JSON(http.StatusOK, bookings)
				} else {
					return c.JSON(http.StatusOK, TeamFilter(bookings, team.Team))
				}
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	g.DELETE("/booking/:id", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		bookings := readBookingJson()

		for i, booking := range bookings {
			if strconv.FormatInt(booking.Id, 10) == c.Param("id") && (team.Admin || booking.Team == team.Team) {
				if booking.State == "Canceled" {
					log.WithFields(log.Fields{
						"state": booking.State,
						"boat":  booking.Name,
						"user":  booking.Username,
						"at":    shortDate(booking.Date),
						"from":  shortTime(booking.Time),
					}).Info("Deleting")
					bookings = append(bookings[:i], bookings[i+1:]...)
					writeBookingJson(bookings)
				} else if booking.State != "Cancel" {
					booking.State = "Cancel"
					booking.Message = "Canceled"
					booking.EpochNext = 0
					booking.Logs = append(booking.Logs, LogStruct{Date: time.Now().Unix(), State: booking.State, Log: "Canceled by " + team.Title})
					bookings[i] = booking
					writeBookingJson(bookings)
				}
				if team.Admin {
					return c.JSON(http.StatusOK, bookings)
				} else {
					return c.JSON(http.StatusOK, TeamFilter(bookings, team.Team))
				}
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	g.GET("/users", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		if team.Admin {
			return c.JSON(http.StatusOK, readUsersJson())
		} else {
			return c.JSON(http.StatusOK, TeamFilter(readUsersJson(), team.Team))
		}
	})

	g.GET("/users/:id", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		u := readUsersJson()
		for _, uu := range u {
			if c.Param("id") == strconv.FormatInt(uu.Id, 10) && (team.Admin || uu.Team == team.Team) {
				return c.JSON(http.StatusOK, uu)
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	g.POST("/users", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		u := readUsersJson()
		//Autoincrement booking id
		var id int64 = 0
		for _, t := range u {
			id = MaxInt64(id, t.Id+1)
		}
		new_user := new(UserInterface)
		err = c.Bind(new_user)
		new_user.Team = cif(team.Admin, iif(new_user.Team, team.Team), team.Team)
		new_user.Name = iif(new_user.Name, new_user.Username)
		new_user.Id = id
		if err != nil {
			return c.String(http.StatusBadRequest, "Bad request.")
		}

		u = append(u, *new_user)
		writeUsersJson(u)
		log.WithFields(log.Fields{
			"team":     new_user.Team,
			"username": new_user.Username,
		}).Info("Added user")

		if team.Admin {
			return c.JSON(http.StatusOK, u)
		} else {
			return c.JSON(http.StatusOK, TeamFilter(u, team.Team))
		}
	})

	g.PUT("/users/:id", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		u := readUsersJson()

		updated_user := new(UserInterface)
		err = c.Bind(updated_user)
		updated_user.Team = cif(team.Admin, iif(updated_user.Team, team.Team), team.Team)
		if err != nil {
			log.Error(err, updated_user)
			return c.String(http.StatusBadRequest, "Bad request.")
		}
		log.WithFields(log.Fields{
			"team":     updated_user.Team,
			"username": updated_user.Username,
		}).Info("Updated user")

		for i, uu := range u {
			if strconv.FormatInt(uu.Id, 10) == c.Param("id") && (team.Admin || uu.Team == team.Team) {
				u = append(u[:i], u[i+1:]...)
				u = append(u, *updated_user)
				writeUsersJson(u)
				if team.Admin {
					return c.JSON(http.StatusOK, u)
				} else {
					return c.JSON(http.StatusOK, TeamFilter(u, team.Team))
				}
			}
		}

		return c.String(http.StatusNotFound, "Not found.")
	})

	g.DELETE("/users/:id", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		u := readUsersJson()

		for i, uu := range u {
			if strconv.FormatInt(uu.Id, 10) == c.Param("id") && (team.Admin || uu.Team == team.Team) {
				u = append(u[:i], u[i+1:]...)
				writeUsersJson(u)
				if team.Admin {
					return c.JSON(http.StatusOK, u)
				} else {
					return c.JSON(http.StatusOK, TeamFilter(u, team.Team))
				}
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	g.GET("/whatsappto", func(c echo.Context) error {
		team, err := getTeamByContext(c)
		if err != nil {
			return c.JSON(http.StatusForbidden, err)
		}
		return c.JSON(http.StatusOK, TeamFilter(readWhatsAppJson(), team.Team))
	})

	//Request a new whatsapp connection for specified string
	g.DELETE("/whatsapp", func(c echo.Context) error {
		if !whatsApp {
			return c.JSON(http.StatusForbidden, errors.New("WhatsApp is disabled"))
		}
		//Find or create the store device
		team, _ := getTeamByContext(c)
		devices, err := whatsAppContainer.GetAllDevices()
		if err != nil {
			team.WhatsAppId = ""
			team.QRCode = ""
			writeTeamJson(teams)
			return c.JSON(http.StatusInternalServerError, err)
		}

		if team.WhatsAppId != "" {
			for _, dd := range devices {
				if dd.ID.String() == team.WhatsAppId {
					client := whatsmeow.NewClient(dd, whatsAppLog)
					if client.Store.ID != nil {
						client.Connect()
						client.Logout()
					}
					//Clear whatsapp info
					team.WhatsAppId = ""
					team.QRCode = ""
					//TODO: Fix this
					writeTeamJson(teams)
					return c.JSON(http.StatusOK, "Connection deleted")
				}
			}
		}
		return c.JSON(http.StatusNotFound, "Connection not found deleted")
	})

	g.GET("/whatsapp", func(c echo.Context) error {
		if !whatsApp {
			return c.JSON(http.StatusForbidden, errors.New("WhatsApp is disabled"))
		}
		team, _ := getTeamByContext(c)
		devices, err := whatsAppContainer.GetAllDevices()
		if err != nil {
			log.Debug(err)
			return c.JSON(http.StatusInternalServerError, err)
		}
		//Find or create the store device
		var d *store.Device = nil

		if team.WhatsAppId != "" {
			for _, dd := range devices {
				if dd.ID.String() == team.WhatsAppId {
					d = dd
					break
				}
			}
		}
		if d == nil {
			d = whatsAppContainer.NewDevice()
		}
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		c.Response().WriteHeader(http.StatusOK)
		enc := json.NewEncoder(c.Response())
		//Check if we have a client, if so responde with
		whatsAppClient := whatsmeow.NewClient(d, whatsAppLog)
		if whatsAppClient.Store.ID != nil {
			team.QRCode = ""
			team.WhatsAppId = d.ID.String()
			//TODO: Not the nices way to update, whe should find it
			writeTeamJson(teams)
			if err := enc.Encode(*team); err != nil {
				return err
			}
			c.Response().Flush()
			return nil
		}
		//Invalid whatsappid, so clear it
		team.WhatsAppId = ""

		qrChan, _ := whatsAppClient.GetQRChannel(context.Background())
		err = whatsAppClient.Connect()
		if err != nil {
			log.Error(err)
			return err
		} else {
			log.Info("New WhatsAppClient connected")
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, log.New().WriterLevel(log.InfoLevel))
				team.QRCode = evt.Code
				if err := enc.Encode(*team); err != nil {
					return err
				}
				c.Response().Flush()
			} else {
				team.QRCode = ""
				log.Info("Login event:", evt.Event)
				if evt.Event == "success" && d.ID != nil {
					team.WhatsAppId = d.ID.String()
					//TODO: Not the nices way to update, whe should find it
					writeTeamJson(teams)
					log.WithField("WhatsAppId", team.WhatsAppId).Debug("Created whatsapp id")
					if err := enc.Encode(*team); err != nil {
						return err
					}
					c.Response().Flush()
				}
			}
		}
		//Close the connection without error
		return nil

	})

	e.POST("/data/login", func(c echo.Context) error {
		new_login := new(LoginInterface)
		err := c.Bind(new_login)
		new_login.Status = "Error"
		if err == nil {
			teams = readTeamJson()
			for _, t := range teams {
				if t.Team == new_login.Team && t.Password == new_login.Password {
					new_login.Status = "ok"
					break
				}
			}
		}
		return c.JSON(http.StatusOK, new_login)
	})

	//Serve the app
	g.Static("/", "public")
	e.Static("/", "public")
	log.Printf("Start jsonserver on %s", bindAddress)
	return e.Start(bindAddress)
}

// Whatsapp logger stuff
type stdoutLogger struct{}

func (s *stdoutLogger) Errorf(msg string, args ...interface{}) { log.Errorf(msg, args...) }
func (s *stdoutLogger) Warnf(msg string, args ...interface{})  { log.Warnf(msg, args...) }
func (s *stdoutLogger) Infof(msg string, args ...interface{})  { log.Infof(msg, args...) }
func (s *stdoutLogger) Debugf(msg string, args ...interface{}) { log.Debugf(msg, args...) }
func (s *stdoutLogger) Sub(_ string) waLog.Logger              { return s }

// Send a whatsapp message
func sendWhatsApp(teamName string, name string, msg string) {
	if !whatsApp {
		log.Error("Trying to send WhatsApp message when disabled")
		return
	}
	team, err := getTeamByName(teamName)
	if err != nil {
		log.WithField("Team", teamName).Error("Failed sending WhatsApp message to unknown team")
		return
	}
	if team.WhatsAppId == "" {
		log.WithField("Team", teamName).Error("Cannot send WhatsApp message, because team Has no WhatsAppId")
		return
	}
	jid, err := types.ParseJID(team.WhatsAppId)
	if err != nil {
		log.Error(err)
		return
	}
	device, err := whatsAppContainer.GetDevice(jid)
	if err != nil {
		log.Error(err)
		return
	}
	client := whatsmeow.NewClient(device, whatsAppLog)
	if client.Store.ID == nil {
		log.Error("Client for deviceID not connected")
		return
	}
	err = client.Connect()
	if err != nil {
		log.Error(err)
	}
	//Ensure we disconnect on return
	defer client.Disconnect()

	if client != nil && client.IsConnected() {
		//Check out if whe should send message to server of user
		wgroups, _ := client.GetJoinedGroups()
		for _, g := range wgroups {
			if strings.EqualFold(g.GroupName.Name, name) {
				_, err := client.SendMessage(context.Background(), g.JID,
					&waProto.Message{
						Conversation: proto.String(msg),
					})
				log.WithFields(log.Fields{
					"msg": msg,
					"to":  name,
				}).Info("Sending Group Whatsapp")
				if err != nil {
					log.Error("Failed to send whatsapp", err)
				}
				return
			}
		}
		//Not found send the message to name
		if _, err := strconv.ParseInt(name, 10, 64); err == nil {
			_, err := client.SendMessage(context.Background(), types.JID{
				User:   name,
				Server: types.DefaultUserServer,
			}, &waProto.Message{
				Conversation: proto.String(msg),
			})
			log.WithFields(log.Fields{
				"msg": msg,
				"to":  name,
			}).Info("Sending Whatsapp")
			if err != nil {
				log.Error("Failed to send whatsapp", err)
			}
		} else {
			log.Error("Failed to send whatsapp not a number", name)
		}
	}
}

func main() {
	var err error
	Init()
	db, err = sql.Open(dbType, "file:"+dbFile+"?_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	//Create whatsAppContainer
	if whatsApp {
		store.SetOSInfo(AppName, sliceVersion(AppVersion))
		var clientLog waLog.Logger = &stdoutLogger{}
		whatsAppContainer = sqlstore.NewWithDB(db, dbType, clientLog)
		err = whatsAppContainer.Upgrade()
		if err != nil {
			log.Fatal(err)
		}
		log.Info("WhatsApp enabled")
	}
	//Catch shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c
		log.Info("Waiting for clean Exit")
		mutex.Lock()
		db.Close()
		os.Exit(0)
	}()

	if !singleRun {
		go bookLoop()
		err := jsonServer()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		switch test {
		case "boatlist":
			names, boats := readBoatJson(nil, 0)
			log.Info("BoatList", names, boats)
		case "regexp":
			b, _ := os.ReadFile("test.html")
			re := regexp.MustCompile(`ReservationId = (.*) `)
			rem := re.FindStringSubmatch(string(b))
			var BookingId int64
			if len(rem) > 0 {

				BookingId, _ = strconv.ParseInt(strings.Trim(rem[1], " "), 10, 64)
			}
			log.Info("ID ", BookingId)
		default:
			bookLoop()
		}
	}
}
