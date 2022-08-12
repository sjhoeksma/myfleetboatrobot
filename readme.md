# MyFleet Robot
With is program you can automate booking a element of you club if the fleet is managed by: [my-feel.eu](https://my-fleet.eu/).

The default values within the system are for the club **HetSpaarne** but you can change it by setting the clubId parameter.
 
## Building Development

For development you can use to run the backend part. Default server port is 1323
```
go run main.go -singleRun=false
```
and for the front end you can use the command below. The app wil be running on port 3000
```
cd app
#Once run install, to install all npm packages
npm install
#Followed by
npm run start
``` 

## Building production

The system has been setup to build and automatically push to hub.docker.com. But for local build you can use

```
docker build --tag 3pidev/myfleetrobot .
docker push 3pidev/myfleetrobot
```

Running and testing the build file can be done by
```
docker run -d -p 1323:1323 -e JSONUSR=admin -e JSONPWD=admin --name=fleetrobot --restart unless-stopped 3pidev/myfleetrobot:latest
```

# ToDo
* Update GUI updater

## Gui Info for booking
mid=bootid
rid=bookingid
from=
dur=Duration/15
https://my-fleet.eu/R1B34/gui/index.php?a=e&menu=Rmenu&extrainfo=mid%3D52%26co%3D0%26rid%3D443308%26from%3D195%26dur%3D6%26rec%3D0%26user%3D263&bounds=2292,2419.9333333333
a: e
menu: Rmenu
extrainfo: mid=52&co=0&rid=443308&from=195&dur=6&rec=0&user=263
bounds: 2292,2419.9333333333

Modify via gui
//Find Data of Gui start
//Second get boot ID and booking id from record
//Simulate the menu action, and store the cookies
POST https://my-fleet.eu/R1B34/gui/index.php?a=e&menu=Rmenu&page=1_modifylogbook
FORDATA
newStart: 170
newEnd: 176
clubcode: 
username: ..
password: ..
POST https://my-fleet.eu/R1B34/gui/index.php?a=e&menu=Amenu
FORMDATA
newStart: 171
newEnd: 176
comment: #:09:30 - 11:00
page: 3_commit
act: Ok


# How the system finds a slot
The system wil move the booking by finding the last option witch is bookable and then move it every 15min.

```
        SunRise                                                           SunSet
          |                                                                 |
----------------------------------------------------------------------------------------------
|         |                      |          |            | 
| BoatList.EpochStart            |  BoatList.EpochEnd    |
BoatList.EpochDate               |                       |
|                          Booking.EpochStart     Booking.EpochEnd        
Booking.EpochDate                <    Booking.Duration   >
```

1. We should check if boat is allready booked for the periode blocking booking
2. Try to book with 	endtime := MinInt64(boatList.EpochEnd, booking.EpochEnd)
	starttime := MinInt64(booking.EpochStart, MinInt64(booking.EpochStart, endtime-booking.Duration*60))
	starttime = MaxInt64(starttime, boatList.SunRise)
3. Check if duration is bigger the minimalDuration