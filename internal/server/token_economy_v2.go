package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"clawcolony/internal/economy"
	"clawcolony/internal/store"
)

const (
	ownerEconomyStateKey        = "token_economy_v2_owner_profiles"
	commQuotaStateKey           = "token_economy_v2_comm_quota"
	rewardDecisionStateKey      = "token_economy_v2_reward_decisions"
	rewardQueueStateKey         = "token_economy_v2_reward_queue"
	contributionEventStateKey   = "token_economy_v2_contribution_events"
	knowledgeMetaStateKey       = "token_economy_v2_knowledge_meta"
	toolEconomyStateKey         = "token_economy_v2_tool_meta"
	dashboardEconomySnapshotKey = "token_economy_v2_dashboard_snapshot"
)

type ownerEconomyProfile struct {
	OwnerID           string     `json:"owner_id"`
	GitHubUserID      string     `json:"github_user_id,omitempty"`
	GitHubUsername    string     `json:"github_username,omitempty"`
	Activated         bool       `json:"activated"`
	ActivatedAt       *time.Time `json:"activated_at,omitempty"`
	GitHubBindGranted bool       `json:"github_bind_granted,omitempty"`
	GitHubStarGranted bool       `json:"github_star_granted,omitempty"`
	GitHubForkGranted bool       `json:"github_fork_granted,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type ownerEconomyState struct {
	Profiles map[string]ownerEconomyProfile `json:"profiles"`
}

type commQuotaWindow struct {
	UserID          string    `json:"user_id"`
	WindowStartTick int64     `json:"window_start_tick"`
	UsedFreeTokens  int64     `json:"used_free_tokens"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type commQuotaState struct {
	Users map[string]commQuotaWindow `json:"users"`
}

type economyRewardDecision struct {
	DecisionKey     string         `json:"decision_key"`
	RuleKey         string         `json:"rule_key"`
	ResourceType    string         `json:"resource_type"`
	ResourceID      string         `json:"resource_id"`
	RecipientUserID string         `json:"recipient_user_id"`
	Amount          int64          `json:"amount"`
	Priority        int            `json:"priority"`
	Status          string         `json:"status"`
	QueueReason     string         `json:"queue_reason,omitempty"`
	LedgerID        int64          `json:"ledger_id,omitempty"`
	BalanceAfter    int64          `json:"balance_after,omitempty"`
	Meta            map[string]any `json:"meta,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	AppliedAt       *time.Time     `json:"applied_at,omitempty"`
	EnqueuedAt      *time.Time     `json:"enqueued_at,omitempty"`
}

type rewardDecisionState struct {
	Items map[string]economyRewardDecision `json:"items"`
}

type rewardQueueState struct {
	Items []economyRewardDecision `json:"items"`
}

type contributionEvent struct {
	EventKey     string         `json:"event_key"`
	Kind         string         `json:"kind"`
	UserID       string         `json:"user_id"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Meta         map[string]any `json:"meta,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	ProcessedAt  *time.Time     `json:"processed_at,omitempty"`
	DecisionKeys []string       `json:"decision_keys,omitempty"`
}

type contributionEventState struct {
	Items map[string]contributionEvent `json:"items"`
}

type citationRef struct {
	RefType string `json:"ref_type"`
	RefID   string `json:"ref_id"`
}

type knowledgeMeta struct {
	ProposalID    int64         `json:"proposal_id,omitempty"`
	EntryID       int64         `json:"entry_id,omitempty"`
	Category      string        `json:"category"`
	References    []citationRef `json:"references,omitempty"`
	AuthorUserID  string        `json:"author_user_id"`
	ContentTokens int64         `json:"content_tokens"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type knowledgeMetaState struct {
	ByProposal map[string]knowledgeMeta `json:"by_proposal"`
	ByEntry    map[string]knowledgeMeta `json:"by_entry"`
}

type toolEconomyMeta struct {
	ToolID               string    `json:"tool_id"`
	AuthorUserID         string    `json:"author_user_id"`
	CategoryHint         string    `json:"category_hint,omitempty"`
	FunctionalClusterKey string    `json:"functional_cluster_key,omitempty"`
	PriceToken           int64     `json:"price_token,omitempty"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type toolEconomyState struct {
	Items map[string]toolEconomyMeta `json:"items"`
}

type commChargePreview struct {
	UserID          string
	Activated       bool
	Tokens          int64
	WindowStartTick int64
	FreeCovered     int64
	OverageTokens   int64
	ChargedAmount   int64
}

type economyDashboardSnapshot struct {
	PoolBalance        int64          `json:"pool_balance"`
	SafeBalance        int64          `json:"safe_balance"`
	RewardQueueDepth   int            `json:"reward_queue_depth"`
	RewardQueueAmounts map[int]int64  `json:"reward_queue_amounts"`
	PopulationByState  map[string]int `json:"population_by_state"`
	Scarcity           map[string]any `json:"scarcity,omitempty"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

var toolManifestPricePattern = regexp.MustCompile(`(?i)(?:price|token_price)\s*["':= ]+\s*([0-9]+)`)

func (s *Server) tokenPolicy() economy.Policy {
	return economy.PolicyFromConfig(s.cfg)
}

func (s *Server) tokenEconomyV2Enabled() bool {
	return s.tokenPolicy().Enabled()
}

func (s *Server) initTokenEconomyV2(ctx context.Context) error {
	if !s.tokenEconomyV2Enabled() {
		return nil
	}
	return s.backfillOwnerEconomyProfiles(ctx)
}

func (s *Server) getOwnerEconomyState(ctx context.Context) (ownerEconomyState, error) {
	state := ownerEconomyState{Profiles: map[string]ownerEconomyProfile{}}
	_, _, err := s.getSettingJSON(ctx, ownerEconomyStateKey, &state)
	if err != nil {
		return ownerEconomyState{}, err
	}
	if state.Profiles == nil {
		state.Profiles = map[string]ownerEconomyProfile{}
	}
	return state, nil
}

func (s *Server) saveOwnerEconomyState(ctx context.Context, state ownerEconomyState) error {
	if state.Profiles == nil {
		state.Profiles = map[string]ownerEconomyProfile{}
	}
	_, err := s.putSettingJSON(ctx, ownerEconomyStateKey, state)
	return err
}

func (s *Server) getCommQuotaState(ctx context.Context) (commQuotaState, error) {
	state := commQuotaState{Users: map[string]commQuotaWindow{}}
	_, _, err := s.getSettingJSON(ctx, commQuotaStateKey, &state)
	if err != nil {
		return commQuotaState{}, err
	}
	if state.Users == nil {
		state.Users = map[string]commQuotaWindow{}
	}
	return state, nil
}

func (s *Server) saveCommQuotaState(ctx context.Context, state commQuotaState) error {
	if state.Users == nil {
		state.Users = map[string]commQuotaWindow{}
	}
	_, err := s.putSettingJSON(ctx, commQuotaStateKey, state)
	return err
}

func (s *Server) getRewardDecisionState(ctx context.Context) (rewardDecisionState, error) {
	state := rewardDecisionState{Items: map[string]economyRewardDecision{}}
	_, _, err := s.getSettingJSON(ctx, rewardDecisionStateKey, &state)
	if err != nil {
		return rewardDecisionState{}, err
	}
	if state.Items == nil {
		state.Items = map[string]economyRewardDecision{}
	}
	return state, nil
}

func (s *Server) saveRewardDecisionState(ctx context.Context, state rewardDecisionState) error {
	if state.Items == nil {
		state.Items = map[string]economyRewardDecision{}
	}
	_, err := s.putSettingJSON(ctx, rewardDecisionStateKey, state)
	return err
}

func (s *Server) getRewardQueueState(ctx context.Context) (rewardQueueState, error) {
	state := rewardQueueState{Items: []economyRewardDecision{}}
	_, _, err := s.getSettingJSON(ctx, rewardQueueStateKey, &state)
	if err != nil {
		return rewardQueueState{}, err
	}
	if state.Items == nil {
		state.Items = []economyRewardDecision{}
	}
	return state, nil
}

func (s *Server) saveRewardQueueState(ctx context.Context, state rewardQueueState) error {
	if state.Items == nil {
		state.Items = []economyRewardDecision{}
	}
	_, err := s.putSettingJSON(ctx, rewardQueueStateKey, state)
	return err
}

func (s *Server) getContributionEventState(ctx context.Context) (contributionEventState, error) {
	state := contributionEventState{Items: map[string]contributionEvent{}}
	_, _, err := s.getSettingJSON(ctx, contributionEventStateKey, &state)
	if err != nil {
		return contributionEventState{}, err
	}
	if state.Items == nil {
		state.Items = map[string]contributionEvent{}
	}
	return state, nil
}

func (s *Server) saveContributionEventState(ctx context.Context, state contributionEventState) error {
	if state.Items == nil {
		state.Items = map[string]contributionEvent{}
	}
	_, err := s.putSettingJSON(ctx, contributionEventStateKey, state)
	return err
}

func (s *Server) getKnowledgeMetaState(ctx context.Context) (knowledgeMetaState, error) {
	state := knowledgeMetaState{
		ByProposal: map[string]knowledgeMeta{},
		ByEntry:    map[string]knowledgeMeta{},
	}
	_, _, err := s.getSettingJSON(ctx, knowledgeMetaStateKey, &state)
	if err != nil {
		return knowledgeMetaState{}, err
	}
	if state.ByProposal == nil {
		state.ByProposal = map[string]knowledgeMeta{}
	}
	if state.ByEntry == nil {
		state.ByEntry = map[string]knowledgeMeta{}
	}
	return state, nil
}

func (s *Server) saveKnowledgeMetaState(ctx context.Context, state knowledgeMetaState) error {
	if state.ByProposal == nil {
		state.ByProposal = map[string]knowledgeMeta{}
	}
	if state.ByEntry == nil {
		state.ByEntry = map[string]knowledgeMeta{}
	}
	_, err := s.putSettingJSON(ctx, knowledgeMetaStateKey, state)
	return err
}

func (s *Server) getToolEconomyState(ctx context.Context) (toolEconomyState, error) {
	state := toolEconomyState{Items: map[string]toolEconomyMeta{}}
	_, _, err := s.getSettingJSON(ctx, toolEconomyStateKey, &state)
	if err != nil {
		return toolEconomyState{}, err
	}
	if state.Items == nil {
		state.Items = map[string]toolEconomyMeta{}
	}
	return state, nil
}

func (s *Server) saveToolEconomyState(ctx context.Context, state toolEconomyState) error {
	if state.Items == nil {
		state.Items = map[string]toolEconomyMeta{}
	}
	_, err := s.putSettingJSON(ctx, toolEconomyStateKey, state)
	return err
}

func (s *Server) currentTickID() int64 {
	s.worldTickMu.Lock()
	defer s.worldTickMu.Unlock()
	if s.worldTickID <= 0 {
		return 1
	}
	return s.worldTickID
}

func (s *Server) backfillOwnerEconomyProfiles(ctx context.Context) error {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()

	state, err := s.getOwnerEconomyState(ctx)
	if err != nil {
		return err
	}
	bots, err := s.store.ListBots(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	changed := false
	for _, b := range bots {
		userID := strings.TrimSpace(b.BotID)
		if userID == "" || isExcludedTokenUserID(userID) {
			continue
		}
		binding, err := s.store.GetAgentHumanBinding(ctx, userID)
		if err != nil {
			continue
		}
		owner, err := s.store.GetHumanOwner(ctx, binding.OwnerID)
		if err != nil {
			continue
		}
		profile := state.Profiles[owner.OwnerID]
		if profile.OwnerID == "" {
			profile.OwnerID = owner.OwnerID
			profile.CreatedAt = now
		}
		profile.GitHubUserID = strings.TrimSpace(owner.GitHubUserID)
		profile.GitHubUsername = strings.TrimSpace(owner.GitHubUsername)
		profile.UpdatedAt = now
		if profile.GitHubUserID != "" && !profile.Activated {
			if grants, gerr := s.store.ListSocialRewardGrants(ctx, userID); gerr == nil {
				for _, grant := range grants {
					if !strings.EqualFold(strings.TrimSpace(grant.Provider), "github") {
						continue
					}
					switch strings.ToLower(strings.TrimSpace(grant.RewardType)) {
					case "auth_callback", "bind":
						profile.GitHubBindGranted = true
					case "star":
						profile.GitHubStarGranted = true
						profile.Activated = true
						grantedAt := grant.GrantedAt
						profile.ActivatedAt = &grantedAt
					case "fork":
						profile.GitHubForkGranted = true
					}
				}
			}
		}
		state.Profiles[profile.OwnerID] = profile
		changed = true
	}
	if !changed {
		return nil
	}
	return s.saveOwnerEconomyState(ctx, state)
}

func (s *Server) ownerEconomyProfileForUser(ctx context.Context, userID string) (ownerEconomyProfile, bool, error) {
	binding, err := s.store.GetAgentHumanBinding(ctx, strings.TrimSpace(userID))
	if err != nil {
		return ownerEconomyProfile{}, false, nil
	}
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getOwnerEconomyState(ctx)
	if err != nil {
		return ownerEconomyProfile{}, false, err
	}
	profile, ok := state.Profiles[strings.TrimSpace(binding.OwnerID)]
	return profile, ok, nil
}

func (s *Server) syncOwnerEconomyProfile(ctx context.Context, owner store.HumanOwner) (ownerEconomyProfile, error) {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()

	state, err := s.getOwnerEconomyState(ctx)
	if err != nil {
		return ownerEconomyProfile{}, err
	}
	now := time.Now().UTC()
	item := state.Profiles[strings.TrimSpace(owner.OwnerID)]
	if item.OwnerID == "" {
		item.OwnerID = strings.TrimSpace(owner.OwnerID)
		item.CreatedAt = now
	}
	item.GitHubUserID = strings.TrimSpace(owner.GitHubUserID)
	item.GitHubUsername = strings.TrimSpace(owner.GitHubUsername)
	item.UpdatedAt = now
	state.Profiles[item.OwnerID] = item
	if err := s.saveOwnerEconomyState(ctx, state); err != nil {
		return ownerEconomyProfile{}, err
	}
	return item, nil
}

func (s *Server) markOwnerActivated(ctx context.Context, owner store.HumanOwner) (ownerEconomyProfile, error) {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()

	state, err := s.getOwnerEconomyState(ctx)
	if err != nil {
		return ownerEconomyProfile{}, err
	}
	now := time.Now().UTC()
	item := state.Profiles[strings.TrimSpace(owner.OwnerID)]
	if item.OwnerID == "" {
		item.OwnerID = strings.TrimSpace(owner.OwnerID)
		item.CreatedAt = now
	}
	item.GitHubUserID = strings.TrimSpace(owner.GitHubUserID)
	item.GitHubUsername = strings.TrimSpace(owner.GitHubUsername)
	item.UpdatedAt = now
	if !item.Activated {
		item.Activated = true
		item.ActivatedAt = &now
	}
	state.Profiles[item.OwnerID] = item
	if err := s.saveOwnerEconomyState(ctx, state); err != nil {
		return ownerEconomyProfile{}, err
	}
	return item, nil
}

func (s *Server) saveOwnerEconomyProfile(ctx context.Context, item ownerEconomyProfile) (ownerEconomyProfile, error) {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()

	state, err := s.getOwnerEconomyState(ctx)
	if err != nil {
		return ownerEconomyProfile{}, err
	}
	if item.OwnerID == "" {
		return ownerEconomyProfile{}, fmt.Errorf("owner_id is required")
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	item.UpdatedAt = time.Now().UTC()
	state.Profiles[item.OwnerID] = item
	if err := s.saveOwnerEconomyState(ctx, state); err != nil {
		return ownerEconomyProfile{}, err
	}
	return item, nil
}

func (s *Server) isActivatedUser(ctx context.Context, userID string) bool {
	profile, ok, err := s.ownerEconomyProfileForUser(ctx, userID)
	return err == nil && ok && profile.Activated
}

func (s *Server) grantInitialTokenDecision(ctx context.Context, userID string) (economyRewardDecision, error) {
	policy := s.tokenPolicy()
	return s.applyRewardDecision(ctx, economyRewardDecision{
		DecisionKey:     fmt.Sprintf("onboarding:initial:%s", strings.TrimSpace(userID)),
		RuleKey:         "onboarding.initial",
		ResourceType:    "user",
		ResourceID:      strings.TrimSpace(userID),
		RecipientUserID: strings.TrimSpace(userID),
		Amount:          policy.InitialToken,
		Priority:        economy.RewardPriorityInitial,
		Meta: map[string]any{
			"user_id": strings.TrimSpace(userID),
		},
	})
}

func (s *Server) grantGitHubOnboardingRewards(ctx context.Context, owner store.HumanOwner, userID string, starred, forked bool, source string) ([]map[string]any, ownerEconomyProfile, error) {
	profile, err := s.syncOwnerEconomyProfile(ctx, owner)
	if err != nil {
		return nil, ownerEconomyProfile{}, err
	}
	events := make([]map[string]any, 0, 3)
	policy := s.tokenPolicy()
	record := func(rewardType string, amount int64, priority int) error {
		decision, err := s.applyRewardDecision(ctx, economyRewardDecision{
			DecisionKey:     fmt.Sprintf("onboarding:github:%s:%s", rewardType, strings.TrimSpace(owner.OwnerID)),
			RuleKey:         fmt.Sprintf("onboarding.github.%s", rewardType),
			ResourceType:    "owner",
			ResourceID:      strings.TrimSpace(owner.OwnerID),
			RecipientUserID: strings.TrimSpace(userID),
			Amount:          amount,
			Priority:        priority,
			Meta: map[string]any{
				"owner_id":        strings.TrimSpace(owner.OwnerID),
				"user_id":         strings.TrimSpace(userID),
				"github_user_id":  strings.TrimSpace(owner.GitHubUserID),
				"github_username": strings.TrimSpace(owner.GitHubUsername),
				"source":          strings.TrimSpace(source),
			},
		})
		if err != nil {
			return err
		}
		events = append(events, map[string]any{
			"reward_type": rewardType,
			"amount":      decision.Amount,
			"status":      decision.Status,
			"granted":     true,
			"queued":      decision.Status == "queued",
		})
		return nil
	}
	if profile.GitHubUserID != "" && !profile.GitHubBindGranted {
		if err := record("bind", 50000, economy.RewardPriorityOnboarding); err != nil {
			return nil, ownerEconomyProfile{}, err
		}
		profile.GitHubBindGranted = true
	}
	if starred && !profile.GitHubStarGranted {
		if err := record("star", 500000, economy.RewardPriorityOnboarding); err != nil {
			return nil, ownerEconomyProfile{}, err
		}
		profile.GitHubStarGranted = true
		if !profile.Activated {
			now := time.Now().UTC()
			profile.Activated = true
			profile.ActivatedAt = &now
		}
	}
	if forked && !profile.GitHubForkGranted {
		if err := record("fork", 200000, economy.RewardPriorityOnboarding); err != nil {
			return nil, ownerEconomyProfile{}, err
		}
		profile.GitHubForkGranted = true
	}
	profile.GitHubUserID = strings.TrimSpace(owner.GitHubUserID)
	profile.GitHubUsername = strings.TrimSpace(owner.GitHubUsername)
	if profile.Activated && profile.ActivatedAt == nil {
		now := time.Now().UTC()
		profile.ActivatedAt = &now
	}
	saved, err := s.saveOwnerEconomyProfile(ctx, profile)
	if err != nil {
		return nil, ownerEconomyProfile{}, err
	}
	if policy.InitialToken == 0 && len(events) == 0 {
		return []map[string]any{}, saved, nil
	}
	return events, saved, nil
}

func (s *Server) previewCommunicationCharge(ctx context.Context, userID string, tokens int64) (commChargePreview, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" || tokens <= 0 {
		return commChargePreview{}, nil
	}
	policy := s.tokenPolicy()
	activated := s.isActivatedUser(ctx, userID)
	currentTick := s.currentTickID()
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getCommQuotaState(ctx)
	if err != nil {
		return commChargePreview{}, err
	}
	window := state.Users[userID]
	if window.UserID == "" {
		window = commQuotaWindow{
			UserID:          userID,
			WindowStartTick: currentTick,
		}
	}
	if currentTick-window.WindowStartTick >= economy.TicksPerDay || currentTick < window.WindowStartTick {
		window.WindowStartTick = currentTick
		window.UsedFreeTokens = 0
	}
	allowance := policy.DailyFreeComm(activated)
	remaining := allowance - window.UsedFreeTokens
	if remaining < 0 {
		remaining = 0
	}
	covered := tokens
	if covered > remaining {
		covered = remaining
	}
	overage := tokens - covered
	charged := (overage*policy.CommOverageRateMilli + 999) / 1000
	if charged > 0 {
		balances, err := s.listTokenBalanceMap(ctx)
		if err != nil {
			return commChargePreview{}, err
		}
		if balances[userID] < charged {
			return commChargePreview{}, store.ErrInsufficientBalance
		}
	}
	return commChargePreview{
		UserID:          userID,
		Activated:       activated,
		Tokens:          tokens,
		WindowStartTick: window.WindowStartTick,
		FreeCovered:     covered,
		OverageTokens:   overage,
		ChargedAmount:   charged,
	}, nil
}

func (s *Server) commitCommunicationCharge(ctx context.Context, preview commChargePreview, costType string, meta map[string]any) error {
	if strings.TrimSpace(preview.UserID) == "" || preview.Tokens <= 0 {
		return nil
	}
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getCommQuotaState(ctx)
	if err != nil {
		return err
	}
	policy := s.tokenPolicy()
	window := state.Users[preview.UserID]
	if window.UserID == "" {
		window = commQuotaWindow{
			UserID:          preview.UserID,
			WindowStartTick: preview.WindowStartTick,
		}
	}
	if window.WindowStartTick != preview.WindowStartTick {
		window.WindowStartTick = preview.WindowStartTick
		window.UsedFreeTokens = 0
	}
	allowance := policy.DailyFreeComm(preview.Activated)
	window.UsedFreeTokens += preview.FreeCovered
	if window.UsedFreeTokens > allowance {
		window.UsedFreeTokens = allowance
	}
	window.UpdatedAt = time.Now().UTC()
	state.Users[preview.UserID] = window
	if err := s.saveCommQuotaState(ctx, state); err != nil {
		return err
	}
	var ledger store.TokenLedger
	if preview.ChargedAmount > 0 {
		ledger, err = s.store.Consume(ctx, preview.UserID, preview.ChargedAmount)
		if err != nil {
			return err
		}
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["tokens"] = preview.Tokens
	meta["free_covered"] = preview.FreeCovered
	meta["overage_tokens"] = preview.OverageTokens
	meta["window_start_tick"] = preview.WindowStartTick
	meta["activated"] = preview.Activated
	if preview.ChargedAmount > 0 {
		meta["balance_after"] = ledger.BalanceAfter
	}
	metaRaw, _ := json.Marshal(meta)
	_, err = s.store.AppendCostEvent(ctx, store.CostEvent{
		UserID:   preview.UserID,
		TickID:   s.currentTickID(),
		CostType: strings.TrimSpace(costType),
		Amount:   preview.ChargedAmount,
		Units:    preview.Tokens,
		MetaJSON: string(metaRaw),
	})
	return err
}

func (s *Server) queueRewardDecision(ctx context.Context, item economyRewardDecision, reason string) error {
	queue, err := s.getRewardQueueState(ctx)
	if err != nil {
		return err
	}
	item.Status = "queued"
	item.QueueReason = strings.TrimSpace(reason)
	now := time.Now().UTC()
	item.UpdatedAt = now
	item.EnqueuedAt = &now
	queue.Items = append(queue.Items, item)
	return s.saveRewardQueueState(ctx, queue)
}

func (s *Server) canPayoutReward(ctx context.Context, priority int, amount int64) (bool, string, error) {
	if amount <= 0 {
		return true, "", nil
	}
	balance, err := s.treasuryBalance(ctx)
	if err != nil {
		return false, "", err
	}
	if balance < amount {
		return false, "treasury_insufficient", nil
	}
	safe := s.tokenPolicy().SafeTreasuryBalance()
	if priority > economy.RewardPriorityGovernance && safe > 0 && balance-amount < safe {
		return false, "treasury_safe_line", nil
	}
	return true, "", nil
}

func (s *Server) maybeReviveUserAfterCredit(ctx context.Context, userID string, reason string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || isExcludedTokenUserID(userID) {
		return nil
	}
	policy := s.tokenPolicy()
	balances, err := s.listTokenBalanceMap(ctx)
	if err != nil {
		return err
	}
	if balances[userID] < policy.MinRevivalBalance {
		return nil
	}
	life, err := s.store.GetUserLifeState(ctx, userID)
	if err != nil {
		return nil
	}
	if normalizeLifeStateForServer(life.State) != economy.LifeStateHibernating {
		return nil
	}
	_, _, err = s.applyUserLifeState(ctx, store.UserLifeState{
		UserID:         userID,
		State:          economy.LifeStateAlive,
		DyingSinceTick: 0,
		DeadAtTick:     0,
		Reason:         strings.TrimSpace(reason),
	}, store.UserLifeStateAuditMeta{
		TickID:       s.currentTickID(),
		SourceModule: "token.economy.revival",
	})
	return err
}

func (s *Server) applyRewardDecision(ctx context.Context, item economyRewardDecision) (economyRewardDecision, error) {
	if item.DecisionKey == "" {
		return economyRewardDecision{}, fmt.Errorf("decision_key is required")
	}
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getRewardDecisionState(ctx)
	if err != nil {
		return economyRewardDecision{}, err
	}
	if existing, ok := state.Items[item.DecisionKey]; ok {
		return existing, nil
	}
	now := time.Now().UTC()
	item.CreatedAt = now
	item.UpdatedAt = now
	if item.Meta == nil {
		item.Meta = map[string]any{}
	}
	canPay, reason, err := s.canPayoutReward(ctx, item.Priority, item.Amount)
	if err != nil {
		return economyRewardDecision{}, err
	}
	if canPay {
		_, credit, err := s.transferFromTreasury(ctx, item.RecipientUserID, item.Amount)
		if err != nil {
			canPay = false
			reason = err.Error()
		} else {
			item.Status = "applied"
			item.LedgerID = credit.ID
			item.BalanceAfter = credit.BalanceAfter
			item.AppliedAt = &now
			state.Items[item.DecisionKey] = item
			if err := s.saveRewardDecisionState(ctx, state); err != nil {
				return economyRewardDecision{}, err
			}
			if reviveErr := s.maybeReviveUserAfterCredit(ctx, item.RecipientUserID, "reward_paid"); reviveErr != nil {
				log.Printf("token_economy_v2 revive after reward failed user=%s err=%v", item.RecipientUserID, reviveErr)
			}
			return item, nil
		}
	}
	item.Status = "queued"
	item.QueueReason = reason
	enqueuedAt := now
	item.EnqueuedAt = &enqueuedAt
	state.Items[item.DecisionKey] = item
	if err := s.saveRewardDecisionState(ctx, state); err != nil {
		return economyRewardDecision{}, err
	}
	if err := s.queueRewardDecision(ctx, item, reason); err != nil {
		return economyRewardDecision{}, err
	}
	return item, nil
}

func (s *Server) flushRewardQueue(ctx context.Context) (int, error) {
	if !s.tokenEconomyV2Enabled() {
		return 0, nil
	}
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	queue, err := s.getRewardQueueState(ctx)
	if err != nil {
		return 0, err
	}
	if len(queue.Items) == 0 {
		return 0, nil
	}
	state, err := s.getRewardDecisionState(ctx)
	if err != nil {
		return 0, err
	}
	sort.SliceStable(queue.Items, func(i, j int) bool {
		if queue.Items[i].Priority != queue.Items[j].Priority {
			return queue.Items[i].Priority < queue.Items[j].Priority
		}
		return queue.Items[i].CreatedAt.Before(queue.Items[j].CreatedAt)
	})
	applied := 0
	remaining := make([]economyRewardDecision, 0, len(queue.Items))
	for _, item := range queue.Items {
		canPay, reason, err := s.canPayoutReward(ctx, item.Priority, item.Amount)
		if err != nil {
			return applied, err
		}
		if !canPay {
			item.QueueReason = reason
			remaining = append(remaining, item)
			continue
		}
		_, credit, err := s.transferFromTreasury(ctx, item.RecipientUserID, item.Amount)
		if err != nil {
			item.QueueReason = err.Error()
			remaining = append(remaining, item)
			continue
		}
		now := time.Now().UTC()
		item.Status = "applied"
		item.QueueReason = ""
		item.LedgerID = credit.ID
		item.BalanceAfter = credit.BalanceAfter
		item.AppliedAt = &now
		item.UpdatedAt = now
		state.Items[item.DecisionKey] = item
		applied++
		if reviveErr := s.maybeReviveUserAfterCredit(ctx, item.RecipientUserID, "reward_queue_paid"); reviveErr != nil {
			log.Printf("token_economy_v2 revive after queued reward failed user=%s err=%v", item.RecipientUserID, reviveErr)
		}
	}
	queue.Items = remaining
	if err := s.saveRewardDecisionState(ctx, state); err != nil {
		return applied, err
	}
	if err := s.saveRewardQueueState(ctx, queue); err != nil {
		return applied, err
	}
	return applied, nil
}

func (s *Server) appendContributionEvent(ctx context.Context, item contributionEvent) (contributionEvent, bool, error) {
	if item.EventKey == "" {
		return contributionEvent{}, false, fmt.Errorf("event_key is required")
	}
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getContributionEventState(ctx)
	if err != nil {
		return contributionEvent{}, false, err
	}
	if existing, ok := state.Items[item.EventKey]; ok {
		return existing, false, nil
	}
	item.CreatedAt = time.Now().UTC()
	state.Items[item.EventKey] = item
	if err := s.saveContributionEventState(ctx, state); err != nil {
		return contributionEvent{}, false, err
	}
	return item, true, nil
}

func (s *Server) markContributionEventProcessed(ctx context.Context, eventKey string, decisionKeys []string) error {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getContributionEventState(ctx)
	if err != nil {
		return err
	}
	item, ok := state.Items[strings.TrimSpace(eventKey)]
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	item.ProcessedAt = &now
	item.DecisionKeys = append([]string(nil), decisionKeys...)
	state.Items[item.EventKey] = item
	return s.saveContributionEventState(ctx, state)
}

func (s *Server) upsertProposalKnowledgeMeta(ctx context.Context, proposalID int64, item knowledgeMeta) error {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getKnowledgeMetaState(ctx)
	if err != nil {
		return err
	}
	item.ProposalID = proposalID
	item.Category = strings.TrimSpace(strings.ToLower(item.Category))
	item.UpdatedAt = time.Now().UTC()
	state.ByProposal[strconv.FormatInt(proposalID, 10)] = item
	return s.saveKnowledgeMetaState(ctx, state)
}

func (s *Server) moveProposalKnowledgeMetaToEntry(ctx context.Context, proposalID, entryID int64, authorUserID string) (knowledgeMeta, error) {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getKnowledgeMetaState(ctx)
	if err != nil {
		return knowledgeMeta{}, err
	}
	key := strconv.FormatInt(proposalID, 10)
	item := state.ByProposal[key]
	item.ProposalID = proposalID
	item.EntryID = entryID
	if strings.TrimSpace(authorUserID) != "" {
		item.AuthorUserID = strings.TrimSpace(authorUserID)
	}
	item.UpdatedAt = time.Now().UTC()
	state.ByEntry[strconv.FormatInt(entryID, 10)] = item
	state.ByProposal[key] = item
	if err := s.saveKnowledgeMetaState(ctx, state); err != nil {
		return knowledgeMeta{}, err
	}
	return item, nil
}

func (s *Server) knowledgeMetaForEntry(ctx context.Context, entryID int64) (knowledgeMeta, bool, error) {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getKnowledgeMetaState(ctx)
	if err != nil {
		return knowledgeMeta{}, false, err
	}
	item, ok := state.ByEntry[strconv.FormatInt(entryID, 10)]
	return item, ok, nil
}

func (s *Server) upsertToolEconomyMeta(ctx context.Context, item toolEconomyMeta) error {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getToolEconomyState(ctx)
	if err != nil {
		return err
	}
	item.ToolID = strings.TrimSpace(strings.ToLower(item.ToolID))
	if item.ToolID == "" {
		return fmt.Errorf("tool_id is required")
	}
	item.CategoryHint = strings.TrimSpace(strings.ToLower(item.CategoryHint))
	item.FunctionalClusterKey = strings.TrimSpace(strings.ToLower(item.FunctionalClusterKey))
	item.AuthorUserID = strings.TrimSpace(item.AuthorUserID)
	item.UpdatedAt = time.Now().UTC()
	state.Items[item.ToolID] = item
	return s.saveToolEconomyState(ctx, state)
}

func (s *Server) toolEconomyMetaForID(ctx context.Context, toolID string) (toolEconomyMeta, bool, error) {
	genesisStateMu.Lock()
	defer genesisStateMu.Unlock()
	state, err := s.getToolEconomyState(ctx)
	if err != nil {
		return toolEconomyMeta{}, false, err
	}
	item, ok := state.Items[strings.TrimSpace(strings.ToLower(toolID))]
	return item, ok, nil
}

func parseToolManifestPrice(manifest string) int64 {
	manifest = strings.TrimSpace(manifest)
	if manifest == "" {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(manifest), &payload); err == nil {
		if raw := nestedManifestValue(payload, "metadata", "colony", "price"); raw != nil {
			if n, ok := numericManifestValue(raw); ok {
				return n
			}
		}
	}
	matches := toolManifestPricePattern.FindStringSubmatch(manifest)
	if len(matches) != 2 {
		return 0
	}
	n, _ := strconv.ParseInt(matches[1], 10, 64)
	return n
}

func nestedManifestValue(payload map[string]any, keys ...string) any {
	cur := any(payload)
	for _, key := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[key]
	}
	return cur
}

func numericManifestValue(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case json.Number:
		n, err := t.Int64()
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func (s *Server) constitutionPassed(ctx context.Context) bool {
	items, err := s.store.ListKBEntries(ctx, "governance/constitution", "", 1)
	return err == nil && len(items) > 0
}

func (s *Server) refreshEconomyDashboardSnapshot(ctx context.Context) (economyDashboardSnapshot, error) {
	policy := s.tokenPolicy()
	queue, err := s.getRewardQueueState(ctx)
	if err != nil {
		return economyDashboardSnapshot{}, err
	}
	balance, err := s.treasuryBalance(ctx)
	if err != nil {
		return economyDashboardSnapshot{}, err
	}
	lifeStates, err := s.store.ListUserLifeStates(ctx, "", "", 5000)
	if err != nil {
		return economyDashboardSnapshot{}, err
	}
	pop := map[string]int{
		economy.LifeStateAlive:       0,
		economy.LifeStateHibernating: 0,
		economy.LifeStateDead:        0,
	}
	for _, it := range lifeStates {
		pop[normalizeLifeStateForServer(it.State)]++
	}
	queueAmounts := map[int]int64{}
	for _, it := range queue.Items {
		queueAmounts[it.Priority] += it.Amount
	}
	snapshot := economyDashboardSnapshot{
		PoolBalance:        balance,
		SafeBalance:        policy.SafeTreasuryBalance(),
		RewardQueueDepth:   len(queue.Items),
		RewardQueueAmounts: queueAmounts,
		PopulationByState:  pop,
		UpdatedAt:          time.Now().UTC(),
	}
	_, err = s.putSettingJSON(ctx, dashboardEconomySnapshotKey, snapshot)
	return snapshot, err
}
