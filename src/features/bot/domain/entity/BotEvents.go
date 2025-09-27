package entity

// BotEvents is the channel for getting bot events
type BotEvents <-chan BotEvent

// BotEvent is the sealed interface for all bot events
type BotEvent interface {
	isBotEvent()
}

// ActivateGroup event for activating a group
type ActivateGroup struct {
	GroupID  int64
	UserID   int64
	UserName string
}

func (ActivateGroup) isBotEvent() {}

// DeactivateGroup event for deactivating a group
type DeactivateGroup struct {
	GroupID  int64
	UserID   int64
	UserName string
}

func (DeactivateGroup) isBotEvent() {}

// GetServerLoad event for requesting server load information
type GetServerLoad struct {
	UserID   int64
	UserName string
}

func (GetServerLoad) isBotEvent() {}

// GetServerLoad event for requesting bot commands
type DirectGetBotCommands struct {
	UserID   int64
	UserName string
}

func (DirectGetBotCommands) isBotEvent() {}

// GroupGetBotCommands event for requesting bot commands in groups
type GroupGetBotCommands struct {
	GroupID  int64
	UserID   int64
	UserName string
}

func (GroupGetBotCommands) isBotEvent() {}

// GetAllGroups event for requesting all groups
type GetAllGroups struct {
	UserID   int64
	UserName string
}

func (GetAllGroups) isBotEvent() {}

// DeleteGroup event for deleting a group
type DeleteGroup struct {
	UserID   int64
	UserName string
	GroupID  int64
}

func (DeleteGroup) isBotEvent() {}

// StartBot event for starting bot
type StartBot struct {
	UserID   int64
	UserName string
}

func (StartBot) isBotEvent() {}

// GetResource event for requesting a resource
type GetResource struct {
	GroupID int64
	Link    string
}

func (GetResource) isBotEvent() {}

// ErrorDirect event for direct user errors
type ErrorDirect struct {
	UserID   int64
	UserName string
	Message  string
}

func (ErrorDirect) isBotEvent() {}

// ErrorGroup event for group errors
type ErrorGroup struct {
	GroupID int64
	Message string
}

func (ErrorGroup) isBotEvent() {}

// IgnoreCommand event for unrecognized commands
type IgnoreCommand struct {
	UserID   int64
	UserName string
	GroupID  int64 // 0 for direct messages
	Command  string
}

func (IgnoreCommand) isBotEvent() {}
