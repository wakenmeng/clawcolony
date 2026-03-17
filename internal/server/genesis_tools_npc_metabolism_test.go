package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestToolInvokeSplitsManifestPriceUnderTokenEconomyV2(t *testing.T) {
	srv := newTestServer()
	authorUserID, authorAPIKey := seedActiveUserWithAPIKey(t, srv)
	callerUserID, callerAPIKey := seedActiveUserWithAPIKey(t, srv)
	reviewerUserID, reviewerAPIKey := seedActiveUserWithAPIKey(t, srv)

	register := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/tools/register", map[string]any{
		"tool_id":       "shared-checker",
		"name":          "Shared Checker",
		"description":   "checks shared state",
		"tier":          "T1",
		"category_hint": "workflow",
		"manifest":      `{"metadata":{"colony":{"price":100}}}`,
		"code":          "echo ok",
	}, apiKeyHeaders(authorAPIKey))
	if register.Code != http.StatusAccepted {
		t.Fatalf("tool register status=%d body=%s", register.Code, register.Body.String())
	}

	review := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/tools/review", map[string]any{
		"tool_id":     "shared-checker",
		"decision":    "approve",
		"review_note": "looks good",
	}, apiKeyHeaders(reviewerAPIKey))
	if review.Code != http.StatusAccepted {
		t.Fatalf("tool review status=%d reviewer=%s body=%s", review.Code, reviewerUserID, review.Body.String())
	}
	beforeTreasury := treasuryBalanceForTest(t, srv)

	invoke := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/tools/invoke", map[string]any{
		"tool_id": "shared-checker",
		"params":  map[string]any{"query": "hello"},
	}, apiKeyHeaders(callerAPIKey))
	if invoke.Code != http.StatusAccepted {
		t.Fatalf("tool invoke status=%d caller=%s body=%s", invoke.Code, callerUserID, invoke.Body.String())
	}

	var resp struct {
		Pricing struct {
			PriceToken         int64  `json:"price_token"`
			CreatorShare       int64  `json:"creator_share"`
			TreasuryShare      int64  `json:"treasury_share"`
			AuthorUserID       string `json:"author_user_id"`
			CallerBalanceAfter int64  `json:"caller_balance_after"`
		} `json:"pricing"`
	}
	if err := json.Unmarshal(invoke.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode invoke response: %v body=%s", err, invoke.Body.String())
	}
	if resp.Pricing.PriceToken != 100 || resp.Pricing.CreatorShare != 70 || resp.Pricing.TreasuryShare != 30 {
		t.Fatalf("unexpected tool pricing split: %+v body=%s", resp.Pricing, invoke.Body.String())
	}
	if resp.Pricing.AuthorUserID != authorUserID || resp.Pricing.CallerBalanceAfter != 900 {
		t.Fatalf("unexpected tool pricing metadata: %+v body=%s", resp.Pricing, invoke.Body.String())
	}
	if got := tokenBalanceForUser(t, srv, callerUserID); got != 900 {
		t.Fatalf("caller balance=%d want 900", got)
	}
	if got := tokenBalanceForUser(t, srv, authorUserID); got != 1070 {
		t.Fatalf("author balance=%d want 1070", got)
	}
	if got := treasuryBalanceForTest(t, srv); got != beforeTreasury+30 {
		t.Fatalf("treasury balance=%d want %d", got, beforeTreasury+30)
	}
}
