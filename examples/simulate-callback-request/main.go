package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/interaction/signature"
)

const host = "http://localhost"
const port = ":9000"
const path = "/qqbot"
const url = host + port + path

type config struct {
	AppID  uint64 `yaml:"appid"`
	Secret string `yaml:"secret"`
}

func main() {
	content, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalln("load config file failed, err:", err)
	}

	conf := &config{}
	if err = yaml.Unmarshal(content, conf); err != nil {
		log.Fatalln("parse config failed, err:", err)
	}
	log.Println("config loaded")

	go simulateRequest(conf)
	var ln string
	fmt.Scanln(&ln)
	fmt.Println("end")
}

func simulateRequest(conf *config) {
	time.Sleep(3 * time.Second)

	heartbeat := &dto.Payload{
		PayloadBase: dto.PayloadBase{
			OPCode: dto.WSHeartbeat,
		},
		Data: 123,
	}
	payload, _ := json.Marshal(heartbeat)
	send(payload, conf)

	dispatchEvent := &dto.Payload{
		PayloadBase: dto.PayloadBase{
			OPCode: dto.DispatchEvent,
			Seq:    1,
			Type:   dto.EventMessageReactionAdd,
		},
		Data: dto.MessageReactionData{
			UserID:    "123",
			ChannelID: "111",
			GuildID:   "222",
			Target: dto.ReactionTarget{
				ID:   "333",
				Type: dto.ReactionTargetTypeMsg,
			},
			Emoji: dto.Emoji{ID: "42", Type: 1},
		},
	}
	payload, _ = json.Marshal(dispatchEvent)
	fmt.Println(string(payload))
	send(payload, conf)
}

func send(payload []byte, conf *config) {
	header := http.Header{}
	header.Set(signature.HeaderTimestamp, strconv.FormatUint(uint64(time.Now().Unix()), 10))

	sig, err := signature.Generate(conf.Secret, header, payload)
	if err != nil {
		fmt.Println(err)
		return
	}
	header.Set(signature.HeaderSig, sig)

	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header = header.Clone()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("receive resp: %s", string(body))
}
