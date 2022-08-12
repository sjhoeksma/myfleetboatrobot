package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

var Version = "0.3.0"                 //The version of application
var clubId = "R1B34"                  //The club code
var bookingFile = "json/booking.json" //The json file to store bookings in
var boatFile = "json/boats.json"      //The json file to store boats
var userFile = "json/users.json"      //The json file to store users
var timeZoneLoc = "Europe/Amsterdam"  //The time zone location for the club
var timeZone = "+02:00"               //The time zone in hour, is also calculated
var minDuration = 60                  //The minimal duration required to book
var maxDuration = 120                 //The maximal duration allowed to book
var bookWindow = 48                   //The number of hours allowed to book
var maxRetry int = 0                  //The maximum numbers of retry before we give up, 0=disabled
var refreshInterval int = 1           //We do a check of the database every 1 minute
var logLevel string = "Info"          //Default loglevel is info

//Convert boat to code
var boatFilter = map[string]string{
	"Alle boten": "0",
	"1x":         "1",
	"1x, jeugd":  "39",
	"2-":         "2",
	"2x":         "3",
	"3x":         "58",
	"4+":         "10",
	"4-":         "9",
	"4x":         "12",
	"4x+/4":      "42",
	"4x-":        "61",
	"8+":         "13",
	"bowa":       "65",
	"C1x":        "15",
	"C2+":        "16",
	"C2x":        "17",
	"C2x+":       "18",
	"C4+":        "20",
	"C4x+":       "21",
	"C4x+/C4+":   "43",
	"Centaur":    "68",
	"Ergometer":  "30",
	"Laser":      "69",
	"Motorboat":  "61",
	"Polyvalk":   "60",
	"Randmeer":   "66",
	"Ruimte":     "67",
	"Wx1+":       "24",
	"Wx2+":       "25",
	"Zeilwherry": "57",
}

//Used to convert months into date
var maandFilter = map[string]string{
	"januari":   "01",
	"februari":  "02",
	"maart":     "03",
	"april":     "04",
	"mei":       "05",
	"juni":      "06",
	"juli":      "07",
	"augustus":  "08",
	"september": "09",
	"oktober":   "10",
	"november":  "11",
	"december":  "12",
}

//Struc used to store user info
type UserInterface struct {
	Username string `json:"user"`
	Password string `json:"password"`
	LastUsed int64  `json:"lastused"`
}

//Struc used to store boat and session info
type BookingInterface struct {
	Id          int64          `json:"id"`
	Name        string         `json:"boat"`
	Date        string         `json:"date"`
	Time        string         `json:"time"`
	Duration    int64          `json:"duration"`
	Username    string         `json:"user"`
	Password    string         `json:"password"`
	Comment     string         `json:"comment"`
	State       string         `json:"state,omitempty"`
	BookingId   string         `json:"bookingid,omitempty"`
	BoatId      string         `json:"boatid,omitempty"`
	BoatFilter  string         `json:"boatfilter,omitempty"`
	Message     string         `json:"message,omitempty"`
	EpochNext   int64          `json:"next,omitempty"`
	Retry       int            `json:"retry,omitempty"`
	UserComment bool           `json:"usercomment,omitempty"`
	TimeZone    string         `json:"-"`
	Cookies     []*http.Cookie `json:"-"`
	Bookings    *[][]string    `json:"-"`
	EpochDate   int64          `json:"-"`
	EpochStart  int64          `json:"-"`
	EpochEnd    int64          `json:"-"`
	Authorized  bool           `json:"-"`
}

//The list of bookings
type BookingSlice []BookingInterface

//The struct used to retreive available boats
type BoatListInterface struct {
	BoatFilter string
	SunRise    int64
	SunSet     int64
	EpochDate  int64
	EpochStart int64
	EpochEnd   int64
	Boats      *[][]string
}

var singleRun bool = true             //Should we do a single runonly = nowebserver
var commentPrefix string = "MYFR:"    //The prefix we use as a comment indicator the booking is ours
var bindAddress string = ":1323"      //The default bind port of web server
var jsonUser string                   //The Basic Auth user of webserer
var jsonPwd string                    //The Basic Auth password of webserver
var jsonProtect bool                  //Should the web server use Basic Auth
var baseUrl string                    //The base url towards the fleet.eu backend
var guiUrl string                     //The gui url towards the fleet.eu backend
var test string = ""                  //The test we should be running, means allways single ru
var mutex *sync.Mutex = &sync.Mutex{} //The lock used where writing files

//Find min of 2 int64 values
func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

//Find max of 2 int64 values
func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

//Function to get an ENV variable and put it into a string
func setEnvValue(key string, item *string) {
	s := os.Getenv(key)
	if s != "" {
		*item = s
	}
}

//Make from a long date string a short one
func shortDate(date string) string {
	return strings.Split(date, "T")[0]
}

//Make from a long data string as short time
func shortTime(timeS string) string {
	if strings.Contains(timeS, "T") {
		thetime, _ := time.Parse(time.RFC3339, timeS)
		loc, _ := time.LoadLocation(timeZoneLoc)
		return thetime.Round(15 * time.Minute).In(loc).Format("15:04")
	}
	thetime, _ := time.Parse(time.RFC3339, "2001-01-01"+"T"+timeS+":00+00:00")
	return thetime.Round(15 * time.Minute).Format("15:04")
}

