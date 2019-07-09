package slack

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nlopes/slack"

	"github.com/alwinius/bow/constants"
	"github.com/alwinius/bow/extension/notification"
	"github.com/alwinius/bow/types"
	"github.com/alwinius/bow/version"

	log "github.com/sirupsen/logrus"
)

const timeout = 5 * time.Second

type sender struct {
	slackClient *slack.Client
	channels    []string
	botName     string
}

func init() {
	notification.RegisterSender("slack", &sender{})
}

func (s *sender) Configure(config *notification.Config) (bool, error) {
	var token string
	// Get configuration
	if os.Getenv(constants.EnvSlackToken) != "" {
		token = os.Getenv(constants.EnvSlackToken)
	} else {
		return false, nil
	}
	if os.Getenv(constants.EnvSlackBotName) != "" {
		s.botName = os.Getenv(constants.EnvSlackBotName)
	} else {
		s.botName = "bow"
	}

	if os.Getenv(constants.EnvSlackChannels) != "" {
		channels := os.Getenv(constants.EnvSlackChannels)
		s.channels = strings.Split(channels, ",")
	} else {
		s.channels = []string{"general"}
	}

	s.slackClient = slack.New(token)

	log.WithFields(log.Fields{
		"name":     "slack",
		"channels": s.channels,
	}).Info("extension.notification.slack: sender configured")

	var msg string
	if version.GetbowVersion().Version != "" {
		msg = fmt.Sprintf("bow has started. Version: '%s'. Revision: %s", version.GetbowVersion().Version, version.GetbowVersion().Revision)
	} else {
		msg = fmt.Sprintf("bow has started. Revision: %s", version.GetbowVersion().Revision)
	}

	err := s.Send(types.EventNotification{
		Message:   msg,
		CreatedAt: time.Now(),
		Type:      types.NotificationSystemEvent,
		Level:     types.LevelInfo,
		Channels:  s.channels,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"error":    err,
			"name":     "slack",
			"channels": s.channels,
		}).Error("extension.notification.slack: failed to set greeting message")
	}

	return true, nil
}

func (s *sender) Send(event types.EventNotification) error {
	params := slack.NewPostMessageParameters()
	params.Username = s.botName
	params.IconURL = constants.BowLogoURL

	attachements := []slack.Attachment{
		slack.Attachment{
			Fallback: event.Message,
			Color:    event.Level.Color(),
			Fields: []slack.AttachmentField{
				slack.AttachmentField{
					Title: event.Type.String(),
					Value: event.Message,
					Short: false,
				},
			},
			Footer: fmt.Sprintf("https://bow.sh %s", version.GetbowVersion().Version),
			Ts:     json.Number(strconv.Itoa(int(event.CreatedAt.Unix()))),
		},
	}

	chans := s.channels
	if len(event.Channels) > 0 {
		chans = event.Channels
	}

	var mgsOpts []slack.MsgOption

	mgsOpts = append(mgsOpts, slack.MsgOptionPostMessageParameters(params))
	mgsOpts = append(mgsOpts, slack.MsgOptionAttachments(attachements...))

	for _, channel := range chans {
		_, _, err := s.slackClient.PostMessage(channel, mgsOpts...)
		if err != nil {
			log.WithFields(log.Fields{
				"error":   err,
				"channel": channel,
			}).Error("extension.notification.slack: failed to send notification")
		}
	}
	return nil
}
