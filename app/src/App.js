import React, { useEffect, useState } from 'react';
import MaterialTable from 'material-table';
import './App.css';
import axios from 'axios';
import { Alert, AlertTitle,Autocomplete} from '@material-ui/lab'
import { TextField} from '@material-ui/core'
import packageJson from '../package.json';
import ActivityDetector from 'react-activity-detector';

var url = "http://localhost:1323/booking"
var urlBoat = "http://localhost:1323/boat"
if (process.env.NODE_ENV === 'production') {
  url = "/booking"
  urlBoat = "/boat"
}

var idleTimer = 0

const App = () => {

  const [booking, setBooking] = useState([]);
  const [boat, setBoat] = useState([]);
  const [iserror, setIserror] = useState(false);
  const [errorMessages, setErrorMessages] = useState([]);

  let columns = [
    { title: 'Id', field: 'id', hidden: true },
    { title: 'Boot', field: 'boat', editable : 'onAdd',
     //, lookup: {"lynx": "Lynx", "sneep": "Sneep"}
     render: rowData => <p>{rowData.boat}</p>,
     editComponent: props => (
     <Autocomplete
          freeSolo
          id="boats"
          options={boat}
          renderInput={params => {
            return (
              <TextField
                {...params}
                label="Search"
                variant="outlined"
                fullWidth
              />
            );
          }}
          onChange={e =>props.onChange(e.target.innerText)}
          onInputChange={e =>props.onChange(e.target.value)}
        />)
    },
    { title: 'Datum', field: 'date' ,  type : 'date'},
    { title: 'Tijd', field: 'time', sorting :false,
      // type : 'time',
      initialEditValue : "09:30",
      lookup: {
       
        "08:00" : "08:00",  "08:15" : "08:15", "08:30" : "08:30", "08:45" : "08:45",
        "09:00" : "09:00",  "09:15" : "09:15", "09:30" : "09:30", "09:45" : "09:45",
        "10:00" : "10:00",  "10:15" : "10:15", "10:30" : "10:30", "10:45" : "10:45",
        "11:00" : "11:00",  "11:15" : "11:15", "11:30" : "11:30", "11:45" : "11:45",
        "12:00" : "12:00",  "12:15" : "12:15", "12:30" : "12:30", "12:45" : "12:45",
        "13:00" : "13:00",  "13:15" : "13:15", "13:30" : "13:30", "13:45" : "13:45",
        "14:00" : "14:00",  "14:15" : "14:15", "14:30" : "14:30", "14:45" : "14:45",
        "15:00" : "15:00",  "15:15" : "15:15", "15:30" : "15:30", "15:45" : "15:45",
        "16:00" : "16:00",  "16:15" : "16:15", "16:30" : "16:30", "16:45" : "16:45",
        "17:00" : "17:00",  "17:15" : "17:15", "17:30" : "17:30", "17:45" : "17:45",
        "18:00" : "18:00",  "18:15" : "18:15", "18:30" : "18:30", "18:45" : "18:45",
        "19:00" : "19:00",  "19:15" : "19:15", "19:30" : "19:30", "19:45" : "19:45",
        "20:00" : "20:00",  "20:15" : "20:15", "20:30" : "20:30", "20:45" : "20:45",
        "21:00" : "21:00",  "21:15" : "21:15", "21:30" : "21:30", "21:45" : "21:45",
        "22:00" : "22:00",  "22:15" : "22:15", "22:30" : "22:30", "22:45" : "22:45",
        "05:00" : "05:00",  "05:15" : "05:15", "05:30" : "05:30", "05:45" : "05:45",
        "06:00" : "06:00",  "06:15" : "06:15", "06:30" : "06:30", "06:45" : "06:45",
        "07:00" : "07:00",  "07:15" : "07:15", "07:30" : "07:30", "07:45" : "07:45"
       }
     },
    { title: 'Duur', field: 'duration', type : 'numeric', sorting :false ,initialEditValue : 90,
      lookup: {60: 60, 75: 75, 90: 90,105:105,120: 120}  },
    { title: 'Gebruiker', field: 'user',  },
    { title: 'Password', field: 'password', sorting :false , 
      render: rowData => <p>{rowData.password.split('').map(() => '*')}</p>,
      editComponent: props => (
        <input
            type="password"
            value={props.value}
            onChange={e => props.onChange(e.target.value)}
        />) },
    { title: 'Commentaar', field: 'comment', editable : 'onAdd', sorting :false  },
    { title: 'UserCommentaar', field: 'usercomment', hidden: true ,type : 'boolean' },
    { title: 'Status', field: 'state' , editable : 'never'},
    { title: 'Melding', field: 'message', editable : 'never', sorting :false },
  ]

  const refreshData = () =>{
    axios.get(`${url}`)
    .then(res => {
      const booking = res.data;
      setBooking(booking);
      //console.log(booking);
    })
  }

  const refreshBoat = () =>{
    axios.get(`${urlBoat}`)
    .then(res => {
      const boat = res.data;
      setBoat(boat);
      //console.log(boat);
    })
  }
  useEffect(() => {
    refreshData()
    refreshBoat()
   }, [])
 

  //function for updating the existing row details
  const handleRowUpdate = (newData, oldData, resolve,reject) => {
    //validating the data inputs
    let errorList = []
    if (!('password' in newData) || newData.password === "") {
      errorList.push("Try Again, You didn't enter the Password field")
    }
    if (!('user' in newData) || newData.user === "") {
      errorList.push("Try Again, You didn't enter the User field")
    } else  {
     newData.user = newData.user.toUpperCase() 
    }
    if (!('boat' in newData) || newData.boat === "") {
      errorList.push("Try Again, You didn't enter the Boat field")
    }
    if (!('date' in newData) || newData.date === "") {
      errorList.push("Try Again, You didn't enter a valid Date field")
    }

    if (!('time' in newData) || newData.time === "" ) {
      errorList.push("Try Again, You didn't enter a valid  Time field")
    }
    if (!('duration' in newData) || newData.duration === "") {
      errorList.push("Try Again, You didn't enter the Duration field")
    } else {
      newData.duration = parseInt(newData.duration)
    }

    if (errorList.length < 1) {
      newData.usercomment = oldData.usercomment || oldData.commment !== newData.comment
      axios.put(`${url}/${newData.id}`, newData)
        .then(response => {
          const data = response.data;
          setBooking(data);
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
  const handleRowDelete = (oldData, resolve,reject) => {
    axios.delete(`${url}/${oldData.id}`)
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
    if (!('password' in newData) || newData.password === "") {
      errorList.push("Try Again, You didn't enter the Password field")
    }
    if (!('user' in newData) || newData.user === "") {
      errorList.push("Try Again, You didn't enter the User field")
    } else  {
     newData.user = newData.user.toUpperCase() 
    }
    if (!('boat' in newData) || newData.boat === "") {
      errorList.push("Try Again, You didn't enter the Boat field")
    }
    if (!('date' in newData) || newData.date === "") {
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

    if (errorList.length < 1) {
      axios.post(`${url}`, newData)
        .then(response => {
          const data = response.data;
          setBooking(data);
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

  const customActivityEvents = [
    'click', 'keydown', 'mousedown', 'touchstart', 'focus'
  ];

  const onIdle = () => {
    //Refresh data every minute
    idleTimer = setInterval(refreshData, 60000)
  }

  const onActive = () => {
    if (idleTimer !== 0) {
      clearInterval(idleTimer);
      refreshData()
    }
    idleTimer=0
  }

  return (
    <div className="app">
      <ActivityDetector activityEvents={customActivityEvents} enabled={true} timeout={30*1000} onIdle={onIdle} onActive={onActive}/>
      <h1>Boot Robot</h1> <br /><br />

      <MaterialTable
        title="Booking"
        columns={columns}
        data={booking}
        options={{
          headerStyle: { borderBottomColor: 'red', borderBottomWidth: '3px', fontFamily: 'verdana' },
          actionsColumnIndex: -1,
          pageSize: 10
        }}
        editable={{
          onRowUpdate: (newData, oldData) =>
            new Promise((resolve,reject) => {
              handleRowUpdate(newData, oldData, resolve,reject);
            }),
          onRowAdd: (newData) =>
            new Promise((resolve,reject) => {
              handleRowAdd(newData, resolve,reject)
            }),
          onRowDelete: (oldData) =>
            new Promise((resolve,reject) => {
              handleRowDelete(oldData, resolve,reject)
            }),
          onRowAddCancelled: (rowData) =>
            new Promise((resolve,reject) => {
              setErrorMessages([])
              setIserror(false)
              resolve()
          }),
          onRowUpdateCancelled: (rowData) =>
            new Promise((resolve,reject) => {
              setErrorMessages([])
              setIserror(false)
              resolve()
          }),
        }}
      />

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
      <small class="version">v {packageJson.version}</small>
    </div>
  );
}

export default App;