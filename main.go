package main

import (
	"fmt"
	"github.com/0-u-0/dugon-signal-server/libs"
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
	httpsEnable := viper.GetBool("https_enable")
	cert := viper.GetString("cert")
	key := viper.GetString("key")
	natsUrls := viper.GetStringSlice("nats_urls")

	fmt.Println(viper.AllSettings())

	clientGroup := libs.NewClientGroup(natsUrls)
	go clientGroup.Run()
	libs.InitWsServer(clientGroup, port, httpsEnable, cert, key)
}
