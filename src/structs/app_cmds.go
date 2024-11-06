package structs

type AppCmdType = uint8

const (
	AppCmdTypeChatInput  AppCmdType = 1
	AppCmdTypeUser       AppCmdType = 2
	AppCmdTypeMessage    AppCmdType = 3
	AppPrimaryEntryPoint AppCmdType = 4
)

type AppCmdIntegrationType = uint8

const (
	AppIntegrationTypeGuildInstall AppCmdIntegrationType = 0
	AppIntegrationTypeUserInstall  AppCmdIntegrationType = 1
)

type AppCmdInteractionCtxType = uint8

const (
	AppInteractionContextTypeGuild          AppCmdInteractionCtxType = 0
	AppInteractionContextTypeBotDM          AppCmdInteractionCtxType = 1
	AppInteractionContextTypePrivateChannel AppCmdInteractionCtxType = 2
)

type AppCmd struct {
	ID                      string                     `json:"id,omitempty"`
	Type                    AppCmdType                 `json:"type,omitempty"`
	ApplicationID           string                     `json:"application_id,omitempty"`
	GuildId                 string                     `json:"guild_id,omitempty"`
	Name                    string                     `json:"name"`
	NameLocalization        interface{}                `json:"name_localization,omitempty"`
	Description             string                     `json:"description"`
	DescriptionLocalization string                     `json:"description_localizations,omitempty"`
	Options                 interface{}                `json:"options,omitempty"`
	IntegrationTypes        []AppCmdIntegrationType    `json:"integration_types"`
	Contexts                []AppCmdInteractionCtxType `json:"contexts"`
	Nsfw                    bool                       `json:"nsfw,omitempty"`
	Version                 string                     `json:"version,omitempty"`
}
