package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr                         string
	ClawWorldNamespace                 string
	DatabaseURL                        string
	InternalSyncToken                  string
	ClawWorldAPIBase                   string
	PublicBaseURL                      string
	IdentitySigningKey                 string
	XOAuthClientID                     string
	XOAuthClientSecret                 string
	XOAuthAuthorizeURL                 string
	XOAuthTokenURL                     string
	XOAuthUserInfoURL                  string
	SocialRewardXAuth                  int64
	SocialRewardXMention               int64
	SocialRewardGitHubAuth             int64
	SocialRewardGitHubStar             int64
	SocialRewardGitHubFork             int64
	GitHubOAuthClientID                string
	GitHubOAuthClientSecret            string
	GitHubOAuthAuthorizeURL            string
	GitHubOAuthTokenURL                string
	GitHubOAuthUserInfoURL             string
	ColonyRepoURL                      string
	ColonyRepoBranch                   string
	ColonyRepoLocalPath                string
	ColonyRepoSync                     bool
	TianDaoLawKey                      string
	TianDaoLawVersion                  int64
	LifeCostPerTick                    int64
	ThinkCostRateMilli                 int64
	CommCostRateMilli                  int64
	ToolCostRateMilli                  int64
	ToolRuntimeExec                    bool
	ToolSandboxImage                   string
	ToolT3AllowHosts                   string
	ActionCostConsume                  bool
	DeathGraceTicks                    int
	InitialToken                       int64
	RegistrationGrantToken             int64
	TreasuryInitialToken               int64
	TickIntervalSeconds                int64
	ExtinctionThreshold                int
	MinPopulation                      int
	MetabolismInterval                 int
	MetabolismWeightE                  float64
	MetabolismWeightV                  float64
	MetabolismWeightA                  float64
	MetabolismWeightT                  float64
	MetabolismTopK                     int
	MetabolismMinValidators            int
	AutonomyReminderIntervalTicks      int64
	AutonomyReminderOffsetTicks        int64
	CommunityCommReminderIntervalTicks int64
	CommunityCommReminderOffsetTicks   int64
	KBEnrollmentReminderIntervalTicks  int64
	KBEnrollmentReminderOffsetTicks    int64
	KBVotingReminderIntervalTicks      int64
	KBVotingReminderOffsetTicks        int64
}

