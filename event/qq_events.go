package event

import "github.com/WindowsSov8forUs/botgo-plus/dto"

type GroupMessageEventHandler func(*dto.WSPayload, *dto.WSGroupMessageData) error
type GroupRobotEventHandler func(*dto.WSPayload, *dto.GroupRobotEvent) error
type GroupMemberEventHandler func(*dto.WSPayload, *dto.GroupMemberEvent) error
type GroupJoinRequestEventHandler func(*dto.WSPayload, *dto.GroupJoinRequest) error

func init() {
	RegisterHandler(dto.WSDispatchEvent, dto.EventGroupMessageCreate, func(p *dto.WSPayload, raw []byte) error {
		var value dto.WSGroupMessageData
		if err := ParseData(raw, &value); err != nil {
			return err
		}
		if h := DefaultHandlers.GroupMessage; h != nil {
			return h(p, &value)
		}
		return plainFallback(p, raw)
	})
	for _, kind := range []dto.EventType{dto.EventGroupAddRobot, dto.EventGroupDelRobot, dto.EventGroupMsgReceive, dto.EventGroupMsgReject} {
		RegisterHandler(dto.WSDispatchEvent, kind, func(p *dto.WSPayload, raw []byte) error {
			var value dto.GroupRobotEvent
			if err := ParseData(raw, &value); err != nil {
				return err
			}
			if h := DefaultHandlers.GroupRobot; h != nil {
				return h(p, &value)
			}
			return plainFallback(p, raw)
		})
	}
	for _, kind := range []dto.EventType{dto.EventGroupMemberAdd, dto.EventGroupMemberRemove} {
		RegisterHandler(dto.WSDispatchEvent, kind, func(p *dto.WSPayload, raw []byte) error {
			var value dto.GroupMemberEvent
			if err := ParseData(raw, &value); err != nil {
				return err
			}
			if h := DefaultHandlers.GroupMember; h != nil {
				return h(p, &value)
			}
			return plainFallback(p, raw)
		})
	}
	RegisterHandler(dto.WSDispatchEvent, dto.EventGroupJoinRequest, func(p *dto.WSPayload, raw []byte) error {
		var value dto.GroupJoinRequest
		if err := ParseData(raw, &value); err != nil {
			return err
		}
		if h := DefaultHandlers.GroupJoinRequest; h != nil {
			return h(p, &value)
		}
		return plainFallback(p, raw)
	})
	RegisterHandler(dto.WSDispatchEvent, dto.EventGuildMemberDelete, guildMemberHandler)
}

func plainFallback(p *dto.WSPayload, raw []byte) error {
	if DefaultHandlers.Plain != nil {
		return DefaultHandlers.Plain(p, raw)
	}
	return nil
}
