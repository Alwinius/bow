package constants

// DefaultDockerRegistry - default docker registry
const DefaultDockerRegistry = "https://index.docker.io"

// DefaultNamespace - default namespace to initialise configmaps based kv
const DefaultNamespace = "kube-system"

// WebhookEndpointEnv if set - enables webhook notifications
const WebhookEndpointEnv = "WEBHOOK_ENDPOINT"

// slack bot/token
const (
	EnvSlackToken            = "SLACK_TOKEN"
	EnvSlackBotName          = "SLACK_BOT_NAME"
	EnvSlackChannels         = "SLACK_CHANNELS"
	EnvSlackApprovalsChannel = "SLACK_APPROVALS_CHANNEL"

	EnvHipchatToken    = "HIPCHAT_TOKEN"
	EnvHipchatBotName  = "HIPCHAT_BOT_NAME"
	EnvHipchatChannels = "HIPCHAT_CHANNELS"

	EnvHipchatApprovalsChannel   = "HIPCHAT_APPROVALS_CHANNEL"
	EnvHipchatApprovalsUserName  = "HIPCHAT_APPROVALS_USER_NAME"
	EnvHipchatApprovalsBotName   = "HIPCHAT_APPROVALS_BOT_NAME"
	EnvHipchatApprovalsPasswort  = "HIPCHAT_APPROVALS_PASSWORT"
	EnvHipchatConnectionAttempts = "HIPCHAT_CONNECTION_ATTEMPTS"

	// Mattermost webhook endpoint, see https://docs.mattermost.com/developer/webhooks-incoming.html
	// for documentation on setting it up
	EnvMattermostEndpoint = "MATTERMOST_ENDPOINT"
	EnvMattermostName     = "MATTERMOST_USERNAME"
)

// EnvNotificationLevel - minimum level for notifications, defaults to info
const EnvNotificationLevel = "NOTIFICATION_LEVEL"

// Basic Auth - User / Password
const EnvBasicAuthUser = "BASIC_AUTH_USER"
const EnvBasicAuthPassword = "BASIC_AUTH_PASSWORD"
const EnvAuthenticatedWebhooks = "AUTHENTICATED_WEBHOOKS"
const EnvTokenSecret = "TOKEN_SECRET"

// BowLogoURL - is a logo URL for bot icon
const BowLogoURL = "https://bow.sh/images/logo.png"
