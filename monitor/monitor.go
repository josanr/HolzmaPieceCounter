package monitor

import (
	"database/sql"
	"strconv"
)

type Config struct {
	RunPath string
	UserID  int
	ToolID  int
}

//RecordID identification for run
type RecordID struct {
	RunID string
}

//FatalResponse responce on error
type FatalResponse struct {
	RecordID
	Error        bool
	ErrorMessage string
}

type result struct {
	id           int    //ID
	partid       int    //IntID
	amount       int    //Val
	actionType   string //Type
	actionResult string //ClassName
	mapID        string //Plan
	runName      string //Lauf
}

func (r result) getIndex() string {

	return strconv.FormatInt(int64(r.id), 10) + ":" + r.runName + ":" + r.actionResult + ":" + strconv.FormatInt(int64(r.partid), 10)
}

//Counter information pipeline
type Counter struct {
	Boards chan Board
	Parts  chan Parts
	Errors chan FatalResponse
	Exit   chan bool
}

//New constructor
func New(db *sql.DB, config *Config, exitChan chan bool) Counter {
	errChan := make(chan FatalResponse)
	partsChan := make(chan Parts)
	boardsChan := make(chan Board)
	syncer := InfoSync{
		cache: make(map[string]InfoNode, 0),
	}
	syncer.setBasePath(config.RunPath)
	syncer.setActiveUser(&config.UserID)
	syncer.setActiveTool(config.ToolID)

	go monitorParts(db, syncer, partsChan, errChan, exitChan)
	go monitorBoards(db, syncer, boardsChan, errChan, exitChan)

	return Counter{
		Boards: boardsChan,
		Parts:  partsChan,
		Errors: errChan,
	}
}
