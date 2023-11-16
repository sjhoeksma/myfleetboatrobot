import React, { useState,useEffect} from 'react';
import MaterialTable, { MTableBodyRow } from 'material-table';
import './App.css';
import axios from 'axios';
import { Alert, AlertTitle, Autocomplete  } from '@material-ui/lab'
import { TextField,} from '@material-ui/core'
import ActivityDetector from 'react-activity-detector';
import WhatsAppIcon from '@material-ui/icons/WhatsApp';
import PowerIcon from '@material-ui/icons/PowerSettingsNew';
import SettingsIcon from '@material-ui/icons/Tune';
import BookingIcon from '@material-ui/icons/GridOn';
import PlanningIcon from '@material-ui/icons/EventAvailable';
import UsersIcon from '@material-ui/icons/PeopleOutline';
import CloseIcon from '@material-ui/icons/Close';
import Modal from "react-modal";
import Cookies from 'js-cookie';
import QRCode from "react-qr-code";
import IconButton from "@material-ui/core/IconButton";
import MenuIcon from '@material-ui/icons/Menu';
//import MenuContainer from './component/menu/MenuContainer';
import { StyledOffCanvas, Menu, Overlay } from 'styled-off-canvas'

var url = "http://localhost:1323/data/"
if (process.env.NODE_ENV === 'production') {
  url = "/data/"
}

var idleTimer = 0

