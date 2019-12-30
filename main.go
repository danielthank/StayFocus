package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/coreos/go-systemd/daemon"
	"github.com/robfig/cron"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Ip       string `yaml:"ip"`
	Hostname string `yaml:"hostname"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

var config Config

func login(client *http.Client) ([]*http.Cookie, error) {
	jsonStr := fmt.Sprintf(`{"name":"%s","password":"%s"}`, config.Username, config.Password)
	url := fmt.Sprintf("https://%s/control/login", config.Ip)

	req, err := http.NewRequest("POST",
		url,
		bytes.NewBuffer([]byte(jsonStr)))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return resp.Cookies(), nil
}

func printBlockedList(client *http.Client, cookie *http.Cookie) error {
	url := fmt.Sprintf("https://%s/control/blocked_services/list", config.Ip)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.AddCookie(cookie)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(byteArray))
	return nil
}

func setBlockedList(client *http.Client, cookie *http.Cookie, jsonStr string) error {
	url := fmt.Sprintf("https://%s/control/blocked_services/set", config.Ip)

	req, err := http.NewRequest("POST",
		url,
		bytes.NewBuffer([]byte(jsonStr)))
	if err != nil {
		return err
	}

	req.AddCookie(cookie)

	_, err = client.Do(req)
	return err
}

func unblockAll(client *http.Client) error {
	cookie, err := login(client)
	if err != nil {
		return err
	}

	err = setBlockedList(client, cookie[0], "[]")
	if err != nil {
		return err
	}
	return nil
}

func blockAll(client *http.Client) error {
	cookie, err := login(client)
	if err != nil {
		return err
	}

	err = setBlockedList(client,
		cookie[0],
		`["facebook","whatsapp","instagram","twitter","youtube","netflix","snapchat","messenger","twitch","discord","skype","amazon","ebay","origin","cloudflare","steam","epic_games","reddit","ok","vk","mail_ru","tiktok"]`)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	// The path should be absolute path
	f, err := os.Open("config.yml")
	if err != nil {
		log.Fatalln("Cannot open config.yml")
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatalln("Cannot process yaml config", err)
	}

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{ServerName: config.Hostname},
	}}

	c := cron.New()
	c.AddFunc("0 23 * * *", func() {
		err := unblockAll(client)
		if err != nil {
			log.Println("Cannot unblock", err)
		} else {
			log.Println("Unblocking success")
		}
	})
	c.AddFunc("0 0 * * *", func() {
		err := blockAll(client)
		if err != nil {
			log.Println("Cannot block", err)
		} else {
			log.Println("Blocking success")
		}
	})
	c.Start()
	daemon.SdNotify(false, "READY=1")
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, os.Kill)
	<-quit
}
