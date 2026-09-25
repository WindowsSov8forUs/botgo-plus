package main

import (
	"context"
	"log"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	sdklog "github.com/WindowsSov8forUs/botgo-plus/log"
)

func (p Processor) setEmoji(ctx context.Context, channelID string, messageID string) {
	err := p.api.CreateMessageReaction(
		ctx, channelID, messageID, dto.Emoji{
			ID:   "307",
			Type: 1,
		},
	)
	if err != nil {
		log.Printf("add reaction to message %s in channel %s failed: %s", sdklog.SafeText(messageID), sdklog.SafeText(channelID), sdklog.SafeError(err))
	}
}

func (p Processor) setPins(ctx context.Context, channelID, msgID string) {
	_, err := p.api.AddPins(ctx, channelID, msgID)
	if err != nil {
		log.Printf("pin message %s in channel %s failed: %s", sdklog.SafeText(msgID), sdklog.SafeText(channelID), sdklog.SafeError(err))
	}
}

func (p Processor) setAnnounces(ctx context.Context, data *dto.WSATMessageData) {
	if _, err := p.api.CreateChannelAnnounces(
		ctx, data.ChannelID,
		&dto.ChannelAnnouncesToCreate{MessageID: data.ID},
	); err != nil {
		log.Printf("create announcement in channel %s failed: %s", sdklog.SafeText(data.ChannelID), sdklog.SafeError(err))
	}
}

func (p Processor) sendChannelReply(ctx context.Context, channelID string, toCreate *dto.MessageToCreate) error {
	if _, err := p.api.PostMessage(ctx, channelID, toCreate); err != nil {
		log.Printf("send reply to channel %s failed: %s", sdklog.SafeText(channelID), sdklog.SafeError(err))
		return err
	}
	return nil
}

func (p Processor) sendGroupReply(ctx context.Context, groupID string, toCreate dto.APIMessage) error {
	log.Printf("sending reply to group %s", sdklog.SafeText(groupID))
	if _, err := p.api.PostGroupMessage(ctx, groupID, toCreate); err != nil {
		log.Printf("send reply to group %s failed: %s", sdklog.SafeText(groupID), sdklog.SafeError(err))
		return err
	}
	return nil
}

func (p Processor) sendC2CReply(ctx context.Context, userID string, toCreate dto.APIMessage) error {
	log.Printf("sending reply to user %s", sdklog.SafeText(userID))
	if _, err := p.api.PostC2CMessage(ctx, userID, toCreate); err != nil {
		log.Printf("send reply to user %s failed: %s", sdklog.SafeText(userID), sdklog.SafeError(err))
		return err
	}
	return nil
}
