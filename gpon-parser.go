package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vincejv/gpon-parser/core"
	"github.com/vincejv/gpon-parser/device"
	"github.com/vincejv/gpon-parser/svc"
	"github.com/vincejv/gpon-parser/util"
)

/**

Available ENVIRONMENT Variables

ONT_WEB_HOST
ONT_WEB_PORT
ONT_WEB_PROTOCOL
ONT_WEB_USER
ONT_WEB_PASS
ONT_TELNET_PORT
LISTEN_PORT
LISTEN_IP

**/

func main() {
	log.Println("Starting GPON Parser")

	initGponSvc()
	core.RunCronJobs()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/gpon/opticalInfo", svc.ServOpticalInfo)
	router.GET("/gpon/deviceInfo", svc.ServDeviceInfo)
	router.GET("/gpon/allInfo", svc.ServAllInfo)
	router.GET("/health", svc.Health)

	ip := util.Getenv("LISTEN_IP", "0.0.0.0")
	port := util.Getenv("LISTEN_PORT", "8092")
	log.Printf("Starting web server on %s:%s\n", ip, port)
	log.Printf("Polling ONT every %ss", util.Getenv("ONT_POLL_SEC", "60"))
	router.Run(fmt.Sprintf("%s:%s", ip, port))
}

func initGponSvc() {
	device.TelnetInit.SetFlag(false)
	device.TelnetScripts.SetFlag(false)
	device.SvcHealth.SetFlag(false)

	model := os.Getenv("ONT_MODEL")
	if len(model) > 0 {
		log.Print("ONT Model is ")
		if strings.EqualFold(model, "an5506_stock") {
			log.Println("Fiberhome AN5506")
			device.GponSvc = new(device.AN5506_Stock)
		} else if strings.EqualFold(model, "hg6245d_globe") {
			log.Println("Fiberhome HG6245D")
			device.GponSvc = new(device.HG6245D_Globe)
		} else if strings.EqualFold(model, "zte_f670") {
			log.Println("ZTE F670L")
			device.GponSvc = new(device.ZTEF670L)
		} else if strings.EqualFold(model, "zlt_g3000a") {
			log.Println("ZLT G3000A WiFi 6")
			device.GponSvc = new(device.ZLTG3000A)
		} else if strings.EqualFold(model, "zlt_g202") {
			log.Println("ZLT G202 WiFi 5")
			device.GponSvc = new(device.ZLTG202)
		} else if strings.EqualFold(model, "skyworth_gn630v") {
			log.Println("Skyworth GN630V WiFi 6")
			device.GponSvc = new(device.SKYW_GN)
		} else if strings.Contains(strings.ToLower(model), "skyworth_gn256") {
			log.Println("Skyworth GN256 WiFi 5")
			device.GponSvc = new(device.SKYW_GN)
		} else if strings.EqualFold(model, "nokia_g010s") {
			log.Println("Nokia G-010S-A/P SFP Module")
			device.GponSvc = new(device.NOKIA_G010S)
		} else {
			log.Println("Invalid ONT model provided in env variable 'ONT_MODEL', valid args are ['an5506_stock', 'hg6245d_globe', 'zte_f670', 'zlt_g3000a', 'zlt_g202', 'skyworth_gn630v', 'skyworth_gn256', 'nokia_g010s']")
			os.Exit(-10)
		}
	} else {
		// by default use AN5506 stock
		log.Println("Did not specify any ONT models in env variable 'ONT_MODEL', using default model Fiberhome AN5506")
		device.GponSvc = new(device.AN5506_Stock)
	}
}
