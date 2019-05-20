package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/denisenkom/go-mssqldb"
	"github.com/gorilla/websocket"
	"github.com/josanr/HolzmaPieceCounter/counter"
	"log"
	"net/http"
	"time"
)

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

var connectionStrings = map[string]string{
	"Homag": "server=localhost\\Holzma;user id=sa;password=holzma;encrypt=disable",
	"H2008": "server=localhost\\HO2008;user id=sa;password=Homag;encrypt=disable"}

var db *sql.DB
var err error

func connectHomag() (*sql.DB, error) {
	db, err := sql.Open("mssql", connectionStrings["Homag"])
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func connectHo2008() (*sql.DB, error) {
	db, err := sql.Open("mssql", connectionStrings["H2008"])
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func init() {
	db, err = connectHomag()
	if err != nil {
		db, err = connectHo2008()
		if err != nil {
			log.Fatal("Could not connect to Database")
		}
	}
}

const (
	pongWait = 60 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	defer db.Close()

	http.HandleFunc("/counter", func(w http.ResponseWriter, r *http.Request) {

		var runId = r.FormValue("runId")
		if runId == "" {
			log.Println("no run id requested")
			http.Error(w, "no run id requested", http.StatusBadRequest)
			return
		}
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}

		_ = ws.SetReadDeadline(time.Now().Add(pongWait))
		ws.SetPongHandler(func(string) error { _ = ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

		var connActive = make(chan bool)
		var query = counter.New(db, runId, connActive)

		var message []byte
		for {

			select {
			case board := <-query.Boards:
				fmt.Println(board.BoardId)
				message, _ = json.Marshal(board)
			case part := <-query.Parts:
				fmt.Println(part.PartId)
				message, _ = json.Marshal(part)
			case ex := <-query.Errors:
				fmt.Println(ex.ErrorMessage)
				message, _ = json.Marshal(ex)
			}
			//log.Println(string(message))
			if err = ws.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Println(err)
				close(connActive)
				return
			}
		}

	})

	err = http.ListenAndServe(":1603", nil)
	if err != nil {
		log.Fatal("Could not start server")
	}
}
