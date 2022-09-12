package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

const initSql string = `
  CREATE TABLE IF NOT EXISTS notifies (
  id INTEGER NOT NULL PRIMARY KEY,
  type STRING,
  time DATETIME NOT NULL,
  message TEXT
  );`

type Config struct {
	BotToken        string            `json:"botToken"`
	ChatId          int64             `json:"chatId"`
	Tokens          []string          `json:"tokens"`
	DurationTimeout []DurationTimeout `json:"durationTimeout"`
}

type DurationTimeout struct {
	Type          string  `json:"type"`
	TimeoutSecond float64 `json:"timeoutSecond"`
}

type Notify struct {
	Id      int64
	Type    string
	Time    time.Time
	Message string
}

type NotifyRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Token   string `json:"token"`
}

type Notifies struct {
	mu sync.Mutex
	db *sql.DB
}

var tgBot *tgbotapi.BotAPI = nil
var notifies *Notifies = nil
var config Config

func main() {
	var err error

	config = readConfig()
	notifies, err = initDb()
	if err != nil {
		panic(err)
	}

	tgBot, err = tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		panic(err)
	}

	tgBot.Debug = true

	http.HandleFunc("/notify/", handler)
	http.ListenAndServe(":"+os.Getenv("TGNOTIFY_PORT"), nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var notifyRequest NotifyRequest
	if err := dec.Decode(&notifyRequest); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !checkRequestToken(notifyRequest.Token) {
		http.Error(w, "Invalid token", http.StatusForbidden)
		return
	}

	if !checkTimeoutNotify(notifyRequest.Type) {
		w.WriteHeader(http.StatusFound)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"notification timeout"}`))
		return
	}

	notify := Notify{Type: notifyRequest.Type, Message: notifyRequest.Message, Time: time.Now()}

	msg := tgbotapi.NewMessage(config.ChatId, notify.Message)
	msg.DisableNotification = false

	_, err := tgBot.Send(msg)
	if err != nil {
		fmt.Println("не удалось отправить сообщение ", err)
	}

	saveNotify(notify)

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "notified"}`))
}

func initDb() (*Notifies, error) {
	db, err := sql.Open("sqlite3", "database.sqlite3")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(initSql); err != nil {
		return nil, err
	}
	return &Notifies{
		db: db,
	}, nil
}

func readConfig() Config {
	pwd, _ := os.Getwd()
	jsonFile, err := os.Open(pwd + "/config.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		panic(err)
	}

	byteValue, _ := io.ReadAll(jsonFile)

	var config Config
	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		panic(err)
	}

	defer jsonFile.Close()

	return config
}

func checkRequestToken(token string) bool {
	for _, validTokens := range config.Tokens {
		if validTokens == token {
			return true
		}
	}
	return false
}

func fetchLastOfType(notifyType string) *Notify {
	// Query DB row based on ID
	row := notifies.db.QueryRow("SELECT id, type, time, message FROM notifies WHERE type=? ORDER BY time desc LIMIT 1", notifyType)

	// Parse row into Activity struct
	notify := Notify{}
	var err error
	if err = row.Scan(&notify.Id, &notify.Type, &notify.Time, &notify.Message); err == sql.ErrNoRows {
		return nil
	}
	return &notify
}

func saveNotify(notify Notify) (int, error) {
	res, err := notifies.db.Exec("INSERT INTO notifies VALUES(NULL,?,?,?);", notify.Type, notify.Time.Format(time.RFC3339), notify.Message)
	if err != nil {
		return 0, err
	}

	var id int64
	if id, err = res.LastInsertId(); err != nil {
		return 0, err
	}
	return int(id), nil
}

func checkTimeoutNotify(typeNotify string) bool {
	for _, timeout := range config.DurationTimeout {
		if timeout.Type == typeNotify {
			lastNotify := fetchLastOfType(typeNotify)
			if lastNotify == nil {
				return true
			}

			return time.Now().Sub(lastNotify.Time).Seconds() > timeout.TimeoutSecond
		}
	}

	return true
}
