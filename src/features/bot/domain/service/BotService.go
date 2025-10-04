package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"tg-downloader/env"
	"tg-downloader/env/accesslevel"
	"tg-downloader/src/core"
	"tg-downloader/src/features/bot/data/converter"
	"tg-downloader/src/features/bot/domain/entity"
	"tg-downloader/src/features/bot/domain/repository"
	systemEntity "tg-downloader/src/features/system/domain/entity"
	systemRepo "tg-downloader/src/features/system/domain/repository"
)

type BotService struct {
	botRepo     repository.IBotRepository
	cacheRepo   repository.IBotCacheRepository
	systemRepo  systemRepo.ISystemRepository
	environment env.TGDownloader
	converter   *converter.EnvCommandToCommandConverter
}

func NewBotService(botRepo repository.IBotRepository, cacheRepo repository.IBotCacheRepository, systemRepo systemRepo.ISystemRepository, environment env.TGDownloader) *BotService {
	return &BotService{
		botRepo:     botRepo,
		cacheRepo:   cacheRepo,
		systemRepo:  systemRepo,
		environment: environment,
		converter:   converter.NewEnvCommandToCommandConverter(),
	}
}

func (s *BotService) UpdateCommandsForUser(userID int64, userName string) error {
	isAdmin, err := s.botRepo.IsAdmin(userName)
	if err != nil {
		return err
	}

	commands := s.filterCommands(isAdmin, true)
	return s.botRepo.SetCommandsForDirectMessages(userID, commands)
}

func (s *BotService) UpdateCommandsForGroupUser(chatID int64, userID int64, userName string) error {
	isAdmin, err := s.botRepo.IsAdmin(userName)
	if err != nil {
		return err
	}

	commands := s.filterCommands(isAdmin, false)
	return s.botRepo.SetCommandsForChatMember(chatID, userID, commands)
}

func (s *BotService) filterCommands(isAdmin bool, isDirectMessage bool) []entity.Command {
	var filteredCommands []entity.Command
	codec := s.converter.Convert()

	for _, envCommand := range s.environment.CommandConfiguration.Commands {
		if !isAdmin && envCommand.AccessLevel == accesslevel.Admin {
			continue
		}

		if s.isCommandApplicableForContext(envCommand, isDirectMessage) {
			command := codec.Convert(envCommand)
			filteredCommands = append(filteredCommands, command)
		}
	}

	return filteredCommands
}

func (s *BotService) isCommandApplicableForContext(command env.Command, isDirectMessage bool) bool {
	if isDirectMessage {
		return s.isDirectMessageCommand(command)
	}
	return s.isGroupCommand(command)
}

func (s *BotService) isDirectMessageCommand(command env.Command) bool {
	directCommands := map[string]bool{
		core.GetBotCommandsKey: true,
		core.GetServerLoadKey:  true,
		core.GetAllGroupsKey:   true,
		core.DeleteGroupKey:    true,
	}

	for key, envCmd := range s.environment.CommandConfiguration.Commands {
		if envCmd.Command == command.Command && envCmd.Description == command.Description {
			return directCommands[key]
		}
	}
	return false
}

func (s *BotService) isGroupCommand(command env.Command) bool {
	groupCommands := map[string]bool{
		core.ActivateCommandKey:   true,
		core.DeactivateCommandKey: true,
		core.GetBotCommandsKey:    true,
		core.LoadResourceKey:      true,
	}

	for key, envCmd := range s.environment.CommandConfiguration.Commands {
		if envCmd.Command == command.Command && envCmd.Description == command.Description {
			return groupCommands[key]
		}
	}
	return false
}

func (s *BotService) GetBotEvents() entity.BotEvents {
	return s.botRepo.ReceiveEvents()
}

