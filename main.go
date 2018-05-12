package main

import (
	"config"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"netctrl"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	flag.Parse()
	configFile := flag.Arg(0)
	if configFile == "" {
		configFile = "config.hcl"
	}
	c, err := config.LoadConfigFile(configFile)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	s := makeServer(c)
	log.Println("Listening on " + c.Listener + " ...")
	go s.ListenAndServe()
	defer s.Shutdown(context.Background())

	ctr, err := netctrl.NewController(c)
	if err != nil {
		fmt.Printf("Failed to start network controller: %v\n", err)
		return
	}
	defer ctr.Close()

	if len(c.VPNConfigurations) > 0 {
		if err := ctr.SetVPN(&c.VPNConfigurations[0]); err != nil {
			fmt.Printf("Failed to setup VPN %q: %v\n", c.VPNConfigurations[0].Name, err)
			return
		}
	}

	for {
		sig := waitInterrupt()
		if sig == syscall.SIGHUP {
			fmt.Println("Got SIGHUP")
		} else {
			return
		}
	}
}

func makeServer(c *config.Config) *http.Server {
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		d, _ := ioutil.ReadFile("static/index.html")
		w.Write(d)
	})
	s := &http.Server{
		Addr: c.Listener,
	}
	return s
}

func waitInterrupt() os.Signal {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	return <-sig
}