//Read and set settings
func Init() {
	setEnvValue("JSONUSR", &jsonUser)
	setEnvValue("JSONPWD", &jsonPwd)
	setEnvValue("PREFIX", &commentPrefix)
	setEnvValue("TIMEZONE", &timeZone)
	setEnvValue("CLUBID", &clubId)
	setEnvValue("LOGLEVEL", &logLevel)

	version := flag.Bool("version", false, "Prints current version ("+Version+")")
	flag.BoolVar(&singleRun, "singleRun", singleRun, "Should we only do one run")
	flag.StringVar(&commentPrefix, "prefix", commentPrefix, "Comment prefix")
	flag.StringVar(&timeZoneLoc, "timezone", timeZoneLoc, "The timezone location used by user")
	flag.IntVar(&refreshInterval, "refresh", refreshInterval, "The iterval in minutes used for refeshing")
	flag.IntVar(&bookWindow, "bookWindow", bookWindow, "The interval in hours for allowed bookings")
	flag.IntVar(&maxRetry, "maxRetry", maxRetry, "The maximum retry's before failing, 0=disabled")
	flag.StringVar(&bindAddress, "bind", bindAddress, "The bind address to be used for webserver")
	flag.StringVar(&jsonUser, "jsonUsr", jsonUser, "The user to protect jsondata")
	flag.StringVar(&jsonPwd, "jsonPwd", jsonPwd, "The password to protect jsondata")
	flag.StringVar(&clubId, "clubId", clubId, "The clubId used")
	flag.StringVar(&logLevel, "logLevel", logLevel, "The log level to use")
	flag.StringVar(&test, "test", test, "The test action to perform")

	flag.Parse() // after declaring flags we need to call it
	if *version {
		log.Println("Version ", Version)
		os.Exit(0)
	}
	//When test action is specified we are allways in singlerun
	if test != "" {
		singleRun = true
	}

	//Setup the logging
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}
	log.SetFormatter(&log.TextFormatter{DisableColors: false, FullTimestamp: true})
	//log.SetFormatter(&log.JSONFormatter{DisableColors: false, FullTimestamp: true,})

	//Only enable jsonProtection if we have a username and password
	jsonProtect = (jsonUser != "" && jsonPwd != "")
	baseUrl = "https://my-fleet.eu/" + clubId + "/mobile/index0.php?&system=mobile&language=NL"
	guiUrl = "https://my-fleet.eu/" + clubId + "/gui/index.php"
	loc, err := time.LoadLocation(timeZoneLoc)
	if err != nil {
		log.Fatal(err)
	}
	timeZone = time.Now().In(loc).Format("-07:00")

	//Log the version
	log.Info("MyFleet Robot v" + Version)
}

//Create from the html response a booking array
func readbookingList(htm *string) *[][]string {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(*htm))
	var row []string
	var rows [][]string
	re := regexp.MustCompile(`goIE8\(this, "edit", "", ([0-9]+)\);$`)
	// Find each table
	var curDate string
	doc.Find("table").Each(func(base int, basehtml *goquery.Selection) {
		basehtml.Find("tr").Each(func(basetr int, baserow *goquery.Selection) {
			baserow.Find("td").Each(func(baseth int, basecell *goquery.Selection) {
				if basecell.HasClass("rsrv_overview_date") {
					dateS := strings.Fields(basecell.Text()) //donderdag 21 juli 2022
					curDate = dateS[3] + "-" + maandFilter[dateS[2]] + "-" + dateS[1]
				}
				basecell.Find("table").Each(func(indextbl int, tablehtml *goquery.Selection) {
					tablehtml.Find("tr").Each(func(indextr int, rowhtml *goquery.Selection) {
						rowhtml.Find("td").Each(func(indexth int, tablecell *goquery.Selection) {
							s := strings.TrimSpace(tablecell.Text())
							if s != "" {
								//On a new row add the last time
								if row == nil {
									row = append(row, curDate)
								}
								row = append(row, s)
							}
						})
					})

				})
			})
			link, exist := baserow.Attr("onclick")
			if row != nil && exist {
				nr := re.FindStringSubmatch(link)
				if len(nr) == 2 {
					row = append([]string{nr[1]}, row...)
				}
			}
			if row != nil {
				rows = append(rows, row)
				row = nil
			}
		})

	})
	return &rows
}

//Create from the html response a boatlist of available boats and store it in the boatList passed
func readboatList(booking *BookingInterface, boatList *BoatListInterface, htm *string) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(*htm))
	var row []string
	var rows [][]string
	re := regexp.MustCompile(`go\(this, "new", "make", ([0-9]+)\);$`)
	// Find the sunrise, sunset, min and max times allowed
	doc.Find("form").Each(func(base int, basehtml *goquery.Selection) {
		basehtml.Find("input").Each(func(baseint int, basein *goquery.Selection) {
			val, exists := basein.Attr("placeholder")
			if exists && val == ":Gebruikersnaam" {
				booking.Authorized = false
			}
		})
		basehtml.Find("img").Each(func(basesl int, baseselect *goquery.Selection) {
			val, exists := baseselect.Attr("src")
			//SunRise -15min truncated at 15min mark
			if exists && val == "css/solopp.gif" {
				thetime, _ := time.Parse(time.RFC3339, shortDate(booking.Date)+"T"+strings.Split(strings.TrimSpace(baseselect.Parent().Text()), "-")[0]+":00"+booking.TimeZone)
				boatList.SunRise = thetime.Add(-time.Minute * 15).Truncate(15 * time.Minute).Unix()
				//boatList.SunRise = thetime.Add(-time.Minute * 15).Round(15 * time.Minute).Unix()
			}
			//SunSet +15 min
			if exists && val == "css/solned.gif" {
				thetime, _ := time.Parse(time.RFC3339, shortDate(booking.Date)+"T"+strings.Split(strings.TrimSpace(baseselect.Parent().Text()), "+")[0]+":00"+booking.TimeZone)
				boatList.SunSet = thetime.Add(time.Minute * 15).Round(15 * time.Minute).Unix()
			}
		})
		basehtml.Find("select").Each(func(basesl int, baseselect *goquery.Selection) {
			val, exists := baseselect.Attr("id")
			if exists && val == "date" {
				//Get the max date of the last option
				baseselect.Find("option").Each(func(baseop int, baseoption *goquery.Selection) {
					val2, _ := baseoption.Attr("value")
					boatList.EpochDate, _ = strconv.ParseInt(val2, 10, 64)
				})
			}
			if exists && val == "start" {
				//Get the first start  of the option
				baseselect.Find("option").Each(func(baseop int, baseoption *goquery.Selection) {
					val2, _ := baseoption.Attr("value")
					if baseop == 0 {
						boatList.EpochStart, _ = strconv.ParseInt(val2, 10, 64)
					}
					ee, _ := strconv.ParseInt(val2, 10, 64)
					ee += int64(minDuration) * 60 //Add the minimal book time
					boatList.EpochEnd = MaxInt64(ee, boatList.EpochEnd)
				})
				baseselect.Find("optgroup").Each(func(baseop int, baseoption *goquery.Selection) {
					val2, _ := baseoption.Attr("label")
					thetime, _ := time.Parse(time.RFC3339, shortDate(booking.Date)+"T"+val2+":00"+booking.TimeZone)
					ee := thetime.Unix()
					ee += int64(minDuration) * 60 //Add the minimal book time
					boatList.EpochEnd = MaxInt64(ee, boatList.EpochEnd)
				})
			}
			if exists && val == "end" {
				//Get the first start  of the option
				baseselect.Find("option").Each(func(baseop int, baseoption *goquery.Selection) {
					val2, _ := baseoption.Attr("value")
					epochEnd, _ := strconv.ParseInt(val2, 10, 64)
					boatList.EpochEnd = MaxInt64(epochEnd, boatList.EpochEnd)
				})
			}
		})
	})

	// Find each table
	doc.Find("table").Each(func(base int, basehtml *goquery.Selection) {
		if basehtml.HasClass("rsrv_overview") {
			basehtml.Find("tr").Each(func(basetr int, baserow *goquery.Selection) {
				baserow.Find("td").Each(func(baseth int, basecell *goquery.Selection) {
					basecell.Find("table").Each(func(indextbl int, tablehtml *goquery.Selection) {
						tablehtml.Find("tr").Each(func(indextr int, rowhtml *goquery.Selection) {
							rowhtml.Find("td").Each(func(indexth int, tablecell *goquery.Selection) {
								s := strings.TrimSpace(tablecell.Text())
								if s != "" {
									row = append(row, s)
								}
							})
						})
					})
				})
				link, exist := baserow.Attr("onclick")
				if row != nil && exist {
					nr := re.FindStringSubmatch(link)
					if len(nr) == 2 {
						row = append([]string{nr[1]}, row...)
					}
				}
				if row != nil {
					rows = append(rows, row)
					row = nil
				}
			})
		}
	})
	//Save the rows as boats
	boatList.Boats = &rows
}