func (s *BotService) ActivateGroup(groupID int64, userID int64, userName string) error {
	// Check if user is admin
	isAdmin, err := s.botRepo.IsAdmin(userName)
	if err != nil {
		return s.sendGroupMessage(groupID, "❌ Error checking admin status")
	}

	if !isAdmin {
		return s.sendGroupMessage(groupID, "❌ Only admins can activate groups")
	}

	// Convert groupID to string for cache operations
	groupIDStr := strconv.FormatInt(groupID, 10)

	// Check if group doesn't exist (GetGroup should return error if not found)
	_, err = s.cacheRepo.GetGroup(groupIDStr)
	if err == nil {
		// Group already exists
		return s.sendGroupMessage(groupID, "⚠️ Group already activated")
	}

	// Create and save new group
	group := &entity.Group{
		GroupID:       groupIDStr,
		AdminUserName: userName,
	}

	err = s.cacheRepo.WriteGroup(group)
	if err != nil {
		return s.sendGroupMessage(groupID, "❌ Error activating group")
	}

	return s.sendGroupMessage(groupID, "✅ Group activated for downloading")
}

func (s *BotService) sendGroupMessage(groupID int64, message string) error {
	return s.botRepo.SendGroupMessage(groupID, message)
}

func (s *BotService) DeactivateGroup(groupID int64, userID int64, userName string) error {
	// Check if user is admin
	isAdmin, err := s.botRepo.IsAdmin(userName)
	if err != nil {
		return s.sendGroupMessage(groupID, "❌ Error checking admin status")
	}

	if !isAdmin {
		return s.sendGroupMessage(groupID, "❌ Only admins can deactivate groups")
	}

	// Convert groupID to string for cache operations
	groupIDStr := strconv.FormatInt(groupID, 10)

	// Check if group exists
	_, err = s.cacheRepo.GetGroup(groupIDStr)
	if err != nil {
		// Group doesn't exist
		return s.sendGroupMessage(groupID, "⚠️ Group is not activated")
	}

	// Delete group from cache
	err = s.cacheRepo.DeleteGroup(groupIDStr)
	if err != nil {
		return s.sendGroupMessage(groupID, "❌ Error deactivating group")
	}

	return s.sendGroupMessage(groupID, "✅ Group deactivated")
}

func (s *BotService) DeleteGroup(groupID int64, userID int64, userName string) error {
	// Check if user is admin
	isAdmin, err := s.botRepo.IsAdmin(userName)
	if err != nil {
		return s.sendDirectMessage(userID, "❌ Error checking admin status")
	}

	if !isAdmin {
		return s.sendDirectMessage(userID, "❌ Only admins can delete groups")
	}

	// Convert groupID to string for cache operations
	groupIDStr := strconv.FormatInt(groupID, 10)

	// Check if group exists
	_, err = s.cacheRepo.GetGroup(groupIDStr)
	if err != nil {
		// Group doesn't exist
		return s.sendDirectMessage(userID, fmt.Sprintf("⚠️ Group %d is not found", groupID))
	}

	// Delete group from cache
	err = s.cacheRepo.DeleteGroup(groupIDStr)
	if err != nil {
		return s.sendDirectMessage(userID, fmt.Sprintf("❌ Error deleting group %d", groupID))
	}

	return s.sendDirectMessage(userID, fmt.Sprintf("✅ Group %d deleted successfully", groupID))
}

func (s *BotService) GetAllGroups(userID int64, userName string) error {
	// Check if user is admin
	isAdmin, err := s.botRepo.IsAdmin(userName)
	if err != nil {
		return s.sendDirectMessage(userID, "❌ Error checking admin status")
	}

	if !isAdmin {
		return s.sendDirectMessage(userID, "❌ Only admins can view all groups")
	}

	// Get all groups for this admin
	groups, err := s.cacheRepo.GetAllGroupsByUserName(userName)
	if err != nil {
		return s.sendDirectMessage(userID, "❌ Error retrieving groups")
	}

	if len(groups) == 0 {
		return s.sendDirectMessage(userID, "📝 No groups found")
	}

	// Fetch chat info in parallel and format message
	results := s.fetchGroupInfoInParallel(groups)
	message := s.formatGroupsMessage(results)

	return s.sendDirectMessage(userID, message)
}

