package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/archip-io/deployment/api-gateway/internal/cfg"
	"github.com/archip-io/deployment/api-gateway/internal/proxy"
	"log"
	"os"
)

var (
	cfgPath = flag.String("cfg", "./configs/cfg.yaml", "config file path")
	addres  = flag.String("addr", ":8080", "http service address")
)

func main() {
	flag.Parse()

	t := string("{\"err\":123}")

	var val map[string]interface{}

	err := json.Unmarshal([]byte(t), &val)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(val)

	file, err := os.Open(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	cfgs, err := cfg.GetCfgs(file)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfgs)

	backs, err := proxy.GetBackends(cfgs)
	if err != nil {
		log.Fatal(err)
	}
	err = proxy.ListenConnections(*addres, backs)

	if err != nil {
		log.Fatal(err)
	}
}
