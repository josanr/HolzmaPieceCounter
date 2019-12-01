package monitor

import (
	"database/sql"
	"log"
	"strconv"
	"time"
)

//Board Board response from tool db
type Board struct {
	RecordID
	Info         BoardInfo
	MapID        int
	ActionType   string
	Error        bool
	ErrorMessage string
}

func (r *Board) setActionType(actionResult string) {
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

func monitorBoards(conn *sql.DB, syncer InfoSync, boardCan chan Board, errorChanel chan FatalResponse, exit chan bool) {
	var lastID int
	lastIDArr := conn.QueryRow(`
		SELECT 
			max(id) 
		from 
			Cadmatic4.dbo.PieceCounter  
		WHERE 
			ClassName NOT IN ('Rest', 'Teil');
			`)
	err := lastIDArr.Scan(&lastID)
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
						from Cadmatic4.dbo.PieceCounter
						WHERE id > ?
						AND ClassName NOT IN ('Rest', 'Teil')`)
	if err != nil {
		errorChanel <- FatalResponse{
			Error:        true,
			ErrorMessage: "Prepare Boards query failed:" + err.Error(),
		}
	}
	defer stmt.Close()

	for {
		rows, err := stmt.Query(lastID)
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
				ErrorMessage: "Query Boards error:" + err.Error(),
			}
			continue
		}

		for rows.Next() {

			res := result{}
			response := Board{}

			err = rows.Scan(&res.id, &res.runName, &res.mapID, &res.actionResult, &res.actionType, &res.partid, &res.amount)
			if err != nil {
				log.Println(err)
				continue
			}

			boardID, _ := strconv.Atoi(res.mapID)
			response.RunID = res.runName
			response.setActionType(res.actionType)
			response.MapID = boardID
			lastID = res.id
			boardInfo, _ := syncer.GetBoardByID(res.runName, res.partid)
			response.Info = boardInfo
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
				log.Println("closing watch of boards")
				return
			}
		default:
			time.Sleep(time.Second * 1)
		}

	}

}
