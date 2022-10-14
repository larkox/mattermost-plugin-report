package main

import (
	"fmt"
	"strings"
	"sync"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin
	client *pluginapi.Client

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	BotUserID string
}

func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	botID, err := p.client.Bot.EnsureBot(&model.Bot{
		Username:    "report",
		DisplayName: "Reporter Bot",
		Description: "Created by the Reporter plugin.",
	})
	if err != nil {
		return errors.Wrap(err, "failed to ensure todo bot")
	}
	p.BotUserID = botID

	return nil
}

const REPORT_POST_TYPE = "custom_report"

// See https://developers.mattermost.com/extend/plugins/server/reference/
func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	if post.Type == REPORT_POST_TYPE {
		p.client.Log.Debug("Not doing anything")
		// A report post. Avoid loops
		return
	}

	if len(p.configuration.Terms) == 0 {
		p.client.Log.Debug("Not doing anything")
		// Nothing to report about
		return
	}

	if len(p.configuration.Users) == 0 && len(p.configuration.Channels) == 0 {
		p.client.Log.Debug("Not doing anything")
		// Nobody to report to
		return
	}

	if len(p.configuration.Teams) == 0 {
		p.client.Log.Debug("Not doing anything")

		// Nowhere to report from
		return
	}

	channel, err := p.API.GetChannel(post.ChannelId)
	if err != nil {
		p.client.Log.Debug("Error getting channel")

		return
	}

	teamIds := splitStringList(p.configuration.Teams)

	found := false
	for _, teamId := range teamIds {
		if channel.TeamId == teamId {
			found = true
			break
		}
	}

	if !found {
		p.client.Log.Debug("Channel not in team")
		// Channel not in reporting team
		return
	}

	allText := strings.ToLower(extractTextFromPost(post))
	for _, term := range splitStringList(p.configuration.Terms) {
		if strings.Index(allText, strings.ToLower(term)) >= 0 {
			p.client.Log.Debug("Term found, reporting")

			p.report(post, channel, term)
			return
		}
	}

	// No term found
	p.client.Log.Debug("No term found")
}

func (p *Plugin) report(post *model.Post, channel *model.Channel, term string) {
	users := splitStringList(p.configuration.Users)
	channels := splitStringList(p.configuration.Channels)

	team, err := p.API.GetTeam(channel.TeamId)
	if err != nil {
		return
	}

	postToSend := &model.Post{
		UserId:  p.BotUserID,
		Message: fmt.Sprintf("@all Term `%s` has been used in channel `%s` in team `%s`. Link: %s", term, channel.DisplayName, team.DisplayName, p.getPermalink(post, team)),
		Type:    REPORT_POST_TYPE,
	}

	for _, channel := range channels {
		postToSend.ChannelId = channel
		_, _ = p.API.CreatePost(postToSend.Clone())
	}

	for _, userName := range users {
		user, err := p.API.GetUserByUsername(userName)
		if err != nil {
			continue
		}
		channel, err := p.API.GetDirectChannel(user.Id, p.BotUserID)
		if err != nil || channel == nil {
			continue
		}

		postToSend.ChannelId = channel.Id
		_, _ = p.API.CreatePost(postToSend.Clone())
	}
}

func (p *Plugin) getPermalink(post *model.Post, team *model.Team) string {
	config := p.API.GetConfig()
	siteURL := config.ServiceSettings.SiteURL
	if siteURL == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s/pl/%s", *config.ServiceSettings.SiteURL, team.Name, post.Id)
}

func splitStringList(received string) []string {
	splitted := strings.Split(received, ",")
	result := make([]string, 0, len(splitted))
	for _, v := range splitted {
		newV := strings.TrimSpace(v)
		if len(newV) > 0 {
			result = append(result, newV)
		}
	}

	return result
}

func extractTextFromPost(post *model.Post) string {
	allStrings := []string{
		post.Message,
	}

	if post.Metadata != nil && post.Metadata.Files != nil {
		for _, file := range post.Metadata.Files {
			allStrings = append(allStrings, file.Name)
		}
	}

	attachmentsProp := post.GetProp("attachments")

	if attachmentsProp != nil {
		attachments := attachmentsProp.([]*model.SlackAttachment)
		for _, attachment := range attachments {
			if attachment == nil {
				continue
			}
			allStrings = append(allStrings,
				attachment.Fallback,
				attachment.Pretext,
				attachment.AuthorName,
				attachment.Title,
				attachment.Text,
				attachment.Footer,
			)
			for _, field := range attachment.Fields {
				if field == nil {
					continue
				}
				allStrings = append(allStrings,
					field.Title,
					fmt.Sprintf("%v", field.Value),
				)
			}
			for _, action := range attachment.Actions {
				if action == nil {
					continue
				}
				allStrings = append(allStrings,
					action.Name,
					action.DefaultOption,
				)
				for _, option := range action.Options {
					if option == nil {
						continue
					}
					allStrings = append(allStrings,
						option.Text,
						option.Value,
					)
				}
			}
		}
	}

	return strings.Join(allStrings, "\n")
}
