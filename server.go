package main

import (
	"encoding/json"
	errs "errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type ServerInfo struct {
	BuildDate time.Time `json:"build_date"`
	BuildSha  string    `json:"build_sha"`
	Version   string    `json:"version"`
}

func (info ServerInfo) String() string {
	if info.Version == "" {
		return "unknown apm-server version"
	}
	return fmt.Sprintf("apm-server version %s built on %d %s [%s]",
		info.Version, info.BuildDate.Day(), info.BuildDate.Month().String(), info.BuildSha[:7])
}

func QueryServerInfo(secret, url string) (ServerInfo, error) {
	req, _ := http.NewRequest("GET", url, nil)
	if secret != "" {
		req.Header.Set("Authorization", "Beater "+secret)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()

	info := ServerInfo{}

	if err != nil {
		return info, err
	}

	if resp.StatusCode != 200 {
		return info, errs.New("server status not OK: " + resp.Status)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &info)
	return info, err
}
