package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/namsral/flag"
)

var connectionStrings = map[string]string{
	"Homag": "server=192.168.221.254\\Holzma;user id=sa;password=holzma;encrypt=disable",
	"H2008": "server=192.168.221.254\\HO2008;user id=sa;password=Homag;encrypt=disable"}

/*
	types:
	 	Eingestapelt = взять лист
		Produziert = произведено
		className - даёт тип произведённой детали
				Rest - обрезок
				Teil - деталь => intID это partid - 1, val => это количество произведённых деталей
				Platte - выплнение листа окончено

		)

*/
type Response struct {
	OrderId      int    `json:"orderId"`
	ToolId       int    `json:"toolId"`
	IsOffcut     bool   `json:"isOffcut"`
	PartId       int64  `json:"partId"`
	Error        bool   `json:"isError"`
	ErrorMessage string `json:"message"`
}

func (r *Response) setId(runName string) {
	ids := strings.Split(runName, "-")
	var err error

	r.OrderId, err = strconv.Atoi(ids[0])
	if err != nil {
		r.Error = true
		r.ErrorMessage = err.Error()
	}
	r.ToolId, err = strconv.Atoi(ids[1])
	if err != nil {
		r.Error = true
		r.ErrorMessage = err.Error()
	}
}
func (r *Response) setPartType(actionResult string) {
	switch actionResult {
	case "Rest":
		r.IsOffcut = true
	case "Teil":
		r.IsOffcut = false
	default:
		r.Error = true
		r.ErrorMessage = "action result is not tail or rest"
	}
}

type result struct {
	id           int64  //ID
	partid       int64  //IntID
	amount       int64  //Val
	actionType   string //Type
	actionResult string //ClassName
	mapId        string //Plan
	runName      string //Lauf
}

func (r result) getIndex() string {

	return strconv.FormatInt(r.id, 10) + ":" + r.runName + ":" + r.actionResult + ":" + strconv.FormatInt(r.partid, 10)
}

// var mapId, _ = strconv.Atoi(plan)

// fmt.Println("Serial: " + strconv.FormatInt(id, 10))
// fmt.Println(" RunName(usedAsId): " + run)
// fmt.Println(" MapNum in run: " + strconv.Itoa(mapId))
// fmt.Println(" ProdType: " + classname)
// fmt.Println(" ActType " + types)
// fmt.Println(" PartId: " + strconv.FormatInt(intID, 10))
// fmt.Println(" PartAmount: " + strconv.FormatInt(val, 10))
type runerList map[string]result

func main() {
	connSelect := "H2008"
	runId := "176862-10"

	flag.StringVar(&connSelect, "connection", connSelect, "Connection Selector")
	flag.StringVar(&runId, "runId", runId, "run file name not set")
	flag.Parse()

	if runId == "" {
		log.Println("No run id selected")
		os.Exit(251)
	}

	conn, err := sql.Open("mssql", connectionStrings["H2008"])
	if err != nil {
		log.Fatal("Open connection failed:", err.Error())
	}
	err = conn.Ping()
	if err != nil {
		log.Fatal("Ping failed:", err.Error())
	}
	defer conn.Close()

	queryParts(conn)
}

func queryParts(conn *sql.DB) {
	var runList = runerList{}
	stmt, err := conn.Prepare(`select 
							ID,
							"Lauf",
							"Plan",
							"ClassName",
							"Type",
							"IntID",
							Val 
						from 
							Cadmatic4.dbo.PieceCounter 
						WHERE 
							Lauf=?
							AND ClassName IN ('Rest', 'Teil')`)
	if err != nil {
		log.Fatal("Prepare failed:", err.Error())
	}
	defer stmt.Close()
	var isInitialRun = true
	for {
		rows, err := stmt.Query("176862-10")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		for rows.Next() {

			res := result{}
			response := Response{}
			err = rows.Scan(&res.id, &res.runName, &res.mapId, &res.actionResult, &res.actionType, &res.partid, &res.amount)
			if err != nil {
				log.Println("Scan failed:", err.Error())
			}
			if isInitialRun {
				runList[res.getIndex()] = res
				continue
			}
			runItem, ok := runList[res.getIndex()]
			if ok == false {
				runList[res.getIndex()] = res
				response.setId(res.runName)
				response.setPartType(res.actionResult)
				response.PartId = res.partid
				message, _ := json.Marshal(response)
				fmt.Println(string(message))
				continue
			}

			if runItem.amount != res.amount {
				runList[res.getIndex()] = res

				response.setId(res.runName)
				response.setPartType(res.actionResult)
				response.PartId = res.partid
				message, _ := json.Marshal(response)
				fmt.Println(string(message))
			}

		}
		isInitialRun = false
		if err = rows.Err(); err != nil {
			log.Println(err)
		}

		time.Sleep(time.Second * 1)
	}

}
