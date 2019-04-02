package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)
import _ "github.com/denisenkom/go-mssqldb"
import "database/sql"

func main() {
	conn, err := sql.Open("mssql", "server=localhost;port=57034;user id=sa;password=Homag;encrypt=disable")
	if err != nil {
		log.Fatal("Open connection failed:", err.Error())
	}
	err = conn.Ping()
	if err != nil {
		log.Fatal("Ping failed:", err.Error())
	}
	defer conn.Close()

	var id int64
	var intID int64
	var val int64
	var types string
	var classname string
	var plan string
	var lauf string
	for x := 0; x < 10; x = x + 1 {
		err = conn.QueryRow(`select ID,
						Lauf,
						"Plan",
						ClassName,
						Type,
						IntID,
						Val from Cadmatic4.dbo.PieceCounter  WHERE intID=?`, 119).
			Scan(&id, &lauf, &plan, &classname, &types, &intID, &val)

		if err != nil {
			log.Fatal("Query failed:", err.Error())
		}
		var matId string = strings.Split(lauf, " ")[1]
		fmt.Println(id, matId, plan, classname, types, intID, val)
		time.Sleep(100 * time.Millisecond)

	}

	/*
		stmt, err := conn.Prepare(`select ID,
							Lauf,
							"Plan",
							ClassName,
							Type,
							IntID,
							Val from Cadmatic4.dbo.PieceCounter  WHERE intID=?`)
		if err != nil {
			log.Fatal("Prepare failed:", err.Error())
		}
		defer stmt.Close()

		row := stmt.QueryRow(119)
		var id int64
		var intID int64
		var val int64
		var types string
		var classname string
		var plan string
		var lauf string

		err = row.Scan(&id, &lauf, &plan, &classname, &types, &intID, &val)
		if err != nil {
			log.Fatal("Scan failed:", err.Error())
		}*/
	//	fmt.Printf(string(id), lauf, plan, classname, types, string(intID), string(val))
	//	fmt.Printf("somechars:%s\n", somechars)
}
