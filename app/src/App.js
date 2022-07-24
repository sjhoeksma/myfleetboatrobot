import React, { useEffect, useState } from 'react';
import MaterialTable from 'material-table';
import './App.css';
import axios from 'axios';
import { Alert, AlertTitle } from '@material-ui/lab';

//const url = "https://spaarne.os1.nl/booking"
const url = "http://localhost:1323/booking"

const App = () => {

  const [booking, setBooking] = useState([]);
  const [iserror, setIserror] = useState(false);
  const [errorMessages, setErrorMessages] = useState([]);

  let columns = [
    { title: 'BOOT', field: 'boat' , editable : 'onAdd'},
    { title: 'DATUM', field: 'date' },
    { title: 'TIJD', field: 'time', sorting :false  },
    { title: 'MIN', field: 'duration', type : 'numeric', sorting :false  },
    { title: 'GEBRUIKER', field: 'user' },
    { title: 'PASSWORD', field: 'password', sorting :false  },
    { title: 'COMMENTAAR', field: 'comment', editable : 'onAdd', sorting :false  },
    { title: 'STATUS', field: 'state' },
    { title: 'MSG', field: 'message', editable : 'never', sorting :false },
  ]

  useEffect(() => {
    axios.get(`${url}`)
      .then(res => {
        const booking = res.data;
        setBooking(booking);
         console.log(booking);
      })
  }, [])



  //function for updating the existing row details
  const handleRowUpdate = (newData, oldData, resolve) => {
    //validating the data inputs
    let errorList = []
    /*
    if (newData.name === "") {
      errorList.push("Try Again, You didn't enter the name field")
    }
    if (newData.username === "") {
      errorList.push("Try Again, You didn't enter the Username field")
    }
    if (newData.email === "" || validateEmail(newData.email) === false) {
      errorList.push("Oops!!! Please enter a valid email")
    }
    if (newData.phone === "") {
      errorList.push("Try Again, Phone number field can't be blank")
    }
    if (newData.website === "") {
      errorList.push("Try Again, Enter website url before submitting")
    }
    */

    if (errorList.length < 1) {
      axios.put(`${url}/${newData.id}`, newData)
        .then(response => {
          const data= [...booking];
          const index = oldData.tableData.id;
          data[index] = newData;
          setBooking([...data]);
          resolve()
          setIserror(false)
          setErrorMessages([])
        })
        .catch(error => {
          setErrorMessages(["Update failed! Server error"])
          setIserror(true)
          resolve()

        })
    } else {
      setErrorMessages(errorList)
      setIserror(true)
      resolve()

    }
  }


  //function for deleting a row
  const handleRowDelete = (oldData, resolve) => {
    axios.delete(`${url}/${oldData.id}`)
      .then(response => {
        const data = [...booking];
        const index = oldData.tableData.id;
        data.splice(index, 1);
        setBooking([...data]);
        resolve()
      })
      .catch(error => {
        setErrorMessages(["Delete failed! Server error"])
        setIserror(true)
        resolve()
      })
  }


  //function for adding a new row to the table
  const handleRowAdd = (newData, resolve) => {
    //validating the data inputs
    let errorList = []
    /*
    if (newData.name === "") {
      errorList.push("Try Again, You didn't enter the name field")
    }
    if (newData.username === "") {
      errorList.push("Try Again, You didn't enter the Username field")
    }
    if (newData.email === "" || validateEmail(newData.email) === false) {
      errorList.push("Oops!!! Please enter a valid email")
    }
    if (newData.phone === "") {
      errorList.push("Try Again, Phone number field can't be blank")
    }
    if (newData.website === "") {
      errorList.push("Try Again, Enter website url before submitting")
    }
    */

    if (errorList.length < 1) {
      axios.post(`${url}`, newData)
        .then(response => {
          let data = [...booking];
          data.push(newData);
          setBooking(data);
          resolve()
          setErrorMessages([])
          setIserror(false)
        })
        .catch(error => {
          setErrorMessages(["Cannot add data. Server error!"])
          setIserror(true)
          resolve()
        })
    } else {
      setErrorMessages(errorList)
      setIserror(true)
      resolve()
    }
  }


  return (
    <div className="app">
      <h1>Spaarne Auto Booking</h1> <br /><br />

      <MaterialTable
        title="Booking Details"
        columns={columns}
        data={booking}
        options={{
          headerStyle: { borderBottomColor: 'red', borderBottomWidth: '3px', fontFamily: 'verdana' },
          actionsColumnIndex: -1,
          pageSize: 14
        }}
        editable={{
          onRowUpdate: (newData, oldData) =>
            new Promise((resolve) => {
              handleRowUpdate(newData, oldData, resolve);

            }),
          onRowAdd: (newData) =>
            new Promise((resolve) => {
              handleRowAdd(newData, resolve)
            }),
          onRowDelete: (oldData) =>
            new Promise((resolve) => {
              handleRowDelete(oldData, resolve)
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
