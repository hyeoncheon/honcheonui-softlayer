package main

import (
	"encoding/json"
	"fmt"
	"os"
	"plugin"
	"time"

	spec "github.com/hyeoncheon/honcheonui-spec"
)

func main() {
	mod := "softlayer.so"

	plug, err := plugin.Open(mod)
	if err != nil {
		fmt.Printf("plugin open error: %v\n", err)
		os.Exit(1)
	}

	symbol, err := plug.Lookup("Provider")
	if err != nil {
		fmt.Printf("symbol lookup error: %v\n", err)
		os.Exit(2)
	}

	var provider spec.Provider
	provider, ok := symbol.(spec.Provider)
	if !ok {
		fmt.Printf("symbol cast error\n")
		os.Exit(3)
	}

	// test

	err = provider.Init()
	if err != nil {
		fmt.Printf("Init() error: %v\n", err)
	}

	user := os.Getenv("SL_USERNAME")
	pass := os.Getenv("SL_API_KEY")

	userID, accountID, err := provider.CheckAccount(user, pass)
	if err != nil {
		fmt.Printf("Error() error: %v\n", err)
	}
	fmt.Printf("-- user_id: %v, account_id: %v\n", userID, accountID)

	resources, err := provider.GetResources(user, pass)
	if err != nil {
		fmt.Printf("GetResources() error: %v\n", err)
	}
	if jsonbyte, err := json.Marshal(resources); err == nil {
		fmt.Printf("-- resources json: %v\n", string(jsonbyte))
	}

	statuses, err := provider.GetStatuses(user, pass)
	if err != nil {
		fmt.Printf("GetStatuses() error: %v\n", err)
	}
	if jsonbyte, err := json.Marshal(statuses); err == nil {
		fmt.Printf("-- statuses json: %v\n", string(jsonbyte))
	}

	notes, err := provider.GetNotifications(user, pass, time.Now().AddDate(-1, 0, 0))
	if err != nil {
		fmt.Printf("GetStatuses() error: %v\n", err)
	}
	if jsonbyte, err := json.Marshal(notes); err == nil {
		fmt.Printf("-- notifications json: %v\n", string(jsonbyte))
	}

	fmt.Println("-- done")
}
