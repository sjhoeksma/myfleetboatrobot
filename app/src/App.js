import React, { useEffect, useState } from 'react';
import MaterialTable from 'material-table';
import './App.css';
import axios from 'axios';
import { Alert, AlertTitle } from '@material-ui/lab';

var url = "http://localhost:1323/booking"
if (process.env.NODE_ENV === 'production') {
  url = "/booking"
}

const App = () => {

  const [booking, setBooking] = useState([]);
  const [iserror, setIserror] = useState(false);
  const [errorMessages, setErrorMessages] = useState([]);

  let columns = [
    { title: 'Id', field: 'id', hidden: true },
    { title: 'Boot', field: 'boat' },
    { title: 'Datum', field: 'date' ,  type : 'date'},
    { title: 'Tijd', field: 'time', sorting :false, type : 'time'  },
    { title: 'Duur', field: 'duration', type : 'numeric', sorting :false ,initialEditValue : 90 },
    { title: 'Gebruiker', field: 'user' },
    { title: 'Password', field: 'password', sorting :false  },
    { title: 'Commentaar', field: 'comment', editable : 'onAdd', sorting :false  },
    { title: 'Status', field: 'state' },
    { title: 'Melding', field: 'message', editable : 'never', sorting :false },
  ]

  useEffect(() => {
    axios.get(`${url}`)
      .then(res => {
        const booking = res.data;
        setBooking(booking);
        // console.log(booking);
      })
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
    }

    if (errorList.length < 1) {
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
    console.log("delete",oldData)
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


  return (
    <div className="app">
      <h1>Spaarne Boot Robot</h1> <br /><br />

      <MaterialTable
        title="Booking Details"
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

    </div>
  );
}

export default App;
