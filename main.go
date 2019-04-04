package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/namsral/flag"
)

var connectionStrings = map[string]string{
	"Homag": "server=localhost\\Holzma;user id=sa;password=holzma;encrypt=disable",
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
	PartAmount   int64  `json:"partAmount"`
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

type BoardResponse struct {
	OrderId      int    `json:"orderId"`
	ToolId       int    `json:"toolId"`
	BoardId      int    `json:"boardId"`
	ActionType   string `json:"actionType"`
	Error        bool   `json:"isError"`
	ErrorMessage string `json:"message"`
}

func (r *BoardResponse) setId(runName string) {
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
func (r *BoardResponse) setActionType(actionResult string) {
	switch actionResult {
	case "Eingestapelt":
		r.ActionType = "start"
	case "Produziert":
		r.ActionType = "end"
	default:
		r.Error = true
		r.ErrorMessage = "action type is not palete or produced"
	}
}

type FatalResponse struct {
	OrderId      int    `json:"orderId"`
	ToolId       int    `json:"toolId"`
	Error        bool   `json:"isError"`
	ErrorMessage string `json:"message"`
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
		message, _ := json.Marshal(FatalResponse{
			Error:        true,
			ErrorMessage: "No run id selected",
		})
		fmt.Println(string(message))
		os.Exit(251)
	}

	conn, err := sql.Open("mssql", connectionStrings["H2008"])
	if err != nil {
		message, _ := json.Marshal(FatalResponse{
			Error:        true,
			ErrorMessage: "Open connection failed:" + err.Error(),
		})
		fmt.Println(string(message))
		os.Exit(251)
	}
	err = conn.Ping()
	if err != nil {
		message, _ := json.Marshal(FatalResponse{
			Error:        true,
			ErrorMessage: "Ping failed:" + err.Error(),
		})
		fmt.Println(string(message))
		os.Exit(251)
	}
	defer conn.Close()

	go queryBoards(conn, runId)
	queryParts(conn, runId)
}

func queryParts(conn *sql.DB, runId string) {
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
		message, _ := json.Marshal(FatalResponse{
			Error:        true,
			ErrorMessage: "Prepare parts query failed:" + err.Error(),
		})
		fmt.Println(string(message))
		os.Exit(251)
	}
	defer stmt.Close()
	var isInitialRun = true
	for {
		rows, err := stmt.Query(runId)
		if err != nil {
			message, _ := json.Marshal(FatalResponse{
				Error:        true,
				ErrorMessage: "Query parts failed:" + err.Error(),
			})
			fmt.Println(string(message))
			os.Exit(251)
		}
		defer rows.Close()

		for rows.Next() {

			res := result{}
			response := Response{}
			err = rows.Scan(&res.id, &res.runName, &res.mapId, &res.actionResult, &res.actionType, &res.partid, &res.amount)
			if err != nil {
				message, _ := json.Marshal(FatalResponse{
					Error:        true,
					ErrorMessage: "Part Scan failed:" + err.Error(),
				})
				fmt.Println(string(message))
				continue
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
				response.PartAmount = res.amount
				message, _ := json.Marshal(response)
				fmt.Println(string(message))
				continue
			}

			if runItem.amount != res.amount {
				response.setId(res.runName)
				response.setPartType(res.actionResult)
				response.PartId = res.partid
				response.PartAmount = res.amount - runItem.amount
				message, _ := json.Marshal(response)
				runList[res.getIndex()] = res
				fmt.Println(string(message))
			}

		}
		isInitialRun = false
		if err = rows.Err(); err != nil {
			message, _ := json.Marshal(FatalResponse{
				Error:        true,
				ErrorMessage: "Error on rows:" + err.Error(),
			})
			fmt.Println(string(message))
		}

		time.Sleep(time.Second * 1)
	}

}

func queryBoards(conn *sql.DB, runId string) {
	var lastID int64 = 0
	lastIdArr := conn.QueryRow(`
		SELECT 
			max(id) 
		from 
			Cadmatic4.dbo.PieceCounter  
		WHERE 
			Lauf = '176862-10'
			AND ClassName NOT IN ('Rest', 'Teil');
			`)
	err := lastIdArr.Scan(&lastID)
	if err != nil {
		lastID = 0
	}
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
							Lauf = ?
							AND id > ?
							AND ClassName NOT IN ('Rest', 'Teil')`)
	if err != nil {
		message, _ := json.Marshal(FatalResponse{
			Error:        true,
			ErrorMessage: "Prepare boards query failed:" + err.Error(),
		})
		fmt.Println(string(message))
	}
	defer stmt.Close()

	for {
		rows, err := stmt.Query(runId, lastID)
		switch {
		case err == sql.ErrNoRows:
			time.Sleep(time.Second * 1)
			continue
		case err != nil:
			message, _ := json.Marshal(FatalResponse{
				Error:        true,
				ErrorMessage: "Query boards failed:" + err.Error(),
			})
			fmt.Println(string(message))
		}
		defer rows.Close()

		for rows.Next() {

			res := result{}
			response := BoardResponse{}
			err = rows.Scan(&res.id, &res.runName, &res.mapId, &res.actionResult, &res.actionType, &res.partid, &res.amount)
			if err != nil {
				continue
			}

			boardId, _ := strconv.Atoi(res.mapId)
			response.setId(res.runName)
			response.setActionType(res.actionType)
			response.BoardId = boardId
			lastID = res.id
			message, _ := json.Marshal(response)
			fmt.Println(string(message))
		}
		if err = rows.Err(); err != nil {
			message, _ := json.Marshal(FatalResponse{
				Error:        true,
				ErrorMessage: "Error on board rows:" + err.Error(),
			})
			fmt.Println(string(message))
		}

		time.Sleep(time.Second * 1)
	}

}
