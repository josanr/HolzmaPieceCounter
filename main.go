package main

import (
	"database/sql"
	"log"

	_ "github.com/denisenkom/go-mssqldb"
)

func main() {
	conn, err := sql.Open("mssql", "server=192.168.221.254\\HO2008;user id=sa;password=Homag;encrypt=disable")
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
	var run string

	stmt, err := conn.Prepare(`select ID,
						Lauf,
						"Plan",
						ClassName,
						Type,
						IntID,
						Val from Cadmatic4.dbo.PieceCounter  WHERE Lauf=?`)
	if err != nil {
		log.Fatal("Prepare failed:", err.Error())
	}
	defer stmt.Close()

	row := stmt.QueryRow("176862-10")

	err = row.Scan(&id, &run, &plan, &classname, &types, &intID, &val)
	if err != nil {
		log.Fatal("Scan failed:", err.Error())
	}
	//	fmt.Printf(string(id), lauf, plan, classname, types, string(intID), string(val))
	//	fmt.Printf("somechars:%s\n", somechars)
}
