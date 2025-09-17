package main

import (
	"context"
	"database/sql"
	"log"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"liveRecord/internal/config"
	"liveRecord/internal/web/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("connecting to db: %s", cfg.Database.URL)
	db, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		log.Fatal(err)
	}
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			log.Fatal("db ping failed: ", err)
		}
	}
	log.Printf("db connected")
	log.Printf("site title: %s", cfg.Site.Title)

	r := router.New(db, cfg)

	if _, err := strconv.Atoi(cfg.Server.Port); err != nil {
		log.Fatal("invalid PORT")
	}
	log.Printf("starting server on :%s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal(err)
	}
}