//Search a boat for the specified booking
func boatSearchByTime(booking *BookingInterface, starttime int64) (*BoatListInterface, error) {
	var filter string = "0"
	var boatList BoatListInterface
	if booking.BoatFilter != "" {
		filter = boatFilter[booking.BoatFilter]
	}
	boatList.BoatFilter = filter
	boatList.EpochDate = booking.EpochDate
	data := url.Values{}
	data.Set("action", "new")
	data.Set("exec", "")
	data.Set("id", "")
	data.Set("typeFilter", filter)
	data.Set("date", strconv.FormatInt(booking.EpochDate, 10))
	data.Set("start", strconv.FormatInt(starttime, 10))
	//data.Set("end", strconv.FormatInt(endtime, 10))
	data.Set("username", booking.Username)
	data.Set("password", booking.Password)
	data.Set("comment", "")
	request, err := http.NewRequest(http.MethodPost, baseUrl, strings.NewReader(data.Encode()))
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("DNT", "1")
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return nil, errors.New("HTTP Status is out of the 2xx range")
	}
	b, _ := io.ReadAll(response.Body)
	str := string(b)
	readboatList(booking, &boatList, &str)
	if !booking.Authorized {
		return &boatList, errors.New("user is not authorized")
	}
	return &boatList, err
}

//Default boat search for the specifed period
func boatSearch(booking *BookingInterface) (*BoatListInterface, error) {
	return boatSearchByTime(booking, booking.EpochDate)
}

func boatBook(booking *BookingInterface, starttime int64, endtime int64) error {
	//We need to have a boot id before we can do a booking
	if booking.BoatId == "" {
		boatList, err := boatSearchByTime(booking, starttime)
		if err != nil {
			return err
		}
		for _, bb := range *boatList.Boats { //Array element 2 is the boat name
			if strings.Contains(strings.ToLower(bb[2]), strings.ToLower(booking.Name)) {
				//Book the boat id is element 0
				booking.BoatId = bb[0]
				break
			}
		}
		if booking.BoatId == "" {
			return errors.New("no boatID found")
		}
	}

	data := url.Values{}
	data.Set("action", "new")
	data.Set("exec", "make")
	data.Set("id", booking.BoatId) //When making a boat booking we need to use the boatId
	data.Set("typeFilter", booking.BoatFilter)
	data.Set("date", strconv.FormatInt(booking.EpochDate, 10))
	data.Set("start", strconv.FormatInt(starttime, 10))
	data.Set("end", strconv.FormatInt(endtime, 10))
	data.Set("username", booking.Username)
	data.Set("password", booking.Password)
	data.Set("comment", commentPrefix+booking.Comment)
	request, err := http.NewRequest(http.MethodPost, baseUrl, strings.NewReader(data.Encode()))
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("DNT", "1")
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
	b, _ := io.ReadAll(response.Body)
	booking.BookingId = ""
	//<input type=button onclick="document.getElementById('loader').src='../gui/generateICS.php?rid=442863&invite=1';" value="iCal uitnodiging" /><br /></span></form>
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(string(b)))
	re := regexp.MustCompile(`.*?rid=([0-9]+).*;$`)
	// Find the sunrise, sunset, min and max times allowed
	doc.Find("input").Each(func(base int, baseinput *goquery.Selection) {
		val, exists := baseinput.Attr("value")
		//TODO: Read the response and set bookingId
		if exists && val == "iCal afspraak" {
			link, _ := baseinput.Attr("onclick")
			nr := re.FindStringSubmatch(link)
			booking.BookingId = nr[1]
		}
	})
	if booking.BookingId == "" {
		return errors.New("booking number could not befound")
	}
	return err
}

//Cancel a booking
func boatCancel(booking *BookingInterface) error {
	data := url.Values{}
	data.Set("action", "edit")
	data.Set("exec", "cancel") //Perhase also update, check javascript
	data.Set("id", booking.BookingId)
	data.Set("username", booking.Username)
	data.Set("password", booking.Password)
	data.Set("comment", commentPrefix+booking.Comment)
	request, err := http.NewRequest(http.MethodPost, baseUrl, strings.NewReader(data.Encode()))
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("DNT", "1")
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
	booking.State = "Canceled"
	return err
}

