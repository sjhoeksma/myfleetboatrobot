#

Buidling: **docker build --tag 3pidev/spaarne .**
docker run -d -p 1323:1323 -e JSONUSR=admin -e JSONPWD=admin --name spaarne --restart unless-stopped 3pidev/spaarne:latest
docker run -d -p 1323:1323 -e JSONUSR=admin -e JSONPWD=admin --name spaarne  3pidev/spaarne:latest


## How to find slot

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