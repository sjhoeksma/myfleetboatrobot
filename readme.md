# Boot Robot
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

The system has been setup that onchecking of the oce a new version is build and automaticly push to hub.docker.com. But for local build you can use

```
docker build --tag 3pidev/spaarne .
docker push 3pidev/spaarne
```

Running and testing the build file can be done by
```
docker run -d -p 1323:1323 -e JSONUSR=admin -e JSONPWD=admin --name spaarne --restart unless-stopped 3pidev/spaarne:latest
```


# ToDo
* Bootlist instead of input
* Username and password from DB


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