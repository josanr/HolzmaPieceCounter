package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/josanr/HolzmaPieceCounter/monitor"
	"github.com/spf13/viper"
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
var monitorConfig monitor.Config
var theAPIClient *http.Client

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
	theAPIClient = http.DefaultClient
	db, err = connectHomag()
	if err == nil {
		log.Println("connected to homag")
		return
	}
	log.Println("Could not connect to Database homag....")

	db, err = connectHo2008()
	if err == nil {
		log.Println("connected to holzma")
		return
	}
	log.Fatal("Could not connect to Database holzma")
}

func main() {
	f, err := os.OpenFile("counter.log", os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	wrt := io.MultiWriter(os.Stdout, f)
	log.SetOutput(wrt)
	log.Println("Counter Monitor Started")

	defer db.Close()
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatal("Fatal error reading config file")
	}
	baseRunPath := viper.GetString("monitorBasePath")
	servicePort := viper.GetString("servicePort")
	toolID := viper.GetInt("toolId")

	UserID := viper.GetInt("defaultUserID")
	lastUserID := viper.GetInt("lastUserID")
	if lastUserID != 0 {
		UserID = lastUserID
	}
	monitorConfig := monitor.Config{
		RunPath: baseRunPath,
		UserID:  UserID,
		ToolID:  toolID,
	}

	http.HandleFunc("/setUid/", setUIDHandler)
	go http.ListenAndServe(":"+servicePort, nil)

	var connActive = make(chan bool)
	var monitorer = monitor.New(db, &monitorConfig, connActive)
	// var message []byte
	for {

		select {
		case board := <-monitorer.Boards:

			err = markBoard(board)
			log.Println("board sent")
		case part := <-monitorer.Parts:
			_, _ = json.Marshal(part)
		case ex := <-monitorer.Errors:
			_, _ = json.Marshal(ex)
		}
		// log.Println(string(message))
	}

}

func markBoard(board monitor.Board) error {
	urlString := viper.GetString("apiendpoints.board")

	form := url.Values{}
	form.Add("gid", strconv.Itoa(board.Info.Gid))
	form.Add("length", strconv.Itoa(board.Info.Length))
	form.Add("width", strconv.Itoa(board.Info.Width))
	form.Add("thick", strconv.Itoa(board.Info.Thick))

	form.Add("orderId", strconv.Itoa(board.Info.OrderID))
	form.Add("toolId", strconv.Itoa(board.Info.ToolID))
	form.Add("userId", strconv.Itoa(board.Info.UserID))

	form.Add("boardId", strconv.Itoa(board.Info.Id))
	form.Add("isFromOffcut", strconv.FormatBool(board.Info.IsFromOffcut))

	form.Add("actionType", board.ActionType)

	req, err := http.NewRequest(http.MethodPost, urlString, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := theAPIClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(io.LimitReader(resp.Body, 1048576))
	if err != nil {
		return err
	}

	reqResult := make(map[string]string)
	json.Unmarshal(body, &reqResult)

	if _, ok := reqResult["result"]; ok == false {
		return errors.New("wrong response format")
	}

	if v, _ := reqResult["result"]; v != "OK" {
		return errors.New("error set in response: " + v)
	}

	return nil
}

type okResponse struct {
	Message string `json:"message"`
}

func setUIDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path != "/setUid/" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "wrong request method", http.StatusBadRequest)
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "request form data corrupt", http.StatusBadRequest)
		return
	}
	uid := r.FormValue("uid")
	viper.Set("lastUserID", uid)
	viper.WriteConfig()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(okResponse{"action done, id set: " + strconv.Itoa(monitorConfig.UserID)})
	return
}
