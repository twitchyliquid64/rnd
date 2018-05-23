package main

import (
	"config"
	"context"
	"encoding/json"
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

	ctr, err := netctrl.NewController(c)
	if err != nil {
		fmt.Printf("Failed to start network controller: %v\n", err)
		return
	}
	defer ctr.Close()

	s := makeServer(c, ctr)
	log.Println("Listening on " + c.Listener + " ...")
	go s.ListenAndServe()
	defer s.Shutdown(context.Background())

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

func makeServer(c *config.Config, ctr *netctrl.Controller) *http.Server {
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		d, _ := ioutil.ReadFile("static/index.html")
		w.Write(d)
	})
	http.HandleFunc("/status", func(w http.ResponseWriter, req *http.Request) {
		d, _ := json.Marshal(ctr.GetState())
		w.Write(d)
	})

	http.HandleFunc("/vpns", func(w http.ResponseWriter, req *http.Request) {
		d, _ := json.Marshal(c.VPNConfigurations)
		w.Write(d)
	})

	http.HandleFunc("/setVPN", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var vpn *config.VPNOpt
		for _, v := range c.VPNConfigurations {
			if v.Name == input.Name {
				vpn = &v
				break
			}
		}
		if vpn == nil {
			http.Error(w, "No VPN with that name", http.StatusBadRequest)
			return
		}

		if err := ctr.SetVPN(vpn); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
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