//Confirm a booking
func confirmBoat(booking *BookingInterface) error {
	log.Error("Confirm Boat not implemented")
	return nil
}

/*
//Do a update of the boat by canceling it and booking it again
func boatUpdate(booking *BookingInterface, starttime int64, endtime int64) error {
	err := boatCancel(booking)
	if err == nil {
		err = boatBook(booking, starttime, endtime)
	}
	return err
}
*/

func boatUpdate(booking *BookingInterface, startTime int64, endTime int64) error {
	//STEP: Session
	cookies, guiEpochStart, _, err := guiSession()
	if err != nil {
		return err
	}

	//STEP: Create Reference to the booking
	request, err := http.NewRequest(http.MethodGet, guiUrl, nil)
	values := request.URL.Query()
	values.Set("a", "e")
	values.Set("menu", "Rmenu")
	values.Set("extrainfo", "mid="+booking.BoatId+
		"&co=0&rid="+booking.BookingId+
		"&from="+strconv.FormatInt(int64((booking.EpochStart-guiEpochStart)/(15*60)), 10)+
		"&dur="+strconv.FormatInt(int64(booking.Duration/15), 10)+"&rec=0")
	request.URL.RawQuery = values.Encode()
	for _, o := range cookies {
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
	data.Set("newStart", strconv.FormatInt(int64((booking.EpochStart-guiEpochStart)/(15*60)), 10))
	data.Set("newEnd", strconv.FormatInt(int64((booking.EpochStart-guiEpochStart+booking.Duration*60)/(15*60)), 10))
	data.Set("clubcode", "")
	data.Set("username", booking.Username)
	data.Set("password", booking.Password)
	request, _ = http.NewRequest(http.MethodPost, guiUrl+"?a=e&menu=Rmenu&page=1_modifylogbook", strings.NewReader(data.Encode()))
	for _, o := range cookies {
		request.AddCookie(o)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("DNT", "1")
	//request.Header.Set("Referer", rawQuery)
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
	data.Set("newStart", strconv.FormatInt(int64((startTime-guiEpochStart)/(15*60)), 10))
	data.Set("newEnd", strconv.FormatInt(int64((endTime-guiEpochStart)/(15*60)), 10))
	data.Set("comment", commentPrefix+booking.Comment)
	data.Set("page", "3_commit")
	data.Set("act", "Ok")
	request, _ = http.NewRequest(http.MethodPost, guiUrl+"?a=e&menu=Amenu", strings.NewReader(data.Encode()))
	for _, o := range cookies {
		request.AddCookie(o)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("DNT", "1")
	//request.Header.Set("Referer", rawQuery)
	client = &http.Client{}
	response, err = client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}
	return nil
}

//Create a gui session to work on
func guiSession() ([]*http.Cookie, int64, string, error) {
	//Step 1: Get EportStart of GUI and the FleetID
	var guiEpochStart int64
	var guiFleetId string

	request, _ := http.NewRequest(http.MethodGet, guiUrl+"?language=NL&brsuser=-1&clubname=rvs", nil)
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, guiEpochStart, guiFleetId, err
	}
	defer response.Body.Close()
	cookies := response.Cookies()
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
						guiEpochStart = theTime.Unix()
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
			guiFleetId = re.FindStringSubmatch(val)[1]
		}
	})
	if guiFleetId == "" || guiEpochStart == 0 {
		return cookies, guiEpochStart, guiFleetId, errors.New("guiFleetId or guiEpochStart not found")
	}
	return cookies, guiEpochStart, guiFleetId, nil
}

//Logout for the specified booking
func logout(booking *BookingInterface) error {
	var err error
	if len(booking.Cookies) != 0 {
		var request *http.Request
		data := url.Values{}
		data.Set("action", "logout")
		data.Set("exec", "")
		data.Set("id", "")
		request, err = http.NewRequest(http.MethodPost, baseUrl, strings.NewReader(data.Encode()))
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
		booking.Cookies = response.Cookies()
	}
	return err
}

//Login for the specified booking and save the required cookie
func login(booking *BookingInterface) error {
	var err error
	var request *http.Request
	booking.Authorized = false
	//Step 1: Get Cookie
	var webUrl string = "https://my-fleet.eu/" + clubId + "/text/index.php?clubname=rvs&variant="
	request, err = http.NewRequest(http.MethodGet, webUrl, nil)
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
	booking.Cookies = response.Cookies()
	if len(booking.Cookies) == 0 {
		return errors.New("no session cookie")
	}

	//Step 2: It we will get the main pages including the bookinglist
	data := url.Values{}
	data.Set("action", "new")
	data.Set("exec", "")
	data.Set("id", "")
	data.Set("username", booking.Username)
	data.Set("password", booking.Password)
	request, err = http.NewRequest(http.MethodPost, baseUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	for _, o := range booking.Cookies {
		request.AddCookie(o)
	}
	response, err = client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		return errors.New("HTTP Status is out of the 2xx range")
	}
	b, _ := io.ReadAll(response.Body)
	str := string(b)
	booking.Bookings = readbookingList(&str)
	booking.Authorized = true
	return nil
}

