package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/openapi"
)

// Processor processes webhook events.
type Processor struct {
	api openapi.OpenAPI
}

func (p Processor) ProcessChannelMessage(input string, data *dto.ATMessageData) error {
	msg := generateDemoMessage(input, dto.Message(*data))
	if err := p.sendChannelReply(context.Background(), data.ChannelID, msg); err != nil {
		_ = p.sendChannelReply(context.Background(), data.ChannelID, genErrMessage(dto.Message(*data), err))
	}
	return nil
}

func (p Processor) ProcessInlineSearch(interaction *dto.InteractionEventData) error {
	if interaction == nil || interaction.Data == nil {
		return fmt.Errorf("empty interaction")
	}
	if interaction.Data.Type != dto.InteractionDataTypeChatSearch {
		return fmt.Errorf("interaction data type not chat search")
	}
	if interaction.Data.Resolved.ButtonData != "" && interaction.Data.Resolved.ButtonData != "test" {
		return fmt.Errorf("resolved search key not allowed")
	}

	searchRsp := &dto.SearchRsp{
		Layouts: []dto.SearchLayout{
			{
				LayoutType: dto.LayoutTypeImageText,
				ActionType: dto.ActionTypeSendARK,
				Title:      "内联搜索",
				Records: []dto.SearchRecord{
					{
						Cover: "https://pub.idqqimg.com/pc/misc/files/20211208/311cfc87ce394c62b7c9f0508658cf25.png",
						Title: "内联搜索标题",
						Tips:  "内联搜索 tips",
						URL:   "https://www.qq.com",
					},
				},
			},
		},
	}
	body, _ := json.Marshal(searchRsp)
	if err := p.api.PutInteraction(context.Background(), interaction.ID, string(body)); err != nil {
		log.Println("api call putInteractionInlineSearch error:", err)
		return err
	}
	return nil
}

func genErrMessage(data dto.Message, err error) *dto.MessageToCreate {
	return &dto.MessageToCreate{
		Timestamp: time.Now().UnixMilli(),
		Content:   fmt.Sprintf("处理异常:%v", err),
		MessageReference: &dto.MessageReference{
			MessageID:             data.ID,
			IgnoreGetMessageError: true,
		},
		MsgID: data.ID,
	}
}

func (p Processor) ProcessGroupMessage(input string, data *dto.GroupATMessageData) error {
	msg := generateDemoMessage(input, dto.Message(*data))
	rich := dto.RichMediaMessage{
		EventID:  data.ID,
		FileType: 1,
		URL:      "https://q.qlogo.cn/headimg_dl?dst_uin=1094950020&spec=640",
	}
	if err := p.sendGroupReply(context.Background(), data.GroupID, rich); err != nil {
		_ = p.sendGroupReply(context.Background(), data.GroupID, genErrMessage(dto.Message(*data), err))
	}
	if err := p.sendGroupReply(context.Background(), data.GroupID, msg); err != nil {
		_ = p.sendGroupReply(context.Background(), data.GroupID, genErrMessage(dto.Message(*data), err))
	}
	return nil
}

func (p Processor) ProcessC2CMessage(input string, data *dto.C2CMessageData) error {
	userID := ""
	if data.Author != nil && data.Author.ID != "" {
		userID = data.Author.ID
	}
	msg := generateDemoMessage(input, dto.Message(*data))
	if err := p.sendC2CReply(context.Background(), userID, msg); err != nil {
		_ = p.sendC2CReply(context.Background(), userID, genErrMessage(dto.Message(*data), err))
	}
	return nil
}

func generateDemoMessage(input string, data dto.Message) *dto.MessageToCreate {
	log.Printf("收到指令: %+v", input)
	msg := ""
	if len(input) > 0 {
		msg += "收到:" + input
	}
	for _, attachment := range data.Attachments {
		msg += ",收到文件类型:" + attachment.ContentType
	}
	return &dto.MessageToCreate{
		Timestamp: time.Now().UnixMilli(),
		Content:   msg,
		MessageReference: &dto.MessageReference{
			MessageID:             data.ID,
			IgnoreGetMessageError: true,
		},
		MsgID: data.ID,
	}
}
