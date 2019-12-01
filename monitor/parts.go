package monitor

import (
	"database/sql"
	"log"
	"time"
)

//Parts response for part execution
type Parts struct {
	RecordID
	Info         PartInfo
	PartID       int
	PartAmount   int
	IsOffcut     bool
	Error        bool
	ErrorMessage string
}

func (r *Parts) setPartType(actionResult string) {
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

func monitorParts(conn *sql.DB, syncer InfoSync, partsChan chan Parts, errorChanel chan FatalResponse, exit chan bool) {

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
							ClassName IN ('Rest', 'Teil')`)
	if err != nil {
		errorChanel <- FatalResponse{
			Error:        true,
			ErrorMessage: "Prepare Parts query failed:" + err.Error(),
		}
		return
	}
	defer stmt.Close()

	initialPartsSync(stmt, runList)

	for {

		rows, err := stmt.Query()
		if err != nil {
			errorChanel <- FatalResponse{
				Error:        true,
				ErrorMessage: "Query Parts failed:" + err.Error(),
			}
			return
		}

		for rows.Next() {

			res := result{}
			response := Parts{}

			err = rows.Scan(&res.id, &res.runName, &res.mapID, &res.actionResult, &res.actionType, &res.partid, &res.amount)
			if err != nil {
				errorChanel <- FatalResponse{
					Error:        true,
					ErrorMessage: "Part Scan failed:" + err.Error(),
				}
				continue
			}

			runItem, ok := runList[res.getIndex()]
			if ok == false {
				response.RunID = res.runName
				response.setPartType(res.actionResult)
				response.PartID = res.partid
				response.PartAmount = res.amount
				runList[res.getIndex()] = res

				if response.IsOffcut == true {
					partInfo, _ := syncer.GetOffcutByID(res.runName, res.partid)
					response.Info = partInfo
				} else {
					partInfo, _ := syncer.GetPartByID(res.runName, res.partid)
					response.Info = partInfo
				}

				partsChan <- response
				continue
			}

			if runItem.amount != res.amount {
				response.RunID = res.runName
				response.setPartType(res.actionResult)
				response.PartID = res.partid
				response.PartAmount = res.amount - runItem.amount
				runList[res.getIndex()] = res
				if response.IsOffcut == true {
					partInfo, _ := syncer.GetOffcutByID(res.runName, res.partid)
					response.Info = partInfo
				} else {
					partInfo, _ := syncer.GetPartByID(res.runName, res.partid)
					response.Info = partInfo
				}
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
				log.Println("closing watch of parts")
				return
			}
		default:
			time.Sleep(time.Second * 1)
		}

	}

}

func initialPartsSync(stmt *sql.Stmt, runList map[string]result) {
	rows, err := stmt.Query()
	if err != nil {
		log.Println("initial sync error")
		return
	}

	for rows.Next() {

		res := result{}

		err = rows.Scan(&res.id, &res.runName, &res.mapID, &res.actionResult, &res.actionType, &res.partid, &res.amount)
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