//Read the boat list and create it if not found
func readBoatJson() []string {
	var b []string
	b = append(b, "No Boats")
	fs, err := os.Stat(boatFile)
	//We need to check if we have the boat file, load it for the first authorized
	if errors.Is(err, os.ErrNotExist) || !fs.ModTime().After(time.Now().Add(-24*time.Hour)) {
		cookies, _, guiFleetId, err := guiSession()
		if err != nil {
			log.Error("GuiSession Failed", err)
			return b
		}
		request, _ := http.NewRequest(http.MethodGet, guiUrl, nil)
		values := request.URL.Query()
		values.Set("a", "c")
		values.Set("uniq", guiFleetId)
		request.URL.RawQuery = values.Encode()
		for _, o := range cookies {
			request.AddCookie(o)
		}
		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			log.Error("Client session fleet script", err)
			return b
		}
		defer response.Body.Close()
		if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
			log.Error("Retrieve fleet script", err)
			return b
		}
		bd, _ := io.ReadAll(response.Body)
		str := string(bd)
		re := regexp.MustCompile(`var info=(.*);`)
		rem := re.FindStringSubmatch(str)
		if len(rem) > 0 {
			b = []string{}
			btl := strings.Split(rem[1], "<b>Bootnaam<\\/b>: ")
			for _, bt := range btl {
				bs := strings.Split(bt, "<br")
				if len(bs) > 1 {
					if !slices.Contains(b, strings.TrimSpace(bs[0])) {
						b = append(b, strings.TrimSpace(bs[0]))
					}
				}
			}
			if _, err := os.Stat(boatFile); os.IsNotExist(err) {
				err := os.MkdirAll(filepath.Dir(boatFile), 0644) // Create your file
				if err != nil {
					log.Fatal(err)
				}
			}
			json_to_file, _ := json.Marshal(b)
			mutex.Lock()
			err := ioutil.WriteFile(boatFile, json_to_file, 0644)
			mutex.Unlock()
			if err != nil {
				log.Fatal(err)
			}
			log.Info("Boat list created")
		}
		return b
	}

	file, err := ioutil.ReadFile(boatFile)
	if err != nil {
		log.Error(err)
	} else {
		err = json.Unmarshal(file, &b)
		if err != nil {
			log.Error(err)
		}
	}
	return b
}

//Read the  user info
func readUsersJson() []UserInterface {
	var b []UserInterface
	var u UserInterface = UserInterface{Username: "?", Password: "?"}
	b = append(b, u)
	if _, err := os.Stat(boatFile); errors.Is(err, os.ErrNotExist) {
		return b
	}
	file, err := ioutil.ReadFile(userFile)
	if err == nil {
		err = json.Unmarshal(file, &b)
		if err != nil {
			log.Error(err)
		}
	}
	return b
}

//Write the user info to file
func writeUsersJson(data []UserInterface) {
	if _, err := os.Stat(userFile); os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(userFile), 0644) // Create your file
		if err != nil {
			log.Fatal(err)
		}
	}
	for i := len(data) - 1; i >= 0; i-- {
		if data[i].Username == "?" || data[i].LastUsed < time.Now().Add(-30*24*time.Hour).Unix() {
			data = append(data[:i], data[i+1:]...)
		}
	}
	json_to_file, _ := json.Marshal(data)
	mutex.Lock()
	err := ioutil.WriteFile(userFile, json_to_file, 0644)
	mutex.Unlock()
	if err != nil {
		log.Fatal(err)
	}
}

//Read the booking informatio
func readJson() BookingSlice {
	b := BookingSlice{}
	if _, err := os.Stat(bookingFile); errors.Is(err, os.ErrNotExist) {
		return b
	}
	mutex.Lock()
	file, err := ioutil.ReadFile(bookingFile)
	mutex.Unlock()
	if err != nil {
		log.Error(err)
	} else {
		err = json.Unmarshal(file, &b)
		if err != nil {
			//We have a error try to recover the backup file
			log.Error(err)
			if _, err := os.Stat(bookingFile + ".bak"); errors.Is(err, os.ErrNotExist) {
				writeJson(b)
			} else {
				mutex.Lock()
				file, _ = ioutil.ReadFile(bookingFile + ".bak")
				mutex.Unlock()
				err = json.Unmarshal(file, &b)
				if err != nil {
					writeJson(b)
				}
			}
		}
	}
	return b
}

//Write the data to the booking file, removing expired data
func writeJson(data BookingSlice) {
	if _, err := os.Stat(bookingFile); os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(bookingFile), 0644) // Create your file
		if err != nil {
			log.Fatal(err)
		}
	}
	for i := len(data) - 1; i >= 0; i-- {
		if data[i].State == "Delete" {
			log.WithFields(log.Fields{
				"state": data[i].State,
				"boat":  data[i].Name,
				"user":  data[i].Username,
				"at":    shortDate(data[i].Date),
				"from":  shortTime(data[i].Time),
			}).Info("Deleting")
			data = append(data[:i], data[i+1:]...)
		}
	}
	json_to_file, _ := json.Marshal(data)
	mutex.Lock()
	os.Rename(bookingFile, bookingFile+".bak")
	err := ioutil.WriteFile(bookingFile, json_to_file, 0644)
	mutex.Unlock()
	if err != nil {
		log.Fatal(err)
	}
}