type groupResult struct {
	index    int
	group    *entity.Group
	chatInfo *entity.ChatInfo
	err      error
}

func (s *BotService) fetchGroupInfoInParallel(groups []*entity.Group) []groupResult {
	resultChan := make(chan groupResult, len(groups))

	// Launch goroutines for parallel chat info requests
	for i, group := range groups {
		go s.fetchSingleGroupInfo(i, group, resultChan)
	}

	// Collect results
	results := make([]groupResult, len(groups))
	for i := 0; i < len(groups); i++ {
		result := <-resultChan
		results[result.index] = result
	}

	return results
}

func (s *BotService) fetchSingleGroupInfo(index int, group *entity.Group, resultChan chan<- groupResult) {
	groupID, parseErr := strconv.ParseInt(group.GroupID, 10, 64)
	if parseErr != nil {
		resultChan <- groupResult{index, group, nil, parseErr}
		return
	}

	chatInfo, chatErr := s.botRepo.GetChatInfo(groupID)
	resultChan <- groupResult{index, group, chatInfo, chatErr}
}

func (s *BotService) formatGroupsMessage(results []groupResult) string {
	message := fmt.Sprintf("📋 ACTIVE GROUPS (%d):\n\n", len(results))

	for _, result := range results {
		if result.err != nil || result.chatInfo == nil {
			message += s.formatGroupEntryWithError(result)
		} else {
			message += s.formatGroupEntryWithInfo(result)
		}
	}

	return message
}

func (s *BotService) formatGroupEntryWithError(result groupResult) string {
	return fmt.Sprintf("%d. Group ID: %s (Admin: %s) - ⚠️ Info unavailable\n",
		result.index+1, result.group.GroupID, result.group.AdminUserName)
}

func (s *BotService) formatGroupEntryWithInfo(result groupResult) string {
	return fmt.Sprintf("%d. %s\n   ID: %s\n   Type: %s\n   Admin: %s\n\n",
		result.index+1, strings.ToUpper(result.chatInfo.Title), result.group.GroupID,
		result.chatInfo.Type, result.group.AdminUserName)
}

func (s *BotService) HandleDirectError(userID int64, userName string, message string) error {
	return s.sendDirectMessage(userID, message)
}

func (s *BotService) HandleGroupError(groupID int64, message string) error {
	return s.sendGroupMessage(groupID, message)
}

func (s *BotService) GetServerLoad(userID int64, userName string) error {
	// Check if user is admin
	isAdmin, err := s.botRepo.IsAdmin(userName)
	if err != nil {
		return s.sendDirectMessage(userID, "❌ Error checking admin status")
	}

	if !isAdmin {
		return s.sendDirectMessage(userID, "❌ Only admins can view server load")
	}

	// Get system information
	systemInfo, err := s.systemRepo.GetSystemInfo()
	if err != nil {
		return s.sendDirectMessage(userID, "❌ Error retrieving system information")
	}

	// Format and send system information
	message := s.formatSystemInfo(systemInfo)
	return s.sendDirectMessage(userID, message)
}

