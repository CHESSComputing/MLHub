package main

// server module
//
// Copyright (c) 2023 - Valentin Kuznetsov <vkuznet@gmail.com>
//
import (
	"embed"
	"log"

	srvConfig "github.com/CHESSComputing/golib/config"
	mongo "github.com/CHESSComputing/golib/mongo"
	server "github.com/CHESSComputing/golib/server"
	services "github.com/CHESSComputing/golib/services"
	"github.com/gin-gonic/gin"
)

// content is our static web server content.
//
//go:embed static
var StaticFs embed.FS

var _httpReadRequest *services.HttpRequest
var Verbose int
var StaticDir, StorageDir string

// helper function to setup our router
func setupRouter() *gin.Engine {
	routes := []server.Route{
		server.Route{Method: "GET", Path: "/docs/:name", Handler: DocsHandler, Authorized: false},
		server.Route{Method: "GET", Path: "/models", Handler: ModelsHandler, Authorized: false},
		server.Route{Method: "GET", Path: "/models/:model", Handler: DownloadHandler, Authorized: true},

		server.Route{Method: "POST", Path: "/predict", Handler: PredictHandler, Authorized: true, Scope: "read"},
		server.Route{Method: "POST", Path: "/upload", Handler: UploadHandler, Authorized: true, Scope: "write"},

		server.Route{Method: "DELETE", Path: "/models/:model", Handler: DeleteHandler, Authorized: true, Scope: "delete"},
	}

	r := server.Router(routes, nil, "static", srvConfig.Config.MLHub.WebServer)
	return r
}

// Server defines our HTTP server
func Server() {
	Verbose = srvConfig.Config.MLHub.WebServer.Verbose
	StaticDir = srvConfig.Config.MLHub.WebServer.StaticDir
	StorageDir = srvConfig.Config.MLHub.ML.StorageDir
	log.Println("storage dir", StorageDir)
	_httpReadRequest = services.NewHttpRequest("read", Verbose)

	// init MongoDB
	log.Println("init mongo", srvConfig.Config.MLHub.MongoDB.DBUri)
	mongo.InitMongoDB(srvConfig.Config.MLHub.MongoDB.DBUri)

	// setup web router and start the service
	r := setupRouter()
	webServer := srvConfig.Config.MLHub.WebServer
	log.Printf("### webServer %+v", webServer)
	server.StartServer(r, webServer)
}
