# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [ToDo]
- Implement confirmation
- Add Prefered boats by number of users
- Move all JSON files to the database
- Split code in mutliple files
- Add Planner Team - Planner Dates base on flag planner
- Add PlanLogic
- Change booking using the user dropdown instead of username password

## [Development]
### Added
### Changed
### Removed

## [0.6.1]
### Added
- Support for Multiple usergroups, settings comment header, title and password
- Release notes
- Using -logFile parameter will write log information to file
- Login screen, with cookie support
- Write app version number to datafiles and on startup do upgrade if needed
- Added whatsapp configuration within team
- On startup default team is created if it not exists
- Users can only delete canceled records
- Title can be different from Team name, by setting it in TeamConfig
- Admin in team can edit all other teams
- Add failure indication or message
- Startup spinner
- Prefix is defined by team
- Added date to whatsapp message

### Changed
- When blocking, state is not changed to error
- Only bookings with state Canceled can be deleted by GUI
- User comments are now keept when changed during update
- App can now allways edit record
- Whatsapp confirmation message updated
- Change location of datafiles from json to db + central create of dataDir
- Config is now unprotect and contains authRequired
- Usercomment is now respected
- Cancel check done before blocking check 
- User is added on new_booking
- jsonUsr commandline changed tot jsonTeam
- Repeat is now based on None,Daily,Weekly,Monthly,Yearly
- Loop interval to seconds + retry stated added
- Fix the start time is calculated for one hour instead of duration

### Removed
- Single whatsapp connection
- Clear of Message every update, preserving last message
- BoatFilter is removed

## [0.5.1b] - 2022-08-23
### Changed
- Hot Fix to ensure Confirmed does not end-up in error/failed

## [0.5.1] - 2022-08-21
### Added
- Parallel processing of booking

### Changed
- Fix confirmation, not triggered

## [0.5.0] - 2022-08-20
### Added
- WhatsApp support

## [0.1.0] - 2022-06-06