func (s *BotService) formatSystemInfo(info *systemEntity.SystemInfo) string {
	var builder strings.Builder

	builder.WriteString("🖥️ SERVER LOAD INFORMATION\n\n")

	// Host Information
	if info.Host != nil {
		builder.WriteString("🖥️ HOST INFORMATION:\n")

		hostInfo := info.Host

		if hostInfo.Hostname != nil {
			builder.WriteString(fmt.Sprintf("Hostname: %s\n", *hostInfo.Hostname))
		}
		if hostInfo.OS != nil && hostInfo.PlatformVersion != nil {
			builder.WriteString(fmt.Sprintf("OS: %s %s\n", *hostInfo.OS, *hostInfo.PlatformVersion))
		} else if hostInfo.OS != nil {
			builder.WriteString(fmt.Sprintf("OS: %s\n", *hostInfo.OS))
		}
		if hostInfo.Platform != nil && hostInfo.KernelArch != nil {
			builder.WriteString(fmt.Sprintf("Platform: %s (%s)\n", *hostInfo.Platform, *hostInfo.KernelArch))
		} else if hostInfo.Platform != nil {
			builder.WriteString(fmt.Sprintf("Platform: %s\n", *hostInfo.Platform))
		}
		if hostInfo.UptimeFormatted != nil {
			builder.WriteString(fmt.Sprintf("Uptime: %s\n", *hostInfo.UptimeFormatted))
		}
		builder.WriteString("\n")
	}

	// CPU Information
	if info.CPU != nil {
		builder.WriteString("⚡ CPU INFORMATION:\n")

		cpuInfo := info.CPU

		if cpuInfo.ModelName != nil {
			builder.WriteString(fmt.Sprintf("Model: %s\n", *cpuInfo.ModelName))
		}
		if cpuInfo.PhysicalCores != nil && cpuInfo.LogicalCores != nil {
			builder.WriteString(fmt.Sprintf("Cores: %d physical, %d logical\n", *cpuInfo.PhysicalCores, *cpuInfo.LogicalCores))
		} else if cpuInfo.PhysicalCores != nil {
			builder.WriteString(fmt.Sprintf("Physical Cores: %d\n", *cpuInfo.PhysicalCores))
		} else if cpuInfo.LogicalCores != nil {
			builder.WriteString(fmt.Sprintf("Logical Cores: %d\n", *cpuInfo.LogicalCores))
		}
		if cpuInfo.UsagePercent != nil {
			builder.WriteString(fmt.Sprintf("Usage: %.1f%%\n", *cpuInfo.UsagePercent))
		}
		if cpuInfo.Speed != nil && *cpuInfo.Speed > 0 {
			builder.WriteString(fmt.Sprintf("Speed: %.0f MHz\n", *cpuInfo.Speed))
		}
		builder.WriteString("\n")
	}

	// Memory Information
	if info.Memory != nil {
		builder.WriteString("💾 MEMORY INFORMATION:\n")

		memory := info.Memory

		if memory.TotalFormatted != nil {
			builder.WriteString(fmt.Sprintf("Total: %s\n", *memory.TotalFormatted))
		}
		if memory.UsedFormatted != nil && memory.UsedPercent != nil {
			builder.WriteString(fmt.Sprintf("Used: %s (%.1f%%)\n", *memory.UsedFormatted, *memory.UsedPercent))
		} else if memory.UsedFormatted != nil {
			builder.WriteString(fmt.Sprintf("Used: %s\n", *memory.UsedFormatted))
		}
		if memory.AvailableFormatted != nil {
			builder.WriteString(fmt.Sprintf("Available: %s\n", *memory.AvailableFormatted))
		}
		if memory.SwapTotal != nil && *memory.SwapTotal > 0 {
			if memory.SwapUsedFormatted != nil && memory.SwapTotalFormatted != nil && memory.SwapPercent != nil {
				builder.WriteString(fmt.Sprintf("Swap: %s / %s (%.1f%%)\n",
					*memory.SwapUsedFormatted, *memory.SwapTotalFormatted, *memory.SwapPercent))
			}
		}
		builder.WriteString("\n")
	}

	// Show any collection errors
	if len(info.Errors) > 0 {
		builder.WriteString("⚠️ COLLECTION WARNINGS:\n")
		for _, err := range info.Errors {
			builder.WriteString(fmt.Sprintf("%s\n", err))
		}
		builder.WriteString("\n")
	}

	builder.WriteString(fmt.Sprintf("🕐 Collected at: %s", info.Timestamp.Format("2006-01-02 15:04:05")))

	return builder.String()
}

