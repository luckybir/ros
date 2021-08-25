package main

import (
	"ros/pkg/config"
	"ros/pkg/logger"
	"ros/pkg/route"
)

func main() {
	logger.Initlog()
	config.InitConfig()
	route.InitRoute()
}
