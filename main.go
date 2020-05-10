package main

import (
	"github.com/0-u-0/dugon-signal-server/libs"
	"github.com/spf13/viper"
	"log"
)

func main() {
	defer func() {
		libs.ReleaseLoggerModule()
	}()

	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/dugon-signal-server/")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Fatal error config file: %s \n", err)
	}

	logLevel := viper.GetString("log_level")
	logToFile := viper.GetBool("log_to_file")
	logFilePath := viper.GetString("log_file_path")
	logErrorPath := viper.GetString("err_log_file_path")

	libs.InitGlobalLog()
	libs.LoadLoggerModule(logLevel, logToFile, logFilePath, logErrorPath)

	port := viper.GetInt("port")
	httpsEnable := viper.GetBool("https_enable")
	cert := viper.GetString("cert")
	key := viper.GetString("key")
	natsUrls := viper.GetStringSlice("nats_urls")

	libs.Log.Info("Config ->",viper.AllSettings())

	clientGroup := libs.NewClientGroup(natsUrls)
	go clientGroup.Run()
	libs.InitWsServer(clientGroup, port, httpsEnable, cert, key)
}
