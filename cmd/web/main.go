package main

import (
	"database/sql"
	"encoding/gob"
	"fmt"
	"go-subscription/data"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
)

const webPort = "80"

func main() {
	db := initDB()
	db.Ping()

	session := initSession()

	infoLog := log.New(os.Stdout, "INFO\t, ", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t, ", log.Ldate|log.Ltime|log.Lshortfile)

	wg := sync.WaitGroup{}
	app := Config{
		Session:  session,
		DB:       db,
		InfoLog:  infoLog,
		ErrorLog: errorLog,
		Wait:     &wg,
		Models:   data.New(db),
	}

	app.Mailer = app.createMail()

	go app.listenForMail()

	go app.listenForShutdown()

	app.serve()

}

func (app *Config) serve() {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", webPort),
		Handler: app.routes(),
	}
	app.InfoLog.Println("Starting web server...")
	err := srv.ListenAndServe()
	if err != nil {
		log.Panic("Can't start server", err)
	}
}

func initDB() *sql.DB {
	conn := connectToDB()
	if conn == nil {
		log.Panic("Can't connect to DB")
	}
	return conn
}

func connectToDB() *sql.DB {
	counts := 0
	dsn := os.Getenv("DSN")
	for {
		connection, err := openDB(dsn)
		if err != nil {
			log.Println("Postgres not ready...")
		} else {
			log.Println("Connected to DB...")
			return connection
		}
		if counts > 10 {
			return nil
		}
		log.Println("Waiting a second...")
		time.Sleep(time.Second)
		counts++
		continue
	}

}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return db, nil

}

func initSession() *scs.SessionManager {
	gob.Register(data.User{})
	session := scs.New()
	session.Store = redisstore.New(initRedis())
	session.Lifetime = 24 * time.Hour
	session.Cookie.Persist = true
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Secure = true
	return session

}

func initRedis() *redis.Pool {
	redisPool := &redis.Pool{
		MaxIdle: 10,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", os.Getenv("REDIS"))
		},
	}
	return redisPool
}

func (app *Config) listenForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.shutdown()
	os.Exit(0)
}

func (app *Config) shutdown() {
	// clean
	app.InfoLog.Println("cleanup tasks")

	app.Wait.Wait()
	app.Mailer.Wait.Wait()
	app.Mailer.DoneChan <- true

	app.InfoLog.Println("Closing channels and shutting down")
	close(app.Mailer.MailerChan)
	close(app.Mailer.ErrorChan)
	close(app.Mailer.DoneChan)

}

func (app *Config) createMail() Mail {
	errorChan := make(chan error)
	mailerChan := make(chan Message, 100)
	doneChan := make(chan bool)
	m := Mail{
		Domain:      "localhost",
		Host:        "localhost",
		Port:        1025,
		Encryption:  "none",
		FromName:    "Info",
		FromAddress: "info@some.com",
		ErrorChan:   errorChan,
		MailerChan:  mailerChan,
		DoneChan:    doneChan,
		Wait:        &sync.WaitGroup{},
	}
	return m

}
