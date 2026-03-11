package main

import (
	"fmt"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/event"
)

// ThreadEventHandler handles forum thread events.
func ThreadEventHandler() event.ThreadEventHandler {
	return func(event *dto.Payload, data *dto.ThreadData) error {
		fmt.Println(event, data)
		return nil
	}
}