func (s *BotService) GetDirectCommands(userID int64, userName string) error {
	// Check if user is admin to determine available commands
	isAdmin, err := s.botRepo.IsAdmin(userName)
	if err != nil {
		return s.sendDirectMessage(userID, "❌ Error checking admin status")
	}

	// Get filtered commands for direct messages
	commands := s.filterCommands(isAdmin, true)

	// Format and send command list
	message := s.formatCommandList(commands, "Direct Message", isAdmin)
	return s.sendDirectMessage(userID, message)
}

func (s *BotService) GetGroupCommands(groupID int64, userID int64, userName string) error {
	// Check if user is admin to determine available commands
	isAdmin, err := s.botRepo.IsAdmin(userName)
	if err != nil {
		return s.sendGroupMessage(groupID, "❌ Error checking admin status")
	}

	// Get filtered commands for group messages
	commands := s.filterCommands(isAdmin, false)

	// Format and send command list
	message := s.formatCommandList(commands, "Group", isAdmin)
	return s.sendGroupMessage(groupID, message)
}

func (s *BotService) formatCommandList(commands []entity.Command, context string, isAdmin bool) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("🤖 AVAILABLE %s COMMANDS:\n\n", strings.ToUpper(context)))

	if len(commands) == 0 {
		builder.WriteString("No commands available for your access level.\n")
		return builder.String()
	}

	// Sort commands by command string for consistent display
	for _, cmd := range commands {
		builder.WriteString(fmt.Sprintf("%s - %s\n", cmd.Command, cmd.Description))
	}

	builder.WriteString("\n")

	if isAdmin {
		builder.WriteString("👑 You have administrator privileges\n")
	} else {
		builder.WriteString("👤 You have user privileges\n")
	}

	builder.WriteString("ℹ️ Use any command to see it in action!")

	return builder.String()
}

func (s *BotService) LoadResource(groupID int64, link string) (bool, error) {
	// Check if group is activated first
	groupIDStr := strconv.FormatInt(groupID, 10)
	_, err := s.cacheRepo.GetGroup(groupIDStr)
	if err != nil {
		// Group is not activated
		err := s.sendGroupMessage(groupID, "❌ Group is not activated. Use /a to activate the group first.")
		return false, err
	}

	// Validate URL format
	urlRegex := regexp.MustCompile(core.URLRegexPattern)
	if !urlRegex.MatchString(link) {
		err := s.sendGroupMessage(groupID, "❌ Invalid URL format")
		return false, err
	}

	// Check against supported patterns
	var isSupported bool
	for _, linkPattern := range s.environment.CommandConfiguration.SupportedLinks {
		matched, err := regexp.MatchString(linkPattern.Pattern, link)
		if err != nil {
			continue
		}
		if matched {
			isSupported = true
			break
		}
	}

	if !isSupported {
		// Build error message with supported formats
		var supportedFormats string
		for i, linkPattern := range s.environment.CommandConfiguration.SupportedLinks {
			if i > 0 {
				supportedFormats += "\n"
			}
			supportedFormats += fmt.Sprintf("• %s: %s", linkPattern.Name, linkPattern.Example)
		}

		err := s.sendGroupMessage(groupID, fmt.Sprintf("❌ Unsupported video format. Supported formats:\n%s", supportedFormats))
		return false, err
	}

	// Send confirmation that processing started
	err = s.sendGroupMessage(groupID, "🔄 Processing video... Please wait.")
	return true, err
}

func (s *BotService) HandleVideoProcessSuccess(groupID int64, fileName string) error {
	message := fmt.Sprintf("✅ Video processed successfully: %s", fileName)
	return s.sendGroupMessage(groupID, message)
}

func (s *BotService) HandleVideoProcessFailure(groupID int64, errorMessage string) error {
	message := fmt.Sprintf("❌ Video processing failed: %s", errorMessage)
	return s.sendGroupMessage(groupID, message)
}

func (s *BotService) sendDirectMessage(userID int64, message string) error {
	return s.botRepo.SendDirectMessage(userID, message)
}
