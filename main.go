package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

var Version = "0.0.1"

var baseUrl = "https://my-fleet.eu/R1B34/mobile/index0.php?&system=mobile&language=NL"
var bookingFile = "json/booking.json"
var timeZone = "+02:00"
var minDuration = 60 //The minimal duration required to book

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

//Struc used to store boat and session info
type BookingInterface struct {
	Id         int            `json:"id"`
	Name       string         `json:"boat"`
	Date       string         `json:"date"`
	Time       string         `json:"time"`
	Duration   int64          `json:"duration"`
	Username   string         `json:"user"`
	Password   string         `json:"password"`
	Comment    string         `json:"comment"`
	State      string         `json:"state,omitempty"`
	BookingId  string         `json:"bookingid,omitempty"`
	BoatId     string         `json:"boatid,omitempty"`
	BoatFilter string         `json:"boatfilter,omitempty"`
	TimeZone   string         `json:"-"`
	Cookies    []*http.Cookie `json:"-"`
	Bookings   *[][]string    `json:"-"`
	EpochDate  int64          `json:"-"`
	EpochStart int64          `json:"-"`
	EpochEnd   int64          `json:"-"`
	Message    string         `json:"message,omitempty"`
}

type BookingSlice []BookingInterface

type BoatListInterface struct {
	BoatFilter string
	SunRise    int64
	SunSet     int64
	EpochDate  int64
	EpochStart int64
	EpochEnd   int64
	Boats      *[][]string
}

var singleRun bool = true
var sleepInterval int = 5
var commentPrefix string = "#:"
var bindAddress string = ":1323"

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