export default function App() {
  const [booking, setBooking] = useState([]);
  const [boats, setBoats] = useState([]);
  const [users, setUsers] = useState([]);
  const [appconfig, setAppConfig] = useState({});
  const [teams, setTeams] = useState([]);
  const [whatsAppTo, setWhatsAppTo] = useState([]);
  const [iserror, setIserror] = useState(false);
  const [errorMessages, setErrorMessages] = useState([]);
  const [selectedRow, setSelectedRow] = useState(null);
  const [isWhatsAppOpen, setWhatsAppIsOpen] = useState(false);
  const [isSettingOpen, setSettingOpen] = useState(false);
  const [isPlannerOpen, setPlannerOpen] = useState(false);
  const [isUsersOpen, setUsersOpen] = useState(false);
  const [header, setHeader] = useState({});
  const [whatsAppQR, setWhatsAppQR] = useState("");
  const [failure,setFailure] = useState(false)
  const [isMenuOpen, setIsMenuOpen] = useState(false)
  
  const RepeatLookup ={0:"None", 1: "Daily", 2: "Weekly", 3: "Monthly", 4: "Yearly"}
  //On Start we load the cookies
  useEffect(() => {
    loginCookie()
    refreshAppConfig()
  },[]) // eslint-disable-line react-hooks/exhaustive-deps

  //The team columns
  let teamcolumns = [
    {title: 'Team', field: 'team',editable: 'onAdd', defaultSort: 'desc'},
    {title: 'Password', field: 'password', sorting: false,
      render: rowData => <p>{rowData.password.split('').map(() => '*')}</p>,
      editComponent: props => (
        <TextField
          type="password"
          value={props.value}
          onChange={e => props.onChange(e.target.value)}
        />)
    },
    {title: 'Title', field: 'title'},
    {title: 'Prefix', field: 'prefix'},
    {title: 'AddTime', field: 'addtime',type: "boolean"},
    {title: 'Planner', field: 'planner', type: "boolean"},
    {title: 'WhatsApp', field: 'whatsapp',type: "boolean"},
    {title: 'WhatsApp To', field: 'whatsappto', editComponent: props => (
      <Autocomplete
        freeSolo
        id="whatsappto"
        options={whatsAppTo}
        getOptionLabel={(option) => {
          return (!option || typeof option === "string" || option instanceof String) ? option : option.to
        }}
        value={props.value}
        renderInput={params => {
          return (
            <TextField
              {...params}
              fullWidth
            />
          );
        }}
        onChange={e => { if (e) props.onChange(e.target.innerText) }}
        onInputChange={e => { if (e) props.onChange(e.target.value) }}
      />),
      hidden : !(appconfig && (appconfig.whatsapp))},
    {title: 'WhatsApp Id', field: 'whatsappid',editable: 'never',
    render : rowData => <p>{rowData.whatsappid.split('.')[0]}</p>,
    hidden : !(appconfig && (appconfig.whatsapp))
    },
    {title: 'Admin', field: 'admin', hidden : !(appconfig && appconfig.admin),type: "boolean"},
    {title: 'QRCode', field: 'qrcode',editable: 'never',hidden : true},
    {title: 'Id', field: 'id', editable: 'never',hidden : true},
  ]

  //The user columns
  let userscolumns = [
    {title: 'Team', field: 'team',editable: 'onAdd', hidden : !(appconfig && appconfig.admin)},
    {title: 'Name', field: 'name'},
    {title: 'Username', field: 'user'},
    {title: 'Password', field: 'password', sorting: false,
      render: rowData => <p>{rowData.password.split('').map(() => '*')}</p>,
      editComponent: props => (
        <TextField
          type="password"
          value={props.value}
          onChange={e => props.onChange(e.target.value)}
        />)
    },
    {title: 'Id', field: 'id', editable: 'never',hidden : true},
  ]

  //The booking columns
  let columns = [
    { title: 'State', field: 'state', editable: 'never' },
    {title: 'Team', field: 'team',   initialEditValue: appconfig.team ,hidden : !(appconfig && appconfig.admin)},
    {
      title: 'Boat', field: 'boat', editable: 'onAdd',
      editComponent: props => (
        <Autocomplete
          freeSolo
          id="boats"
          options={boats}
          value={props.value}
          renderInput={params => {
            return (
              <TextField
                {...params}
                fullWidth
              />
            );
          }}
          onChange={e => { if (e) props.onChange(e.target.innerText) }}
          onInputChange={e => { if (e) props.onChange(e.target.value) }}
        />)
    },
    {
      title: 'Fallback', field: 'fallback', editable: 'onAdd',
      editComponent: props => (
        <Autocomplete
          freeSolo
          id="boats"
          options={boats}
          value={props.value}
          renderInput={params => {
            return (
              <TextField
                {...params}
                fullWidth
              />
            );
          }}
          onChange={e => { if (e) props.onChange(e.target.innerText) }}
          onInputChange={e => { if (e) props.onChange(e.target.value) }}
        />)
    },
    { title: 'Date', field: 'date', type: 'date', defaultSort: 'desc' },
    {
      title: 'Time', field: 'time', sorting: false,
      // type : 'time',
      initialEditValue: "09:30",
      lookup: {
        "08:00": "08:00", "08:15": "08:15", "08:30": "08:30", "08:45": "08:45",
        "09:00": "09:00", "09:15": "09:15", "09:30": "09:30", "09:45": "09:45",
        "10:00": "10:00", "10:15": "10:15", "10:30": "10:30", "10:45": "10:45",
        "11:00": "11:00", "11:15": "11:15", "11:30": "11:30", "11:45": "11:45",
        "12:00": "12:00", "12:15": "12:15", "12:30": "12:30", "12:45": "12:45",
        "13:00": "13:00", "13:15": "13:15", "13:30": "13:30", "13:45": "13:45",
        "14:00": "14:00", "14:15": "14:15", "14:30": "14:30", "14:45": "14:45",
        "15:00": "15:00", "15:15": "15:15", "15:30": "15:30", "15:45": "15:45",
        "16:00": "16:00", "16:15": "16:15", "16:30": "16:30", "16:45": "16:45",
        "17:00": "17:00", "17:15": "17:15", "17:30": "17:30", "17:45": "17:45",
        "18:00": "18:00", "18:15": "18:15", "18:30": "18:30", "18:45": "18:45",
        "19:00": "19:00", "19:15": "19:15", "19:30": "19:30", "19:45": "19:45",
        "20:00": "20:00", "20:15": "20:15", "20:30": "20:30", "20:45": "20:45",
        "21:00": "21:00", "21:15": "21:15", "21:30": "21:30", "21:45": "21:45",
        "22:00": "22:00", "22:15": "22:15", "22:30": "22:30", "22:45": "22:45",
        "07:00": "07:00", "07:15": "07:15", "07:30": "07:30", "07:45": "07:45",
        "06:00": "06:00", "06:15": "06:15", "06:30": "06:30", "06:45": "06:45",
        "05:00": "05:00", "05:15": "05:15", "05:30": "05:30", "05:45": "05:45"
      }
    },
    {
      title: 'Duration', field: 'duration', type: 'numeric', sorting: false, initialEditValue: 90,
      lookup: { 60: 60, 75: 75, 90: 90, 105: 105, 120: 120 }
    },
    {
      title: 'User', field: 'user',
      editComponent: props => (
        <Autocomplete
          freeSolo
          id="username"
          options={users}
          getOptionLabel={(option) => {
            return (!option || typeof option === "string" || option instanceof String) ? option : option.user
          }}
          value={props.value}
          onChange={(e, v) => {
            if (e && e.target && e.target.innerText) {
              for (var i = 0; i < users.length; i++) {
                if (users[i]["user"] === e.target.innerText.toUpperCase() &&
                  props.rowData["password"] !== users[i]["password"]) {
                  props.rowData["password"] = users[i]["password"]
                }
              }
              props.onChange(e.target.innerText)
            } else {
              props.onChange(v)
            }
          }}
          onInputChange={(e, v) => {
            if (e && e.target && e.target.value) {
              for (var i = 0; i < users.length; i++) {
                if (users[i]["user"] === e.target.value.toUpperCase() &&
                  props.rowData["password"] !== users[i]["password"]) {
                  props.rowData["password"] = users[i]["password"]
                }
              }
              props.onChange(e.target.value)
            } else {
              props.onChange(v)
            }
          }}
          renderInput={(params) => (
            <TextField {...params}
              fullWidth
            />
          )}
        />)
    },
    {
      title: 'Password', field: 'password', sorting: false,
      render: rowData => <p>{rowData.password.split('').map(() => '*')}</p>,
      editComponent: props => (
        <TextField
          type="password"
          value={props.value}
          onChange={e => props.onChange(e.target.value)}
        />)
    },
    { title: 'Comment', field: 'comment',  sorting: false },
    {
      title: 'WhatsApp', field: 'whatsapp', sorting: false, initialEditValue: appconfig.whatsappto, hidden: !(appconfig ? appconfig.whatsapp && appconfig.whatsappid : false),
      editComponent: props => (
        <Autocomplete
          freeSolo
          id="whatsappid"
          options={whatsAppTo}
          getOptionLabel={(option) => {
            return (!option || typeof option === "string" || option instanceof String) ? option : option.to
          }}
          value={props.value}
          renderInput={params => {
            return (
              <TextField
                {...params}
                fullWidth
              />
            );
          }}
          onChange={e => { if (e) props.onChange(e.target.innerText) }}
          onInputChange={e => { if (e) props.onChange(e.target.value) }}
        />)
    },
    { title: 'Repeat', field: 'repeat', sorting: false, initialEditValue: 0, type: 'numeric',  lookup: RepeatLookup },
    { title: 'Message', field: 'message', editable: 'never', sorting: false},
    { title: 'UserComment', field: 'usercomment', type: 'boolean', hidden: true },
    { title: 'Id', field: 'id', hidden: true },
  ]

  //Refresh the booking data
  const refreshBooking = () => {
    axios.get(`${url}booking`, header).then(res => {
        const booking = res.data;
        setFailure(false)
        setBooking(booking);
      }).catch(reason => {setBooking([]);setFailure(true)})
  }

  //Refresh the boat data
  const refreshBoat = () => {
    axios.get(`${url}boat`, header)
      .then(res => {
        const boat = res.data;
        setBoats(boat);
      }).catch(reason => {setBoats([])})
  }

  //Refresh the appconfig
  const refreshAppConfig = () => {
    axios.get(`${url}config`, header)
      .then(res => {
        const data = res.data;
        setAppConfig(data);
        document.title = (data && data.title ? data.title : "MyFleet Robot")
      }).catch(reason => {setAppConfig({})})
  }

  //Refresh the user list
  const refreshUsers = () => {
    axios.get(`${url}users`, header)
      .then(res => {
        const users = res.data;
        setUsers(users);
      }).catch(reason => {setUsers([])})
  }

  //Refesh the whatsAppTopGroups
  const refreshWhatsAppTo = () => {
    axios.get(`${url}whatsappto`, header)
      .then(res => {
        const whatsapp = res.data;
        setWhatsAppTo(whatsapp);
      }).catch(reason => {setWhatsAppTo([])})
  }

  //Refresh the appconfig
  const refreshTeams = () => {
    axios.get(`${url}teams`, header)
      .then(res => {
        const data = res.data;
        setTeams(data);
      }).catch(reason => {setTeams([])})
  }

  //Refresh all data
  const refreshAll = () =>{
    refreshAppConfig()
    refreshBooking()
    refreshBoat()
    refreshUsers()
    refreshWhatsAppTo()
  }

  //Load the login Cookies
  const loginCookie =() =>{
    var auth = Cookies.get('auth')
    if (auth) {
      var h = header
      h.auth = JSON.parse(atob(auth))
      //Update the cookie to keep 7 days before login out
      Cookies.set('auth', auth, { expires: 7 })
      setHeader(h)
      refreshAll()
    }
  }
  
  //Login
  const logout = () => {
    var h = header
    delete h.auth
    Cookies.remove('auth')
    //To logout some browser need invalid login
   // axios.get(`${url}teams/`,{auth:{username: "invalid",password:"invalid"}})
    setHeader(h)
    setIserror(false)
    setErrorMessages([])
    setBooking([])
    setBoats([])
    setUsers([])
    setAppConfig({})
    setWhatsAppTo([])
    setSettingOpen(false)
    refreshAppConfig()
  }

  //Get a QRCode
  const getQRCode = async () =>{
    axios.get(`${url}whatsapp`, Object.assign(
      {onDownloadProgress: function (progressEvent) {
        const lines = progressEvent.target.responseText.split(/\r?\n/)
        const data = JSON.parse(lines[lines.length - 2])
        setWhatsAppQR(data.qrcode)
      },
    },header))
    .then(res => {
      setWhatsAppQR("")
      refreshAppConfig()
    }).catch(reason => {setWhatsAppQR("")})
  }

  //Disconnect from whatsapp session
  const disconnectWhatsApp = () =>{
    axios.delete(`${url}whatsapp`,header).then(res => {
      refreshAppConfig()
    }).catch(reason => {console.log(reason)})
  }

  //Toggle the whatsApp modal
  const toggleWhatsAppOpen = () => {
    if (!isWhatsAppOpen){ 
      //We are opening, getQRCode incase there is no whatsAppId
      if (!appconfig.whatsappid){
        getQRCode()
      }
    }
    setWhatsAppIsOpen(!isWhatsAppOpen);
  }

  //Toggle the Settings
  const toggleSettingsOpen = () => {
    if (isSettingOpen){ 
      refreshAppConfig() //Apply the changes made to screen
    } else {
      refreshTeams()
    }
    setSettingOpen(!isSettingOpen);
  }

   //Toggle the Planner
   const togglePlannerOpen = () => {
    if (isPlannerOpen){ 
      //refreshAppConfig() //Apply the changes made to screen
    }
    setPlannerOpen(!isPlannerOpen);
  }

    //Toggle the Planner
    const toggleUsersOpen = () => {
    if (!isUsersOpen){ 
      refreshUsers()
    }
    setUsersOpen(!isUsersOpen);
  }

  const openBooking = () => {
    setUsersOpen(false)
    setPlannerOpen(false)
    setSettingOpen(false)
    setWhatsAppIsOpen(false)
  }

  //Function called when we do login
  const handleLoginSubmit = (event) => {
    //Prevent page reload
    event.preventDefault();
    var { team, pass } = document.forms[0];
    axios.post(`${url}login`,{team: team.value, password:pass.value},header)
    .then(res => {
      const data = res.data;
      if (data.status === "ok") {
          var d = {username: team.value, password: pass.value}
          var h = header
          h.auth = d
          Cookies.set('auth', btoa(JSON.stringify(d)), { expires: 7 })
          setHeader(h)
          refreshAll()
      } else {
        setErrorMessages(["Invalid Team or Password"]);
      }
    }).catch(reason =>{
      setErrorMessages([String(reason)]);
    })
  };

   //function for updating the existing row details
   const handleTeamUpdate = (newData, oldData, resolve, reject) => {
    //validating the data inputs
    let errorList = []
    if (!('password' in newData) || newData.password === "" || newData.password === null) {
      errorList.push("Try Again, You didn't enter the Password field")
    }
    if (!('team' in newData) || newData.team === "" || newData.team== null) {
      errorList.push("Try Again, You didn't enter the Team field")
    } 
    if (!('title' in newData) || newData.title === "" || newData.title === null) {
      errorList.push("Try Again, You didn't enter the title field")
    }

    if (errorList.length < 1) {
      axios.put(`${url}teams/${newData.id}`, newData,header)
        .then(response => {
          const data = response.data;
          setTeams(data);
          setIserror(false)
          setErrorMessages([])
          refreshAppConfig()
          resolve()
        })
        .catch(error => {
          setErrorMessages(["Update failed! Server error"])
          setIserror(true)
          reject()
        })
    } else {
      setErrorMessages(errorList)
      setIserror(true)
      reject()
    }
  }

  //function for deleting a row
  const handleTeamDelete = (oldData, resolve, reject) => {
    axios.delete(`${url}teams/${oldData.id}`,header)
      .then(response => {
        const data = response.data;
        setTeams(data);
        setIserror(false)
        setErrorMessages([])
        resolve()
      })
      .catch(error => {
        setErrorMessages(["Delete failed! Server error"])
        setIserror(true)
        reject()
      })
  }

  //function for adding a new row to the table
  const handleTeamAdd = (newData, resolve, reject) => {
    //validating the data inputs
    let errorList = []
    if (!('password' in newData) || newData.password === "" || newData.password === null) {
      errorList.push("Try Again, You didn't enter the Password field")
    }
    if (!('team' in newData) || newData.team === "" || newData.team== null) {
      errorList.push("Try Again, You didn't enter the Temm field")
    } 
    if (!('prefix' in newData) || newData.prefix === "" || newData.prefix === null) {
      errorList.push("Try Again, You didn't enter the prefix field")
    }
    if (!('title' in newData) || newData.title === "" || newData.title === null) {
      errorList.push("Try Again, You didn't enter the title field")
    }

    if (errorList.length < 1) {
      axios.post(`${url}teams`, newData,header)
        .then(response => {
          const data = response.data;
          setTeams(data);
          setErrorMessages([])
          setIserror(false)
          resolve()
        })
        .catch(error => {
          setErrorMessages(["Cannot add data. Server error!"])
          setIserror(true)
          reject()
        })
    } else {
      setErrorMessages(errorList)
      setIserror(true)
      reject()
    }
  }

    //function for updating the existing row details
    const handleUserUpdate = (newData, oldData, resolve, reject) => {
    //validating the data inputs
    let errorList = []
    if (!('password' in newData) || newData.password === "" || newData.password === null) {
      errorList.push("Try Again, You didn't enter the Password field")
    }
    if (!('user' in newData) || newData.user === "" || newData.user === null) {
      errorList.push("Try Again, You didn't enter the Username field")
    }
    if (!('name' in newData) || newData.name === "" || newData.name === null) {
      errorList.push("Try Again, You didn't enter the Name field")
    }

    if (errorList.length < 1) {
      axios.put(`${url}users/${newData.id}`, newData,header)
        .then(response => {
          const data = response.data;
          setUsers(data);
          setIserror(false)
          setErrorMessages([])
          resolve()
        })
        .catch(error => {
          setErrorMessages(["Update failed! Server error"])
          setIserror(true)
          reject()
        })
    } else {
      setErrorMessages(errorList)
      setIserror(true)
      reject()
    }
  }

  //function for deleting a row
  const handleUserDelete = (oldData, resolve, reject) => {
    axios.delete(`${url}users/${oldData.id}`,header)
      .then(response => {
        const data = response.data;
        setUsers(data);
        setIserror(false)
        setErrorMessages([])
        resolve()
      })
      .catch(error => {
        setErrorMessages(["Delete failed! Server error"])
        setIserror(true)
        reject()
      })
  }
  
  //function for adding a new row to the table
  const handleUserAdd = (newData, resolve, reject) => {
    //validating the data inputs
    let errorList = []
    if (!('password' in newData) || newData.password === "" || newData.password === null) {
      errorList.push("Try Again, You didn't enter the Password field")
    }

    if (!('username' in newData) || newData.username === "" || newData.username === null) {
      errorList.push("Try Again, You didn't enter the prefix field")
    }
    if (!('name' in newData) || newData.name === "" || newData.name === null) {
      errorList.push("Try Again, You didn't enter the Name field")
    }

    if (errorList.length < 1) {
      axios.post(`${url}users`, newData,header)
        .then(response => {
          const data = response.data;
          setUsers(data);
          setErrorMessages([])
          setIserror(false)
          resolve()
        })
        .catch(error => {
          setErrorMessages(["Cannot add data. Server error!"])
          setIserror(true)
          reject()
        })
    } else {
      setErrorMessages(errorList)
      setIserror(true)
      reject()
    }
  }
  
  //function for updating the existing row details
  const handleRowUpdate = (newData, oldData, resolve, reject) => {
    //validating the data inputs
    let errorList = []
    if (!('password' in newData) || newData.password === "" || newData.password === null) {
      errorList.push("Try Again, You didn't enter the Password field")
    }
    if (!('user' in newData) || newData.user === "" || newData.user == null) {
      errorList.push("Try Again, You didn't enter the User field")
    } else {
      newData.user = newData.user.toUpperCase()
    }
    if (!('boat' in newData) || newData.boat === "" || newData.boat === null) {
      errorList.push("Try Again, You didn't enter the Boat field")
    }
    if (!('date' in newData) || newData.date === "" || newData.date === null) {
      errorList.push("Try Again, You didn't enter a valid Date field")
    }

    if (!('time' in newData) || newData.time === "") {
      errorList.push("Try Again, You didn't enter a valid  Time field")
    }
    if (!('duration' in newData) || newData.duration === "") {
      errorList.push("Try Again, You didn't enter the Duration field")
    } else {
      newData.duration = parseInt(newData.duration)
    }
    newData.repeat = parseInt(newData.repeat)

    if (errorList.length < 1) {
      axios.put(`${url}booking/${newData.id}`, newData,header)
        .then(response => {
          const data = response.data;
          setBooking(data);
          setIserror(false)
          setErrorMessages([])
          resolve()
          refreshUsers()
          refreshWhatsAppTo()
        })
        .catch(error => {
          setErrorMessages(["Update failed! Server error"])
          setIserror(true)
          reject()
        })
    } else {
      setErrorMessages(errorList)
      setIserror(true)
      reject()
    }
  }

  //function for deleting a row
  const handleRowDelete = (oldData, resolve, reject) => {
    axios.delete(`${url}booking/${oldData.id}`,header)
      .then(response => {
        const data = response.data;
        setBooking(data);
        setIserror(false)
        setErrorMessages([])
        resolve()
      })
      .catch(error => {
        setErrorMessages(["Delete failed! Server error"])
        setIserror(true)
        reject()
      })
  }

  //function for adding a new row to the table
  const handleRowAdd = (newData, resolve, reject) => {
    //validating the data inputs
    let errorList = []
    if (!('password' in newData) || newData.password === "" || newData.password === null) {
      errorList.push("Try Again, You didn't enter the Password field")
    }
    if (!('user' in newData) || newData.user === "" || newData.user === null) {
      errorList.push("Try Again, You didn't enter the User field")
    } else {
      newData.user = newData.user.toUpperCase()
    }
    if (!('boat' in newData) || newData.boat === "" || newData.boat === null) {
      errorList.push("Try Again, You didn't enter the Boat field")
    }
    if (!('date' in newData) || newData.date === "" || newData.date === null) {
      errorList.push("Try Again, You didn't enter the Date field")
    }
    if (!('time' in newData) || newData.time === "") {
      errorList.push("Try Again, You didn't enter the Time field")
    }
    if (!('duration' in newData) || newData.duration === "") {
      errorList.push("Try Again, You didn't enter the Duration field")
    } else {
      newData.duration = parseInt(newData.duration)
    }
    newData.repeat = parseInt(newData.repeat)

    if (errorList.length < 1) {
      axios.post(`${url}booking`, newData,header)
        .then(response => {
          const data = response.data;
          setBooking(data);
          setErrorMessages([])
          setIserror(false)
          resolve()
          refreshUsers()
          refreshWhatsAppTo()
        })
        .catch(error => {
          setErrorMessages(["Cannot add data. Server error!"])
          setIserror(true)
          reject()
        })
    } else {
      setErrorMessages(errorList)
      setIserror(true)
      reject()
    }
  }

  const customActivityEvents = [
    'click', 'keydown', 'mousedown', 'touchstart', 'focus'
  ];

  const onIdle = () => {
    //Refresh data every minute
    if ((appconfig && !appconfig.authRequired) || header.auth) {
       idleTimer = setInterval(refreshBooking, 60000)
    }
  }

  const onActive = () => {
    if (idleTimer !== 0) {
      clearInterval(idleTimer);
      if ((appconfig && !appconfig.authRequired) || header.auth) {
        refreshBooking()
        refreshUsers()
        refreshBoat()
      }
    }
    idleTimer = 0
  }

  const renderFailure = (
     failure ? <div className="Failure">Data Connection Failure</div> : <div style={{display:"none"}}></div>
  )

  // JSX code for login form
  const renderLogin = (
    <div className="Auth-form-container">
      <div className="Auth-form">
      <img src="icon.png" alt="" style={{ display: 'block','marginLeft': 'auto', 'marginRight': 'auto', width:45}}></img>
        <h3 className="Auth-form-title">Sign In</h3>
        <div className="form">
          <form onSubmit={handleLoginSubmit}>
            <div className="Auth-input-container">
              <label>Team </label>
              <input type="text" name="team" required />
            </div>
            <div className="Auth-input-container">
              <label>Password </label>
              <input type="password" name="pass" required />
              {
                errorMessages.map((msg, i) => {
                  return <div key={i} className="Auth-error">{msg}</div>
                })
              }
            </div>
            <div className="Auth-button-container">
              <input type="submit" value="Login" />
            </div>
          </form>
        </div>
      </div>
    </div>
  )

  const renderErrorList = (
    <div>
        {iserror &&
          <Alert severity="error">
            <AlertTitle>ERROR</AlertTitle>
            {errorMessages.map((msg, i) => {
              return <div key={i}>{msg}</div>
            })}
          </Alert>
        }
      </div>
  )

  const renderPlanner = (
    <div>Not yet implemented</div>
  )

  const renderUsers = (
    <div>
      <MaterialTable
        title={<div className="MTableToolbar-title-9"><img className="TableToolbar-icon" src="favicon.ico" alt=""></img><h6 className="MuiTypography-root MuiTypography-h6" style={{ "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" }}>&nbsp;{appconfig && appconfig.title ? appconfig.title : "MyFleet Robot"} - Users</h6></div>}
        columns={userscolumns}
        data={users}
        options={{
          headerStyle: { borderBottomColor: 'red', borderBottomWidth: '3px', fontFamily: 'verdana' },
          actionsColumnIndex: -1,
          paging: false,
          showTitle: true,
          draggable: false,
          addRowPosition: "first",
          rowStyle: rowData => ({
            backgroundColor: (selectedRow === rowData.tableData.id) ? '#EEE' : '#FFF'
          }),
        }}
        components={{
          Row: props => (
            <MTableBodyRow
              {...props}
              onDoubleClick={(e) => {
                props.actions[1]().onClick(e, props.data);
              }}
            />
          )
        }}
        onRowClick={((evt, selectedRow) => setSelectedRow(selectedRow.tableData.id))}
        editable={{
        onRowUpdate: (newData, oldData) =>
          new Promise((resolve, reject) => {
            handleUserUpdate(newData, oldData, resolve, reject);
         }),
        onRowAddCancelled: (rowData) =>
          new Promise((resolve, reject) => {
            setErrorMessages([])
            setIserror(false)
            resolve()
         }),
        onRowUpdateCancelled: (rowData) =>
          new Promise((resolve, reject) => {
            setErrorMessages([])
            setIserror(false)
            resolve()
          }),
         onRowAdd: (newData) =>
          new Promise((resolve, reject) => {
            handleUserAdd(newData, resolve, reject)
          }),
         onRowDelete: (oldData) => 
           new Promise((resolve, reject) => {
            handleUserDelete(oldData, resolve, reject)
          }),
        }}
      />
      {renderErrorList}
    </div>
  )

  const renderSettings = (
    <div>
      <MaterialTable
        title={<div className="MTableToolbar-title-9"><img className="TableToolbar-icon" src="favicon.ico" alt=""></img>
        <h6 className="MuiTypography-root MuiTypography-h6" style={{ "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" }}>&nbsp;{appconfig && appconfig.title ? appconfig.title : "MyFleet Robot"} - Settings</h6></div>}
        columns={teamcolumns}
        data={teams}
        options={{
          headerStyle: { borderBottomColor: 'red', borderBottomWidth: '3px', fontFamily: 'verdana' },
          actionsColumnIndex: -1,
          paging: false,
          showTitle: true,
          draggable: false,
          addRowPosition: "first",
          rowStyle: rowData => ({
            backgroundColor: (selectedRow === rowData.tableData.id) ? '#EEE' : '#FFF'
          }),
        }}
        components={{
          Row: props => (
            <MTableBodyRow
              {...props}
              onDoubleClick={(e) => {
                props.actions[1]().onClick(e, props.data);
              }}
            />
          )
        }}
        onRowClick={((evt, selectedRow) => setSelectedRow(selectedRow.tableData.id))}
        editable={Object.assign({
          isDeletable: rowData => {
            return rowData.team !== appconfig.team
          },
          onRowUpdate: (newData, oldData) =>
          new Promise((resolve, reject) => {
            handleTeamUpdate(newData, oldData, resolve, reject);
          }),
        onRowAddCancelled: (rowData) =>
          new Promise((resolve, reject) => {
            setErrorMessages([])
            setIserror(false)
            resolve()
          }),
        onRowUpdateCancelled: (rowData) =>
          new Promise((resolve, reject) => {
            setErrorMessages([])
            setIserror(false)
            resolve()
          }),
        },!(appconfig && appconfig.admin) ? {
          
         } : { 
         onRowAdd: (newData) =>
          new Promise((resolve, reject) => {
            handleTeamAdd(newData, resolve, reject)
          }),
         onRowDelete: (oldData) => 
           new Promise((resolve, reject) => {
            handleTeamDelete(oldData, resolve, reject)
          }),
        })}
      />
      {renderErrorList}
    </div>
  )
  
  const renderBoat = (
    <div> 
      <Modal
        isOpen={isWhatsAppOpen}
        onRequestClose={toggleWhatsAppOpen}
        contentLabel="WhatsApp AppConfig"
        className="WhatsApp"
        overlayClassName="WhatsApp-overlay"
        closeTimeoutMS={500}
        appElement={document.getElementById('app')}
      >
        <div style={{textAlign: "center", marginBottom : 10}}><h3>WhatsApp AppConfig</h3></div>
        { appconfig.whatsappid ? 
        <div style={{textAlign: "center"}}>
        <div >
          <span>{appconfig.whatsappid.split('.')[0]}</span>
        </div><div>
          <button onClick={disconnectWhatsApp}>Disconnect</button>
        </div>
        </div>
        : whatsAppQR ? <div><div style={{ height: "auto", margin: "0 auto", maxWidth: 256, width: "100%" }}>
            <QRCode
            size={1024}
            level={'Q'}
            style={{ height: "auto", maxWidth: "100%", width: "100%" }}
            value={whatsAppQR}
            viewBox={`0 0 1024 1024`}
            />
        </div> 
        <div><span>Scan QR code in WhatsApp to enable</span></div>
        </div>
        :
        <div style={{textAlign: "center"}}> <button onClick={getQRCode}>New QRCode</button></div>
          }
        
        <div style={{textAlign: "center"}}>
          <button onClick={toggleWhatsAppOpen}>Close</button>
        </div>
      </Modal>
      <ActivityDetector activityEvents={customActivityEvents} enabled={true} timeout={30 * 1000} onIdle={onIdle} onActive={onActive} />    
      <MaterialTable
        title={<div className="MTableToolbar-title-9"><img className="TableToolbar-icon" src="favicon.ico" alt=""></img><h6 className="MuiTypography-root MuiTypography-h6" style={{ "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" }}>&nbsp;{appconfig && appconfig.title ? appconfig.title : "MyFleet Robot"}</h6></div>}
        columns={columns}
        data={booking}
         // other props
         detailPanel={rowData => {
          var out =""
          if (rowData.logs) {
            rowData.logs.forEach((l)=>{out += "<div>"+(new Date(l.date * 1000)).toISOString()+" ["+ l.state+ "] " + l.log+"</div>" })
          }
          return (
            <div
            style={{
              fontSize: 14,
              textAlign: 'left',
              color: 'white',
              backgroundColor: '#505050',
            }}
          >
            {<div dangerouslySetInnerHTML={{ __html: out }} />}
            <div>{new Date(rowData.next*1000).toISOString()} Next update</div>
          </div>
          )
        }}
        options={{
          headerStyle: { borderBottomColor: 'red', borderBottomWidth: '3px', fontFamily: 'verdana' },
          actionsColumnIndex: -1,
          paging: false,
          showTitle: true,
          draggable: false,
          addRowPosition: "first",
          rowStyle: rowData => ({
            backgroundColor: (selectedRow === rowData.tableData.id) ? '#EEE' : '#FFF'
          }),
        }}
        components={{
          Row: props => (
            <MTableBodyRow
              {...props}
              onDoubleClick={(e) => {
                props.actions[1]().onClick(e, props.data);
              }}
            />
          )
        }}
        onRowClick={((evt, selectedRow) => setSelectedRow(selectedRow.tableData.id))}
        editable={{
          // isEditable: true,
          onRowUpdate: (newData, oldData) =>
            new Promise((resolve, reject) => {
              handleRowUpdate(newData, oldData, resolve, reject);
            }),
          onRowAdd: (newData) =>
            new Promise((resolve, reject) => {
              handleRowAdd(newData, resolve, reject)
            }),
          onRowDelete: (oldData) =>
            new Promise((resolve, reject) => {
              handleRowDelete(oldData, resolve, reject)
            }),
          onRowAddCancelled: (rowData) =>
            new Promise((resolve, reject) => {
              setErrorMessages([])
              setIserror(false)
              resolve()
            }),
          onRowUpdateCancelled: (rowData) =>
            new Promise((resolve, reject) => {
              setErrorMessages([])
              setIserror(false)
              resolve()
            }),
        }}
      />
     {renderErrorList}
    </div>
  )

 
  return (
    (!appconfig.version) ? 
     <div id="app" className="App" >
       <ActivityDetector activityEvents={customActivityEvents} enabled={true} timeout={30 * 1000} onIdle={refreshAppConfig} onActive={refreshAppConfig} />
      <div className="loading"><div className="lds-roller"><div></div><div></div><div></div><div></div><div></div><div></div><div></div><div></div></div></div>
     </div> :
    (appconfig && appconfig.authRequired) && !('auth' in header) ? renderLogin :
      <div id="app" className="App" >
      {renderFailure}
      <StyledOffCanvas position = 'left' isOpen={isMenuOpen} onClose={() => setIsMenuOpen(false)} >
      <IconButton className="MenuContainer" onClick={() => setIsMenuOpen(!isMenuOpen)}><MenuIcon /></IconButton>
  
      <Menu className="Menu" >
        <ul >
          <li className="MenuElementTitle" onClick={() => setIsMenuOpen(false)}>
          <CloseIcon /><span>&nbsp;</span>
          </li>
          { isUsersOpen || isPlannerOpen || isSettingOpen || isWhatsAppOpen  ?
          <li onClick={() => {setIsMenuOpen(false);openBooking()}}>
          <BookingIcon /><span>Bookings</span>
          </li>  : ""}
          {!isUsersOpen ? 
          <li onClick={() => {setIsMenuOpen(false);openBooking();toggleUsersOpen()}}>
          <UsersIcon /><span>Users</span>
          </li> : ""}
          {appconfig.planner && !isPlannerOpen ?
          <li onClick={() => {setIsMenuOpen(false);openBooking();togglePlannerOpen()}}>
          <PlanningIcon /><span>Planner</span>
          </li> : ""}
          { appconfig.whatsapp && !isWhatsAppOpen ?
          <li onClick={() => {setIsMenuOpen(false);openBooking();toggleWhatsAppOpen()}}>
          <WhatsAppIcon /><span>WhatsApp</span>
          </li> :""}
          { !isSettingOpen ? 
          <li onClick={() => {setIsMenuOpen(false);openBooking();toggleSettingsOpen()}}>
          <SettingsIcon /><span>Settings</span>
          </li> :""}
          { appconfig.authRequired ?
          <li className="MenuElementBottom" onClick={() => {setIsMenuOpen(false);logout()}}>
          <PowerIcon /><span>Logout</span>
          </li> :""}
        </ul>
      </Menu>
      <Overlay className={isMenuOpen ? "Overlay" : ""}/>
        {isSettingOpen ?  renderSettings : isPlannerOpen ? renderPlanner : isUsersOpen ? renderUsers : renderBoat}
        <small className="myfleet">{appconfig && appconfig.myfleetVersion ? appconfig.myfleetVersion + " - " + appconfig.clubid : ""}</small>
        <small className="version">v {appconfig && appconfig.version ? appconfig.version : ""}</small>
      </StyledOffCanvas> 
      </div>  
  );
}