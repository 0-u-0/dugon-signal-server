package main

import (
	"fmt"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/dugon-signal-server/")

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	port := viper.GetInt("port")
	//httpsEnable := viper.GetBool("https_enable")
	//cert := viper.GetString("cert")
	//key := viper.GetString("key")
	natsUrls := viper.GetStringSlice("nats_urls")


	fmt.Println(port)
	fmt.Println(natsUrls)

	//clientGroup := libs.NewClientGroup()
	//go clientGroup.Run()
	//libs.InitWsServer(clientGroup)
}
