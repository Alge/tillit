package db

import (
  "github.com/Alge/tillit/models"
)


func GetUser(id string) (u *models.User, err error){
  db, err := GetDB()
  if err != nil{
    return
  }

  stmt, err := db.Prepare("SELECT (id, username) FROM users WHERE id = ?")
  if err != nil{
    return
  }
  defer stmt.Close()

  row := stmt.Exec(id)

  err := row.Scan(&user.ID, &user.Username)

  return
}

func CreateUserTable() error{

}
