package counter

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"
)

type PartsResponse struct {
	OrderId      int    `json:"orderId"`
	ToolId       int    `json:"toolId"`
	IsOffcut     bool   `json:"isOffcut"`
	PartId       int64  `json:"partId"`
	PartAmount   int64  `json:"partAmount"`
	Error        bool   `json:"isError"`
	ErrorMessage string `json:"message"`
}

func (r *PartsResponse) setId(runName string) {
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
func (r *PartsResponse) setPartType(actionResult string) {
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

type Counter struct {
	Boards chan BoardResponse
	Parts  chan PartsResponse
	Errors chan FatalResponse
	Exit   chan bool
}

func New(db *sql.DB, runId string, exit chan bool) Counter {

	errChan := make(chan FatalResponse)
	partsChan := make(chan PartsResponse)
	boardsChan := make(chan BoardResponse)

	go queryParts(db, runId, partsChan, errChan, exit)
	go queryBoards(db, runId, boardsChan, errChan, exit)

	return Counter{
		Boards: boardsChan,
		Parts:  partsChan,
		Errors: errChan,
	}
}

func queryParts(conn *sql.DB, runId string, partsChan chan PartsResponse, errorChanel chan FatalResponse, exit chan bool) {

	var runList = make(map[string]result)

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
		errorChanel <- FatalResponse{
			Error:        true,
			ErrorMessage: "Prepare Parts query failed:" + err.Error(),
		}
		return
	}
	defer stmt.Close()

	initialPartsSync(stmt, runId, runList)

	for {

		rows, err := stmt.Query(runId)
		if err != nil {
			errorChanel <- FatalResponse{
				Error:        true,
				ErrorMessage: "Query Parts failed:" + err.Error(),
			}
			return
		}

		for rows.Next() {

			res := result{}
			response := PartsResponse{}

			err = rows.Scan(&res.id, &res.runName, &res.mapId, &res.actionResult, &res.actionType, &res.partid, &res.amount)
			if err != nil {
				errorChanel <- FatalResponse{
					Error:        true,
					ErrorMessage: "Part Scan failed:" + err.Error(),
				}
				continue
			}

			runItem, ok := runList[res.getIndex()]
			if ok == false {
				response.setId(res.runName)
				response.setPartType(res.actionResult)
				response.PartId = res.partid
				response.PartAmount = res.amount
				runList[res.getIndex()] = res

				partsChan <- response
				continue
			}

			if runItem.amount != res.amount {
				response.setId(res.runName)
				response.setPartType(res.actionResult)
				response.PartId = res.partid
				response.PartAmount = res.amount - runItem.amount
				runList[res.getIndex()] = res

				partsChan <- response
			}

		}

		if err = rows.Err(); err != nil {
			errorChanel <- FatalResponse{
				Error:        true,
				ErrorMessage: "Error on rows:" + err.Error(),
			}
		}
		_ = rows.Close()

		select {
		case _, ok := <-exit:
			if !ok {
				log.Println("closing watch of parts for run: " + runId)
				return
			}
		default:
			time.Sleep(time.Second * 1)
		}

	}

}

func initialPartsSync(stmt *sql.Stmt, runId string, runList map[string]result) {
	rows, err := stmt.Query(runId)
	if err != nil {
		log.Println("initial sync error")
		return
	}

	for rows.Next() {

		res := result{}

		err = rows.Scan(&res.id, &res.runName, &res.mapId, &res.actionResult, &res.actionType, &res.partid, &res.amount)
		if err != nil {
			log.Println("initial sync error Part Scan failed:" + err.Error())
			continue
		}
		runList[res.getIndex()] = res
		continue
	}
	if err = rows.Err(); err != nil {
		log.Print("initial sync error Error on rows:" + err.Error())
	}
}

func queryBoards(conn *sql.DB, runId string, boardCan chan BoardResponse, errorChanel chan FatalResponse, exit chan bool) {
	var lastID int64
	lastIdArr := conn.QueryRow(`
		SELECT 
			max(id) 
		from 
			Cadmatic4.dbo.PieceCounter  
		WHERE 
			Lauf = '` + runId + `'
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
		errorChanel <- FatalResponse{
			Error:        true,
			ErrorMessage: "Prepare Boards query failed:" + err.Error(),
		}
	}
	defer stmt.Close()

	for {
		rows, err := stmt.Query(runId, lastID)
		switch {
		case err == sql.ErrNoRows:
			_ = rows.Close()
			log.Println("no rows")
			time.Sleep(time.Second * 1)
			continue
		case err != nil:
			log.Println(err)
			errorChanel <- FatalResponse{
				Error:        true,
				ErrorMessage: "Query Boards failed:" + err.Error(),
			}
			continue
		}

		for rows.Next() {

			res := result{}
			response := BoardResponse{}

			err = rows.Scan(&res.id, &res.runName, &res.mapId, &res.actionResult, &res.actionType, &res.partid, &res.amount)
			if err != nil {
				log.Println(err)
				continue
			}

			boardId, _ := strconv.Atoi(res.mapId)
			response.setId(res.runName)
			response.setActionType(res.actionType)
			response.BoardId = boardId
			lastID = res.id
			boardCan <- response
		}
		if err = rows.Err(); err != nil {
			errorChanel <- FatalResponse{
				Error:        true,
				ErrorMessage: "Error on board rows:" + err.Error(),
			}
		}
		_ = rows.Close()
		select {
		case _, ok := <-exit:
			if !ok {
				log.Println("closing watch of boards for run: " + runId)
				return
			}
		default:
			time.Sleep(time.Second * 1)
		}

	}

}