//Read and set settings
func Init() {
	version := flag.Bool("version", false, "Prints current version ("+Version+")")
	flag.BoolVar(&singleRun, "singleRun", singleRun, "Should we only do one run")
	flag.StringVar(&commentPrefix, "prefix", commentPrefix, "Comment prefix")
	flag.StringVar(&timeZone, "timezone", timeZone, "The timezone used by user")
	flag.StringVar(&bindAddress, "bind", bindAddress, "The bind address to be used for webserver")
	flag.Parse() // after declaring flags we need to call it
	if *version {
		log.Println("Version ", Version)
		os.Exit(0)
	}
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
		basehtml.Find("img").Each(func(basesl int, baseselect *goquery.Selection) {
			val, exists := baseselect.Attr("src")
			//SunRise -15min
			if exists && val == "css/solopp.gif" {
				thetime, _ := time.Parse(time.RFC3339, booking.Date+"T"+strings.Split(strings.TrimSpace(baseselect.Parent().Text()), "-")[0]+":00"+booking.TimeZone)
				boatList.SunRise = thetime.Add(-time.Minute * 15).Unix()
			}
			//SunSet +15 min
			if exists && val == "css/solned.gif" {
				thetime, _ := time.Parse(time.RFC3339, booking.Date+"T"+strings.Split(strings.TrimSpace(baseselect.Parent().Text()), "+")[0]+":00"+booking.TimeZone)
				boatList.SunSet = thetime.Add(time.Minute * 15).Unix()
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
					boatList.EpochEnd, _ = strconv.ParseInt(val2, 10, 64)
					boatList.EpochEnd += int64(minDuration) * 60 //Add the minimal book time
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
func boatSearchByTime(booking *BookingInterface, starttime int64, endtime int64) (*BoatListInterface, error) {
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
	data.Set("end", strconv.FormatInt(endtime, 10))
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
	return &boatList, err
}

//Default boat search for the specifed period
func boatSearch(booking *BookingInterface) (*BoatListInterface, error) {
	return boatSearchByTime(booking, booking.EpochStart, booking.EpochEnd)
}

func boatBook(booking *BookingInterface, starttime int64, endtime int64) error {
	//We need to have a boot id before we can do a booking
	if booking.BoatId == "" {
		boatList, _ := boatSearchByTime(booking, starttime, endtime)
		for _, bb := range *boatList.Boats { //Array element 2 is the boat name
			if strings.Index(bb[2], booking.Name) == 0 {
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

/* We still should try to find a way to edit the booking instead of createing a new one
cancel
action: edit
exec: cancel
id: 442762

Change in screen
newStart: 76
newEnd: 82
*/

//Do a update of the boat by canceling it and booking it again
func boatUpdate(booking *BookingInterface, starttime int64, endtime int64) error {
	err := boatCancel(booking)
	if err == nil {
		err = boatBook(booking, starttime, endtime)
	}
	return err
}

/*
func boatUpdate(booking *BookingInterface, starttime int64, endtime int64) error {
	data := url.Values{}
	data.Set("action", "edit")
	data.Set("exec", "change") //Perhase also update, check javascript
	data.Set("id", booking.BookingId)
	data.Set("typeFilter", "0") //All boats by default
	data.Set("date", strconv.FormatInt(booking.EpochDate, 10))
	data.Set("start", strconv.FormatInt(starttime, 10))
	data.Set("end", strconv.FormatInt(endtime, 10))
	//	data.Set("newStart", strconv.FormatInt(starttime, 10))
	//	data.Set("newEnd", strconv.FormatInt(endtime, 10))
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
	str := string(b)
	log.Println(booking.EpochDate, starttime, endtime)
	log.Println("## UPDATE ## ", str)
	//TODO: We should still check result
	return err
}
*/

func doBooking(b *BookingInterface) (changed bool, err error) {

	//Step 2a: Check if we have a booking for the requested boat date and time
	for _, bb := range *b.Bookings { //Array element 5 is the boat name
		if strings.Index(bb[5], b.Name) == 0 && bb[1] == b.Date {
			//Check if the booking contains the commment created or specified
			if len(bb) < 7 || bb[6] != (commentPrefix+b.Comment) {
				log.Println("Skip", b.Name, bb[3], bb[1], bb[2], "not the correct booking")
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
			//Convert the current start end times to Epoch
			times := strings.Fields(bb[2]) //10:00 - 12:00
			thetime, _ := time.Parse(time.RFC3339, b.Date+"T"+times[0]+":00"+b.TimeZone)
			startTime := thetime.Unix()
			thetime, _ = time.Parse(time.RFC3339, b.Date+"T"+times[2]+":00"+b.TimeZone)
			endTime := thetime.Unix()

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

			//Check if their is a reason to update the booking
			if (startTime != newStartTime || endTime != newEndTime) && newEndTime != newStartTime {
				err = boatUpdate(b, newStartTime, newEndTime)
				if err != nil {
					b.State = "Retry"
				} else {
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

	//Check fail a booking in the past
	if b.EpochDate < time.Now().Unix() {
		b.State = "Failed"
		b.Message = "Booking, not ready and now in the past"
		return true, nil
	}

	//Step 2b: When there is no booking, check if we are allowed to add it
	boatList, _ := boatSearch(b)
	//log.Println(boatList.EpochDate, boatList.EpochStart, boatList.EpochEnd, *boatList.Boats)
	//Check if we have a boatList for the correct day, if not exit it
	if boatList.EpochDate < b.EpochDate {
		//log.Println("Date not valid yet", boatList.EpochDate, b.EpochDate)
		b.State = "Waiting"
		return false, nil
	}
	//Calculate the minimal start and end time
	endtime := MinInt64(boatList.EpochEnd, b.EpochEnd)
	starttime := MinInt64(b.EpochStart, MinInt64(b.EpochStart, endtime-b.Duration*60))
	starttime = MaxInt64(starttime, boatList.SunRise)

	//Check if we are allowed to book this
	if endtime-starttime < int64(minDuration*60) || starttime < boatList.SunRise {
		log.Println("Booking not yet possible", starttime, endtime, boatList.SunRise)
		b.State = "Waiting"
		return false, nil
	}

	//Load the boatList for the need time
	boatList, _ = boatSearchByTime(b, starttime, endtime)
	//log.Println(boatList.EpochDate, boatList.EpochStart, boatList.EpochEnd, *boatList.Boats)
	//Check if the boot is available requested period
	for _, bb := range *boatList.Boats { //Array element 2 is the boat name
		if strings.Index(bb[2], b.Name) == 0 {
			//Book the boat id is element 0
			b.BoatId = bb[0]
			err := boatBook(b, starttime, endtime)
			if b.EpochStart == starttime && b.EpochEnd == endtime {
				b.State = "Finished"
			} else {
				b.State = "Moving"
			}
			return err == nil, err
		}
	}
	b.State = "Failed"
	return true, errors.New("boat not available")
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

	//Step 1: Get Cookie
	var webUrl string = "https://my-fleet.eu/R1B34/text/index.php?clubname=rvs&variant="
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
	return nil
}

func readJson() BookingSlice {
	b := BookingSlice{}
	if _, err := os.Stat(bookingFile); errors.Is(err, os.ErrNotExist) {
		return b
	}
	file, err := ioutil.ReadFile(bookingFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(file, &b)
	if err != nil {
		log.Fatal(err)
	}
	return b
}

func writeJson(data BookingSlice) {
	if _, err := os.Stat(bookingFile); os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(bookingFile), 0644) // Create your file
		if err != nil {
			log.Fatal(err)
		}
	}
	json_to_file, _ := json.Marshal(data)
	err := ioutil.WriteFile(bookingFile, json_to_file, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func jsonServer() {
	e := echo.New()

	e.GET("/booking", func(c echo.Context) error {
		bookings := readJson()

		return c.JSON(http.StatusOK, bookings)
	})

	e.GET("/booking/:id", func(c echo.Context) error {
		bookings := readJson()

		for _, booking := range bookings {
			if c.Param("id") == strconv.Itoa(booking.Id) {
				return c.JSON(http.StatusOK, booking)
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	e.POST("/booking", func(c echo.Context) error {
		bookings := readJson()

		new_booking := new(BookingInterface)
		err := c.Bind(new_booking)
		if err != nil {
			return c.String(http.StatusBadRequest, "Bad request.")
		}

		bookings = append(bookings, *new_booking)
		writeJson(bookings)

		return c.JSON(http.StatusOK, bookings)
	})

	e.PUT("/magazines/:id", func(c echo.Context) error {
		bookings := readJson()

		updated_booking := new(BookingInterface)
		err := c.Bind(updated_booking)
		if err != nil {
			return c.String(http.StatusBadRequest, "Bad request.")
		}

		for i, booking := range bookings {
			if strconv.Itoa(booking.Id) == c.Param("id") {
				bookings = append(bookings[:i], bookings[i+1:]...)
				bookings = append(bookings, *updated_booking)

				writeJson(bookings)

				return c.JSON(http.StatusOK, bookings)
			}
		}

		return c.String(http.StatusNotFound, "Not found.")
	})

	e.DELETE("/booking/:id", func(c echo.Context) error {
		bookings := readJson()

		for i, booking := range bookings {
			if strconv.Itoa(booking.Id) == c.Param("id") {
				bookings = append(bookings[:i], bookings[i+1:]...)
				writeJson(bookings)

				return c.JSON(http.StatusOK, bookings)
			}
		}
		return c.String(http.StatusNotFound, "Not found.")
	})

	e.Start(bindAddress)
}

func main() {
	//log.SetFormatter(&log.JSONFormatter{})

	Init()

	if !singleRun {
		jsonServer()
		log.Println("Json Server started on ", bindAddress)
	}

	var changed bool = false
	//Timing loop
	for {
		//TODO: We should read it from file or json url
		bookingSlice := readJson()
		for i, booking := range bookingSlice {
			//Check if have allready processed the booking, if so skip it
			if booking.State == "Finished" || booking.State == "Canceled" || booking.State == "Failed" {
				continue
			}
			//Step 0: Data convertions

			//Set the timezone
			//TODO: Do Winter and Summer time checking
			booking.TimeZone = timeZone

			//Set the correct EpochDatas
			thetime, err := time.Parse(time.RFC3339, booking.Date+"T00:00:00"+booking.TimeZone)
			if err != nil {
				log.Error("start not valid yyyy-MM-dd")
				continue
			}
			booking.EpochDate = thetime.Unix()
			thetime, err = time.Parse(time.RFC3339, booking.Date+"T"+booking.Time+":00"+booking.TimeZone)
			if err != nil {
				log.Error("time not valid hh:mm")
				continue
			}
			booking.EpochStart = thetime.Unix()
			thetime = thetime.Add(time.Minute * time.Duration(booking.Duration))
			booking.EpochEnd = thetime.Unix()

			//Check if comment is set, if not fill default
			if booking.Comment == "" {
				booking.Comment = booking.Time + " - " + thetime.Format("15:04")
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
				booking.Message = err.Error()
				log.Error(err)
			}

			//If data has been changed update the booking array
			if vchanged {
				changed = true
				bookingSlice[i] = booking
			}
			log.Println(booking.State, booking.Name, booking.Username, booking.Date, booking.Time)

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
		//We sleep before we restart,
		//TODO: align it on the 0,15,30,45 min mark
		time.Sleep(time.Minute * time.Duration(sleepInterval))
	}
}