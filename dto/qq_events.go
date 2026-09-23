package dto

const (
	EventGroupMessageCreate EventType = "GROUP_MESSAGE_CREATE"
	EventGroupAddRobot      EventType = "GROUP_ADD_ROBOT"
	EventGroupDelRobot      EventType = "GROUP_DEL_ROBOT"
	EventGroupMsgReceive    EventType = "GROUP_MSG_RECEIVE"
	EventGroupMsgReject     EventType = "GROUP_MSG_REJECT"
	EventGroupMemberAdd     EventType = "GROUP_MEMBER_ADD"
	EventGroupMemberRemove  EventType = "GROUP_MEMBER_REMOVE"
	EventGroupJoinRequest   EventType = "GROUP_JOIN_REQUEST"
	EventGuildMemberDelete  EventType = "GUILD_MEMBER_DELETE"
	IntentGroupMembers      Intent    = 1 << 24
)

type WSGroupMessageData Message

func (m *WSGroupMessageData) UnmarshalJSON(data []byte) error {
	return (*Message)(m).UnmarshalJSON(data)
}

type GroupRobotEvent struct {
	GroupOpenID    string    `json:"group_openid"`
	GroupID        string    `json:"group_id,omitempty"`
	OperatorOpenID string    `json:"op_member_openid"`
	Timestamp      Timestamp `json:"timestamp"`
}

type GroupMemberEvent struct {
	GroupOpenID    string    `json:"group_openid"`
	GroupID        string    `json:"group_id,omitempty"`
	MemberOpenID   string    `json:"member_openid"`
	OperatorOpenID string    `json:"op_member_openid,omitempty"`
	Timestamp      Timestamp `json:"timestamp"`
}

type GroupJoinRequest struct {
	GroupOpenID   string `json:"group_openid"`
	JoinRequestID string `json:"join_request_id"`
	MemberOpenID  string `json:"member_openid"`
	UnionOpenID   string `json:"union_openid,omitempty"`
	Username      string `json:"username"`
	RiskTips      string `json:"risk_tips,omitempty"`
	ApplyAt       string `json:"apply_at"`
	ApplySource   string `json:"apply_source"`
	InvitedBy     string `json:"invited_by,omitempty"`
	Bot           bool   `json:"bot"`
}