//Function where al checks are done for a single booking and make the booking
func doBooking(b *BookingInterface) (changed bool, err error) {
	loc, _ := time.LoadLocation(timeZoneLoc)
	//Step 2a: Check if we have a booking for the requested boat date and time
	for _, bb := range *b.Bookings { //Array element 5 is the boat name

		if strings.Contains(strings.ToLower(bb[5]), strings.ToLower(b.Name)) &&
			bb[1] == shortDate(b.Date) {

			//Convert the current start end times to Epoch
			times := strings.Fields(bb[2]) //10:00 - 12:00
			//log.Println("boat", bb)
			thetime, _ := time.Parse(time.RFC3339, shortDate(b.Date)+"T"+times[0]+":00"+b.TimeZone)
			startTime := thetime.Unix()
			thetime, _ = time.Parse(time.RFC3339, shortDate(b.Date)+"T"+times[2]+":00"+b.TimeZone)
			endTime := thetime.Unix()

			//Check if the booking contains the commment created or specified
			if len(bb) < 7 || !strings.EqualFold(bb[6], commentPrefix+b.Comment) {
				//Check if there is a blockage
				if ((b.EpochStart > startTime && b.EpochStart <= endTime) ||
					(b.EpochEnd > startTime && b.EpochEnd <= endTime)) &&
					b.State != "Blocked" {
					if b.State == "Moving" {
						log.WithFields(log.Fields{
							"state": b.State,
							"boat":  b.Name,
							"user":  b.Username,
							"at":    shortDate(b.Date),
							"from":  shortTime(b.Time),
						}).Info("Canceled because of blocked")
						err = boatCancel(b)
						if err != nil {
							return true, err
						}
					} else {
						b.State = "Blocked"
					}
					return true, errors.New("booking blocked by " + bb[3])
				}
				//log.Println("Skip", b.Name, bb[3], bb[1], bb[2], "not the correct booking")
				//Skip to next boat because we are not looking for this one
				continue
			}
			//Boat is ours
			b.BookingId = bb[0]
			b.BoatFilter = bb[4]

			//Check if we should cancel the boot
			if b.State == "Cancel" {
				err = boatCancel(b)
				if err == nil {
					b.State = "Canceled"
				}
				return true, err
			}

			//Check if we should move this boat
			if b.EpochStart == startTime && b.EpochEnd == endTime {
				//Boat is on correct time and duration
				return false, nil
			}

			//Find out the minTime and maxTime we our allowed to move the boat for the given day
			boatList, err := boatSearch(b)
			if err != nil {
				return false, err
			}

			newEndTime := MinInt64(boatList.EpochEnd, b.EpochEnd)
			newStartTime := MinInt64(b.EpochStart, MinInt64(b.EpochStart, newEndTime-b.Duration*60))
			newStartTime = MaxInt64(newStartTime, boatList.SunRise)
			//log.Println("Epoch", startTime, newStartTime, endTime, newEndTime, b.EpochStart, b.EpochEnd, boatList.EpochStart, boatList.EpochEnd, boatList.SunRise, boatList.SunSet, bb)

			//Check if their is a reason to update the booking

			if newStartTime > startTime || newEndTime > endTime {
				err = boatUpdate(b, newStartTime, newEndTime)
				if err != nil {
					b.State = "Retry"
				} else {
					b.Message = "At:" + time.Unix(newStartTime, 0).In(loc).Format("15:04") + " - " + time.Unix(newEndTime, 0).In(loc).Format("15:04")
					if b.EpochStart == newStartTime && b.EpochEnd == newEndTime {
						b.State = "Finished"
					} else {
						b.State = "Moving"

					}
				}
				return true, err
			}
			//We found the boat but not updated it on it so we continue
			return false, nil
		}
	}

	//Check if we should just cancel the book which is not yet their
	if b.State == "Cancel" {
		b.State = "Canceled"
		return true, nil
	}

	//Check if we should mark record for removal, after 12 hours
	if b.EpochEnd < time.Now().Add(-time.Hour*12).Unix() {
		//log.Println("Delete", b.EpochEnd, "<", time.Now().Add(-time.Hour*12).Unix())
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

	//Step 2b: When there is no booking, check if we are allowed to add it
	boatList, err := boatSearch(b)
	if err != nil {
		b.State = "Failed"
		return true, err
	}

	//log.Println(boatList.EpochDate, boatList.EpochStart, boatList.EpochEnd, *boatList.Boats)
	//Check if we have a boatList for the correct day, if not exit it
	if boatList.EpochDate < b.EpochDate {
		b.Message = "Date not valid yet"
		b.State = "Waiting"
		b.EpochNext = time.Unix(boatList.SunRise, 0).Add(-(time.Duration(bookWindow)) * time.Hour).Truncate(15 * time.Minute).Unix()
		return true, nil
	}

	//Check if we would be allowed booking, we need to be after Sunrise
	if time.Unix(boatList.EpochEnd, 0).Add(-time.Duration(minDuration)*time.Minute).Unix() < boatList.SunRise {
		b.Message = "Starttime before Sunrise"
		b.State = "Waiting"
		b.EpochNext = time.Unix(boatList.SunRise, 0).Add(-(time.Duration(bookWindow)*time.Hour - time.Duration(minDuration)*time.Minute)).Truncate(15 * time.Minute).Unix()
		return true, nil
	}

	//Calculate the minimal start and end time
	endtime := MinInt64(boatList.EpochEnd, b.EpochEnd)
	starttime := MinInt64(b.EpochStart, MinInt64(b.EpochStart, endtime-b.Duration*60))
	starttime = MaxInt64(starttime, boatList.SunRise)

	//Check if we are allowed to book this
	if endtime-starttime < int64(minDuration*60) {
		b.Message = "Time between start and end, <" + strconv.FormatInt(int64(minDuration), 10) + "min"
		b.State = "Waiting"
		b.EpochNext = time.Now().Unix() - (endtime - starttime)
		return true, nil
	}

	//Load the boatList for the need time
	boatList, err = boatSearchByTime(b, starttime)
	if err != nil {
		return false, err
	}

	//Issue when selection all boot

	//log.Println(boatList.EpochDate, boatList.EpochStart, boatList.EpochEnd, *boatList.Boats)
	//Check if the boot is available requested period
	for _, bb := range *boatList.Boats { //Array element 2 is the boat name
		if strings.Contains(strings.ToLower(bb[2]), strings.ToLower(b.Name)) {
			//Book the boat id is element 0
			b.BoatId = bb[0]
			b.BoatFilter = boatFilter[bb[1]]
			err := boatBook(b, starttime, endtime)
			if err == nil {
				loc, _ := time.LoadLocation(timeZoneLoc)
				b.Message = "At:" + time.Unix(starttime, 0).In(loc).Format("15:04") + " - " + time.Unix(endtime, 0).In(loc).Format("15:04")
				if b.EpochStart == starttime && b.EpochEnd == endtime {
					b.State = "Finished"
				} else {
					b.State = "Moving"
				}
			}
			return err == nil, err
		}
	}

	err = errors.New("boat not in available list")
	if b.State != "Retry" {
		log.WithFields(log.Fields{
			"state":              b.State,
			"boat":               b.Name,
			"user":               b.Username,
			"at":                 shortDate(b.Date),
			"from":               shortTime(b.Time),
			"starttime":          starttime,
			"endtime":            endtime,
			"boatlistSunRise":    boatList.SunRise,
			"boatlistEpochDate":  boatList.EpochDate,
			"boatlistEpochStart": boatList.EpochStart,
			"boatlistEpochEnd":   boatList.EpochEnd,
			"boats":              *boatList.Boats,
		}).Info("Retry data")
		b.State = "Retry"
		b.Retry++
		return true, err
	}
	// Stop Retry after the boat.EpochEnd is lager than b.EpochEnd --> Failed
	if boatList.EpochEnd > b.EpochEnd {
		b.State = "Failed"
		return true, err
	}
	return false, err
}

//The main loop in which we do all the booking processing
func bookLoop() {
	log.Println("Start processing")
	var changed bool = false
	//Timing loop
	for {
		//TODO: We should read it from file or json url
		bookingSlice := readJson()
		for i, booking := range bookingSlice {
			//Step 0: Data convertions

			//Set the timezone
			//TODO: Do Winter and Summer time checking
			booking.TimeZone = timeZone

			//Set the correct EpochDatas
			thetime, err := time.Parse(time.RFC3339, shortDate(booking.Date)+"T00:00:00"+booking.TimeZone)
			if err != nil {
				log.Error("date not valid yyyy-MM-dd")
				booking.State = "Failed"
				booking.Message = "date not valid yyyy-MM-dd"
				changed = true
				bookingSlice[i] = booking
				continue
			}
			booking.EpochDate = thetime.Unix()
			thetime, err = time.Parse(time.RFC3339, shortDate(booking.Date)+"T"+shortTime(booking.Time)+":00"+booking.TimeZone)
			if err != nil {
				log.Error("time not valid hh:mm")
				booking.State = "Failed"
				booking.Message = "time not valid hh:mm"
				changed = true
				bookingSlice[i] = booking
				continue
			}

			//Set the minimal duration
			if booking.Duration < int64(minDuration) {
				booking.Duration = int64(minDuration)
			}
			//Set the maximal duration
			if booking.Duration > int64(maxDuration) {
				booking.Duration = int64(maxDuration)
			}

			booking.EpochStart = thetime.Unix()
			thetime = thetime.Add(time.Minute * time.Duration(booking.Duration))
			booking.EpochEnd = thetime.Unix()

			//Check if we should confirm the booking
			if booking.State == "Finished" &&
				time.Unix(booking.EpochStart, 0).Add(-15*time.Minute).Unix() >= time.Now().Unix() &&
				time.Unix(booking.EpochStart, 0).Unix() <= time.Now().Unix() {
				err = confirmBoat(&booking)
				if err == nil {
					booking.State = "Confirmed"
					booking.Message = "Booking confirmed"
					bookingSlice[i] = booking
					changed = true
					continue
				}
			}

			//Check if have allready processed the booking, if so skip it
			if booking.State == "Finished" || booking.State == "Comfirmed" || booking.State == "Canceled" ||
				booking.State == "Failed" || booking.State == "Blocked" || booking.EpochNext > time.Now().Unix() {
				//log.Println(booking.State, booking.Name, booking.Username, booking.Date, booking.Time)
				//Check if we should mark record for removal, after 12 hours
				if booking.EpochEnd < time.Now().Add(-time.Hour*12).Unix() {
					//log.Println("Delete", b.EpochEnd, "<", time.Now().Add(-time.Hour*12).Unix())
					booking.State = "Delete"
					booking.Message = "Booking marked for Delete"
					bookingSlice[i] = booking
					changed = true
				}
				continue
			}

			//Check if comment is set, if not fill default
			if !booking.UserComment {
				booking.Comment = shortTime(booking.Time) + " - " + thetime.Format("15:04")
			}

			//The message will be rest after every run
			booking.Message = ""

			//Step 1: Login
			err = login(&booking)
			if err != nil {
				log.Error(err)
				continue
			}

			//Step 2: doBooking
			vchanged, err := doBooking(&booking)
			if err != nil {
				booking.Retry++
				booking.State = "Error"
				booking.Message = err.Error()
				vchanged = true
			}

			//If data has been changed update the booking array
			if vchanged {
				changed = true
				//Sleep the booking for at least 15 min
				booking.EpochNext = MaxInt64(booking.EpochNext, time.Now().Add(15*time.Minute).Truncate(15*time.Minute).Unix())
				bookingSlice[i] = booking
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
					}).Error(err)
				} else {
					log.WithFields(log.Fields{
						"state": booking.State,
						"boat":  booking.Name,
						"user":  booking.Username,
						"at":    shortDate(booking.Date),
						"from":  shortTime(booking.Time),
						"next":  shortTime(nextStr),
					}).Info(booking.Message)
				}
			}

			//Step 3: logout
			logout(&booking)
		}

		//Save the change to the bookingFile on changed data
		if changed {
			writeJson(bookingSlice)
		}

		//Exit if we are in single run mode
		if singleRun {
			break
		}
		//We sleep before we restart, where we align as close as possible to interval, but always 5 sec for offset
		time.Sleep(time.Duration(time.Now().Add(time.Duration(refreshInterval)*time.Minute).Round(time.Duration(refreshInterval)*time.Minute).Add(5*time.Second).Unix()-time.Now().Unix()) * time.Second)
		//log.Println("Awake from Sleep", refreshInterval)
	}
}

