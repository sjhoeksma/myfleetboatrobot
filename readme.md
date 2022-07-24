#

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
2. Try to book with end=Min(BoatList.EpochEnd,Booking.EpochEnd) , 
   start=Max(BoatList.EpochStart,Max(Booking.EpochStart,Booking.EpochEnd-Booking.Duration))
3. Check if duration is bigger the minimalDuration