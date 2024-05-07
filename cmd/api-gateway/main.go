package main

import (
	"flag"
	"fmt"
	"github.com/archip-io/deployment/api-gateway/internal/cfg"
	"github.com/archip-io/deployment/api-gateway/internal/proxy"
	"log"
	"os"
)

var (
	cfgPath = flag.String("cfg", "./configs/cfg.yaml", "config file path")
	port    = flag.String("port", "8080", "http service address")
)

func main() {
	//fmt.Println("hello world")
	flag.Parse()

	//fmt.Println(*cfgPath)
	//fmt.Println(*port)
	addres := fmt.Sprintf(":%s", *port, *port)

	file, err := os.Open(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	cfgs, err := cfg.GetCfgs(file)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfgs)

	backs, err := proxy.GetBackends(cfgs)
	if err != nil {
		log.Fatal(err)
	}
	err = proxy.ListenConnections(addres, backs)

	if err != nil {
		log.Fatal(err)
	}
}