//Indicate which CORS sites are allowed
func allowOrigin(origin string) (bool, error) {
	// In this example we use a regular expression but we can imagine various
	// kind of custom logic. For example, an external datasource could be used
	// to maintain the list of allowed origins.
	return true, nil //regexp.MatchString(`^https:\/\/spaarne\.(\w).(\w)$`, origin)
}

//Function to create log entry
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

//Middleware logging services
func middlewareLogging(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		makeLogEntry(c).Debug("incoming request")
		return next(c)
	}
}

//Error handler for JsonSer er
func errorHandler(err error, c echo.Context) {
	report, ok := err.(*echo.HTTPError)
	if ok {
		report.Message = fmt.Sprintf("http error %d - %v", report.Code, report.Message)
	} else {
		report = echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	makeLogEntry(c).Error(report.Message)
	c.HTML(report.Code, report.Message.(string))
}

//The basic web server
func jsonServer() error {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middlewareLogging)
	e.HTTPErrorHandler = errorHandler
	g := e.Group("/data")
	if jsonProtect {
		g.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == jsonUser && password == jsonPwd {
				return true, nil
			}
			return false, nil
		}))
	}

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: allowOrigin,
		AllowMethods:    []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	e.GET("/data/booking", func(c echo.Context) error {
		bookings := readJson()
		return c.JSON(http.StatusOK, bookings)
	})
	e.GET("/data/boat", func(c echo.Context) error {
		boats := readBoatJson()
		return c.JSON(http.StatusOK, boats)
	})

	e.GET("/data/config", func(c echo.Context) error {
		var versionData = map[string]string{"version": Version,
			"interval": strconv.FormatInt(int64(refreshInterval), 10),
			"prefix":   commentPrefix,
			"clubid":   clubId,
			"timezone": timeZone,
		}
		return c.JSON(http.StatusOK, versionData)
	})

	e.GET("/data/booking/:id", func(c echo.Context) error {
		bookings := readJson()

		for _, booking := range bookings {
			if c.Param("id") == strconv.FormatInt(booking.Id, 10) {
				return c.JSON(http.StatusOK, booking)
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	//Protected requests
	g.GET("/users", func(c echo.Context) error {
		users := readUsersJson()
		return c.JSON(http.StatusOK, users)
	})

	g.POST("/booking", func(c echo.Context) error {
		bookings := readJson()
		//Autoincrement booking id
		var id int64 = 0
		for _, booking := range bookings {
			id = MaxInt64(id, booking.Id+1)
		}
		new_booking := new(BookingInterface)
		new_booking.Id = id
		new_booking.State = ""
		new_booking.Message = ""
		new_booking.EpochNext = 0
		new_booking.UserComment = new_booking.Comment != ""
		err := c.Bind(new_booking)
		if err != nil {
			return c.String(http.StatusBadRequest, "Bad request.")
		}

		//Round the time to the closed one
		if strings.Contains(new_booking.Time, "T") {
			thetime, _ := time.Parse(time.RFC3339, new_booking.Time)
			new_booking.Time = thetime.Round(15 * time.Minute).Format(time.RFC3339)
		}

		bookings = append(bookings, *new_booking)
		writeJson(bookings)
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
			if strings.EqualFold(usr.Username, new_booking.Username) {
				users[i].Password = new_booking.Password
				users[i].LastUsed = time.Now().Unix()
				found = true
				break
			}
		}
		if !found {
			users = append(users, UserInterface{Username: new_booking.Username, Password: new_booking.Password, LastUsed: time.Now().Unix()})
		}
		writeUsersJson(users)
		return c.JSON(http.StatusOK, bookings)
	})

	g.PUT("/booking/:id", func(c echo.Context) error {
		bookings := readJson()

		updated_booking := new(BookingInterface)
		err := c.Bind(updated_booking)
		if err != nil {
			log.Error(err, updated_booking)
			return c.String(http.StatusBadRequest, "Bad request.")
		}
		updated_booking.EpochNext = 0
		updated_booking.State = ""
		updated_booking.Message = ""
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
			if strings.EqualFold(usr.Username, updated_booking.Username) {
				users[i].Password = updated_booking.Password
				users[i].LastUsed = time.Now().Unix()
				found = true
				break
			}
		}
		if !found {
			users = append(users, UserInterface{Username: updated_booking.Username, Password: updated_booking.Password, LastUsed: time.Now().Unix()})
		}
		writeUsersJson(users)

		for i, booking := range bookings {
			if strconv.FormatInt(booking.Id, 10) == c.Param("id") {
				bookings = append(bookings[:i], bookings[i+1:]...)
				//Cancel a Boat when you update it, while it is finished
				if booking.State == "Finished" || booking.State == "Confirmed" {
					boatCancel(&booking)
				}
				bookings = append(bookings, *updated_booking)
				writeJson(bookings)
				return c.JSON(http.StatusOK, bookings)
			}
		}

		return c.String(http.StatusNotFound, "Not found.")
	})

	g.DELETE("/booking/:id", func(c echo.Context) error {
		bookings := readJson()

		for i, booking := range bookings {
			if strconv.FormatInt(booking.Id, 10) == c.Param("id") {
				if booking.State == "Failed" || booking.State == "Waiting" ||
					booking.State == "" || booking.State == "Canceled" ||
					booking.State == "Blocked" || booking.State == "Error" {
					log.WithFields(log.Fields{
						"state": booking.State,
						"boat":  booking.Name,
						"user":  booking.Username,
						"at":    shortDate(booking.Date),
						"from":  shortTime(booking.Time),
					}).Info("Deleting")
					bookings = append(bookings[:i], bookings[i+1:]...)
					writeJson(bookings)
				} else if booking.State != "Cancel" {
					booking.State = "Cancel"
					booking.EpochNext = 0
					bookings[i] = booking
					writeJson(bookings)
				}
				return c.JSON(http.StatusOK, bookings)
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	//Serve the app
	e.Static("/", "public")
	log.Printf("Start jsonserver on %s", bindAddress)
	return e.Start(bindAddress)
}

func main() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Waiting for clean Exit")
		mutex.Lock()
		os.Exit(1)
	}()
	Init()
	if !singleRun {
		go bookLoop()
		err := jsonServer()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		switch test {
		case "login_response":
			file, _ := ioutil.ReadFile("html/login-response.html")
			str := string(file)
			log.Println("login_response", readbookingList(&str))
		case "update":
			booking := BookingInterface{BoatId: "75", Name: "d'Armandville", BookingId: "443333", TimeZone: timeZone,
				Duration: 90, Username: "SP3426", Password: "SP3426", Date: "2022-08-02", Time: "10:00", Comment: "Sierk"}
			thetime, _ := time.Parse(time.RFC3339, shortDate(booking.Date)+"T"+shortTime(booking.Time)+":00"+booking.TimeZone)
			booking.EpochStart = thetime.Unix()
			err := boatUpdate(&booking, thetime.Add(15*time.Minute).Unix(), thetime.Add(time.Duration(15+booking.Duration)*time.Minute).Unix())
			log.Println("C2", booking.Cookies)
			if err != nil {
				log.Fatal(err)
			}
		case "boatlist":
			os.Remove(boatFile)
			log.Println("BoatList", readBoatJson())
		default:
			bookLoop()
		}
	}
}
