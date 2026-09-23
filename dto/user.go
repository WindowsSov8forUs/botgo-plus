package dto

// User 用户
type User struct {
	UserOpenID       string `json:"user_openid,omitempty"`
	MemberOpenID     string `json:"member_openid,omitempty"`
	MemberRole       string `json:"member_role,omitempty"`
	Scope            string `json:"scope,omitempty"`
	IsYou            bool   `json:"is_you,omitempty"`
	ID               string `json:"id"`
	Username         string `json:"username"`
	Avatar           string `json:"avatar"`
	Bot              bool   `json:"bot"`
	UnionOpenID      string `json:"union_openid"`       // 特殊关联应用的 openid
	UnionUserAccount string `json:"union_user_account"` // 机器人关联的用户信息，与union_openid关联的应用是同一个
}
