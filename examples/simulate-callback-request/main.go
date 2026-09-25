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

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/interaction/signature"
	sdklog "github.com/WindowsSov8forUs/botgo-plus/log"
	"github.com/WindowsSov8forUs/botgo-plus/token"
	"gopkg.in/yaml.v3"
)

const host = "http://localhost"
const port = ":9000"
const path = "/qqbot"
const url = host + port + path

func main() {
	// 加载 appid 和 token
	content, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalln("load config file failed, err:", err)
	}
	credentials := &token.QQBotCredentials{}
	if err = yaml.Unmarshal(content, credentials); err != nil {
		log.Fatalln("parse config failed, err:", err)
	}
	log.Printf("loaded credentials for app %s", sdklog.SafeText(credentials.AppID))
	if err != nil {
		log.Fatalf("callback simulator setup failed: %s", sdklog.SafeError(err))
	}
	go simulateRequest(credentials)
	var ln string
	fmt.Scanln()
	_, _ = fmt.Sscanln("%v", ln)
	fmt.Println("callback simulator stopped")
}

func simulateRequest(credentials *token.QQBotCredentials) {
	// 等待 http 服务启动
	time.Sleep(3 * time.Second)
	var heartbeat = &dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{
			OPCode: dto.WSHeartbeat,
		},
		Data: 123,
	}
	payload, _ := json.Marshal(heartbeat)
	send(payload, credentials)

	var dispatchEvent = &dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{
			OPCode: dto.WSDispatchEvent,
			Seq:    1,
			Type:   dto.EventMessageReactionAdd,
		},
		Data: dto.WSMessageReactionData{
			UserID:    "123",
			ChannelID: "111",
			GuildID:   "222",
			Target: dto.ReactionTarget{
				ID:   "333",
				Type: dto.ReactionTargetTypeMsg,
			},
			Emoji: dto.Emoji{
				ID:   "42",
				Type: 1,
			},
		},
		RawMessage: nil,
	}
	payload, _ = json.Marshal(dispatchEvent)
	fmt.Printf("sending simulated callback event %s\n", dispatchEvent.Type)
	send(payload, credentials)
}

func send(payload []byte, credentials *token.QQBotCredentials) {
	header := http.Header{}
	header.Set(signature.HeaderTimestamp, strconv.FormatUint(uint64(time.Now().Unix()), 10))

	sig, err := signature.Generate(credentials.AppSecret, header, payload)
	if err != nil {
		fmt.Printf("sign callback request failed: %s\n", sdklog.SafeError(err))
		return
	}
	header.Set(signature.HeaderSig, sig)

	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		fmt.Printf("create callback request failed: %s\n", sdklog.SafeError(err))
		return
	}
	req.Header = header.Clone()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("send callback request failed: %s\n", sdklog.SafeError(err))
		return
	}

	defer resp.Body.Close()
	r, _ := io.ReadAll(resp.Body)
	fmt.Printf("callback request returned %s (%d response bytes)\n", resp.Status, len(r))
}
