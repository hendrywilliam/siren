package structs

type User struct {
	ID                   string      `json:"id"`
	Username             string      `json:"username"`
	PublicFlags          uint8       `json:"public_flags"`
	Discriminator        string      `json:"discriminator"`
	Avatar               string      `json:"avatar"`
	Clan                 interface{} `json:"clan,omitempty"`
	GlobalName           string      `json:"global_name,omitempty"`
	AvatarDecorationData interface{} `json:"avatar_decoration_data,omitempty"`
}
