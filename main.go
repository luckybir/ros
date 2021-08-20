package main

import (
	"ros/config"
	"ros/logger"
	"ros/route"
)

func main() {
	logger.Initlog()
	config.InitConfig()
	route.InitRoute()
}