func FromEnv() Config {
	return Config{
		ListenAddr:                         getEnv("CLAWCOLONY_LISTEN_ADDR", ":8080"),
		ClawWorldNamespace:                 getEnv("CLAWCOLONY_NAMESPACE", "freewill"),
		DatabaseURL:                        getEnv("DATABASE_URL", ""),
		InternalSyncToken:                  getEnv("CLAWCOLONY_INTERNAL_SYNC_TOKEN", ""),
		ClawWorldAPIBase:                   getEnv("CLAWCOLONY_API_BASE_URL", "http://localhost:8080"),
		PublicBaseURL:                      getEnv("CLAWCOLONY_PUBLIC_BASE_URL", ""),
		IdentitySigningKey:                 getEnv("CLAWCOLONY_IDENTITY_SIGNING_KEY", ""),
		XOAuthClientID:                     getEnv("CLAWCOLONY_X_OAUTH_CLIENT_ID", ""),
		XOAuthClientSecret:                 getEnv("CLAWCOLONY_X_OAUTH_CLIENT_SECRET", ""),
		XOAuthAuthorizeURL:                 getEnv("CLAWCOLONY_X_OAUTH_AUTHORIZE_URL", ""),
		XOAuthTokenURL:                     getEnv("CLAWCOLONY_X_OAUTH_TOKEN_URL", ""),
		XOAuthUserInfoURL:                  getEnv("CLAWCOLONY_X_OAUTH_USERINFO_URL", ""),
		SocialRewardXAuth:                  getEnvInt64("CLAWCOLONY_SOCIAL_REWARD_X_AUTH", 10000),
		SocialRewardXMention:               getEnvInt64("CLAWCOLONY_SOCIAL_REWARD_X_MENTION", 10000),
		SocialRewardGitHubAuth:             getEnvInt64("CLAWCOLONY_SOCIAL_REWARD_GITHUB_AUTH", 10000),
		SocialRewardGitHubStar:             getEnvInt64("CLAWCOLONY_SOCIAL_REWARD_GITHUB_STAR", 10000),
		SocialRewardGitHubFork:             getEnvInt64("CLAWCOLONY_SOCIAL_REWARD_GITHUB_FORK", 10000),
		GitHubOAuthClientID:                getEnv("CLAWCOLONY_GITHUB_OAUTH_CLIENT_ID", ""),
		GitHubOAuthClientSecret:            getEnv("CLAWCOLONY_GITHUB_OAUTH_CLIENT_SECRET", ""),
		GitHubOAuthAuthorizeURL:            getEnv("CLAWCOLONY_GITHUB_OAUTH_AUTHORIZE_URL", ""),
		GitHubOAuthTokenURL:                getEnv("CLAWCOLONY_GITHUB_OAUTH_TOKEN_URL", ""),
		GitHubOAuthUserInfoURL:             getEnv("CLAWCOLONY_GITHUB_OAUTH_USERINFO_URL", ""),
		ColonyRepoURL:                      getEnv("COLONY_REPO_URL", ""),
		ColonyRepoBranch:                   getEnv("COLONY_REPO_BRANCH", "main"),
		ColonyRepoLocalPath:                getEnv("COLONY_REPO_LOCAL_PATH", "/tmp/clawcolony-civilization-repo"),
		ColonyRepoSync:                     getEnvBool("COLONY_REPO_SYNC_ENABLED", false),
		TianDaoLawKey:                      getEnv("TIAN_DAO_LAW_KEY", "genesis-v1"),
		TianDaoLawVersion:                  getEnvInt64("TIAN_DAO_LAW_VERSION", 1),
		LifeCostPerTick:                    getEnvInt64("LIFE_COST_PER_TICK", 1),
		ThinkCostRateMilli:                 getEnvInt64("THINK_COST_RATE_MILLI", 1000),
		CommCostRateMilli:                  getEnvInt64("COMM_COST_RATE_MILLI", 1000),
		ToolCostRateMilli:                  getEnvInt64("TOOL_COST_RATE_MILLI", 1000),
		ToolRuntimeExec:                    getEnvBool("TOOL_RUNTIME_EXEC_ENABLED", false),
		ToolSandboxImage:                   getEnv("TOOL_SANDBOX_IMAGE", "alpine:3.21"),
		ToolT3AllowHosts:                   getEnv("TOOL_T3_ALLOWED_HOSTS", ""),
		ActionCostConsume:                  getEnvBool("ACTION_COST_CONSUME_ENABLED", true),
		DeathGraceTicks:                    getEnvInt("DEATH_GRACE_TICKS", 5),
		InitialToken:                       getEnvInt64("INITIAL_TOKEN", 1000),
		RegistrationGrantToken:             getEnvInt64("REGISTRATION_GRANT_TOKEN", 10000),
		TreasuryInitialToken:               getEnvInt64("TREASURY_INITIAL_TOKEN", 1000000),
		TickIntervalSeconds:                getEnvInt64("TICK_INTERVAL_SECONDS", 60),
		ExtinctionThreshold:                getEnvInt("EXTINCTION_THRESHOLD_PCT", 30),
		MinPopulation:                      getEnvInt("MIN_POPULATION", 0),
		MetabolismInterval:                 getEnvInt("METABOLISM_INTERVAL_TICKS", 60),
		MetabolismWeightE:                  getEnvFloat64("METABOLISM_WEIGHT_E", 0.25),
		MetabolismWeightV:                  getEnvFloat64("METABOLISM_WEIGHT_V", 0.35),
		MetabolismWeightA:                  getEnvFloat64("METABOLISM_WEIGHT_A", 0.20),
		MetabolismWeightT:                  getEnvFloat64("METABOLISM_WEIGHT_T", 0.20),
		MetabolismTopK:                     getEnvInt("METABOLISM_CLUSTER_TOP_K", 100),
		MetabolismMinValidators:            getEnvInt("METABOLISM_SUPERSEDE_MIN_VALIDATORS", 2),
		AutonomyReminderIntervalTicks:      getEnvInt64("AUTONOMY_REMINDER_INTERVAL_TICKS", 0),
		AutonomyReminderOffsetTicks:        getEnvInt64("AUTONOMY_REMINDER_OFFSET_TICKS", 0),
		CommunityCommReminderIntervalTicks: getEnvInt64("COMMUNITY_COMM_REMINDER_INTERVAL_TICKS", 0),
		CommunityCommReminderOffsetTicks:   getEnvInt64("COMMUNITY_COMM_REMINDER_OFFSET_TICKS", 10),
		KBEnrollmentReminderIntervalTicks:  getEnvInt64("KB_ENROLLMENT_REMINDER_INTERVAL_TICKS", 0),
		KBEnrollmentReminderOffsetTicks:    getEnvInt64("KB_ENROLLMENT_REMINDER_OFFSET_TICKS", 2),
		KBVotingReminderIntervalTicks:      getEnvInt64("KB_VOTING_REMINDER_INTERVAL_TICKS", 0),
		KBVotingReminderOffsetTicks:        getEnvInt64("KB_VOTING_REMINDER_OFFSET_TICKS", 8),
	}
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func getEnvBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return fallback
	}
}

func getEnvInt64(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

func getEnvFloat64(key string, fallback float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fallback
	}
	return v
}
