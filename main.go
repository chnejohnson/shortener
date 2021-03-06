package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	_ "github.com/joho/godotenv/autoload"

	accountRepo "github.com/chnejohnson/shortener/service/account/repository/postgres"
	accountService "github.com/chnejohnson/shortener/service/account/service"

	redirectRepo "github.com/chnejohnson/shortener/service/redirect/repository/postgres"
	redirectService "github.com/chnejohnson/shortener/service/redirect/service"

	userURLRepo "github.com/chnejohnson/shortener/service/user_url/repository/postgres"
	userURLService "github.com/chnejohnson/shortener/service/user_url/service"

	api "github.com/chnejohnson/shortener/api"
)

var mode string

func init() {
	if d, _ := strconv.ParseBool(os.Getenv("DEBUG")); !d {
		mode = "production"
	} else {
		mode = "development"
	}

	log.Info(fmt.Sprintf("App is running on %s mode", mode))

	viper.SetConfigFile("config.json")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}

func main() {
	// JWT
	jwtSecret := viper.GetString("jwt.secret")
	j := &api.JWT{JWTSecret: []byte(jwtSecret)}

	// Postgres
	pgConfig := viper.GetStringMapString("pg")

	if mode == "development" {
		pgConfig["host"] = "127.0.0.1"
	}

	dsn := []string{}
	for key, val := range pgConfig {
		s := key + "=" + val
		dsn = append(dsn, s)
	}

	pgConn, err := pgx.Connect(context.Background(), strings.Join(dsn, " "))
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		err := pgConn.Close(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	}()

	// web framework
	e := echo.New()

	// middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	e.Pre(middleware.RemoveTrailingSlash())
	// service
	accountRepo := accountRepo.NewRepository(pgConn)
	as := accountService.NewAccountService(accountRepo)
	userURLRepo := userURLRepo.NewRepository(pgConn)
	us := userURLService.NewUserURLService(userURLRepo)
	redirectRepo := redirectRepo.NewRepository(pgConn)
	rs := redirectService.NewRedirectService(redirectRepo, userURLRepo)

	// api
	router := e.Group("/api")
	{
		api.NewAccountHandler(router, as, j)
		api.NewRedirectHandler(router, rs)

		// auth
		auth := router.Group("/auth")
		auth.Use(j.AuthRequired)
		{
			api.NewUserURLHandler(auth, us)
		}

	}

	e.Logger.Fatal(e.Start(viper.GetString("server.address")))
}
