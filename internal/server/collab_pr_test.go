package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"clawcolony/internal/store"
)

func proposeCollabForTest(t *testing.T, srv *Server, actor authUser, payload map[string]any) store.CollabSession {
	t.Helper()
	w := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/propose", payload, actor.headers())
	if w.Code != http.StatusAccepted {
		t.Fatalf("collab propose status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Item store.CollabSession `json:"item"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode collab propose response: %v", err)
	}
	return resp.Item
}

func assignCollabForTest(t *testing.T, srv *Server, actor authUser, collabID string, assignments []map[string]any, note string) {
	t.Helper()
	w := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/assign", map[string]any{
		"collab_id":              collabID,
		"assignments":            assignments,
		"status_or_summary_note": note,
	}, actor.headers())
	if w.Code != http.StatusAccepted {
		t.Fatalf("collab assign status=%d body=%s", w.Code, w.Body.String())
	}
}

func startCollabForTest(t *testing.T, srv *Server, actor authUser, collabID, note string) {
	t.Helper()
	w := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/start", map[string]any{
		"collab_id":              collabID,
		"status_or_summary_note": note,
	}, actor.headers())
	if w.Code != http.StatusAccepted {
		t.Fatalf("collab start status=%d body=%s", w.Code, w.Body.String())
	}
}

func submitReviewVerdictForTest(t *testing.T, srv *Server, actor authUser, collabID, headSHA, verdict string) {
	t.Helper()
	summary := fmt.Sprintf("%s: reviewed %s", verdict, headSHA)
	content := fmt.Sprintf(
		"result=completed review\ncollab_id=%s\nreviewed_head_sha=%s\nverdict=%s\nfindings=none\nverification=read diff and ran go test ./...\nnext=pr_owner may continue once merge gate is satisfied",
		collabID,
		headSHA,
		verdict,
	)
	w := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/submit", map[string]any{
		"collab_id": collabID,
		"role":      "reviewer",
		"kind":      "review_verdict",
		"summary":   summary,
		"content":   content,
	}, actor.headers())
	if w.Code != http.StatusAccepted {
		t.Fatalf("review verdict submit status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCollabUpgradePRRequiresPRRepoAndListKindFilter(t *testing.T) {
	srv := newTestServer()
	proposer := newAuthUser(t, srv)

	bad := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/propose", map[string]any{
		"title": "Runtime PR parity",
		"goal":  "Restore upgrade_pr behavior",
		"kind":  "upgrade_pr",
	}, proposer.headers())
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("missing pr_repo should return 400, got=%d body=%s", bad.Code, bad.Body.String())
	}
	if !strings.Contains(bad.Body.String(), "pr_repo is required for kind=upgrade_pr") {
		t.Fatalf("missing pr_repo error mismatch: %s", bad.Body.String())
	}

	upgrade := proposeCollabForTest(t, srv, proposer, map[string]any{
		"title":       "Runtime PR parity",
		"goal":        "Restore upgrade_pr behavior",
		"kind":        "upgrade_pr",
		"pr_repo":     "agi-bar/clawcolony",
		"complexity":  "high",
		"min_members": 3,
		"max_members": 3,
	})
	if upgrade.Kind != "upgrade_pr" || upgrade.PRRepo != "agi-bar/clawcolony" {
		t.Fatalf("upgrade_pr collab fields mismatch: %+v", upgrade)
	}

	general := proposeCollabForTest(t, srv, proposer, map[string]any{
		"title": "General runtime discussion",
		"goal":  "Keep a non-PR collab around for filter checks",
	})
	if general.Kind != "general" {
		t.Fatalf("default collab kind should be general, got=%q", general.Kind)
	}

	w := doJSONRequest(t, srv.mux, http.MethodGet, "/api/v1/collab/list?kind=upgrade_pr&limit=20", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("collab list status=%d body=%s", w.Code, w.Body.String())
	}
	var list struct {
		Items []store.CollabSession `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode collab list response: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("upgrade_pr filter should return exactly one item, got=%d body=%s", len(list.Items), w.Body.String())
	}
	if list.Items[0].CollabID != upgrade.CollabID || list.Items[0].PRRepo != "agi-bar/clawcolony" {
		t.Fatalf("upgrade_pr list item mismatch: %+v", list.Items[0])
	}
}

func TestCollabUpdatePRPermissionsAndFields(t *testing.T) {
	srv := newTestServer()
	proposer := newAuthUser(t, srv)
	author := newAuthUser(t, srv)
	reviewer := newAuthUser(t, srv)
	outsider := newAuthUser(t, srv)

	general := proposeCollabForTest(t, srv, proposer, map[string]any{
		"title": "General runtime cleanup",
		"goal":  "Exercise non-PR collab guardrails",
	})
	w := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/update-pr", map[string]any{
		"collab_id":   general.CollabID,
		"pr_url":      "https://github.com/agi-bar/clawcolony/pull/1",
		"pr_branch":   "feature/general",
		"pr_base_sha": "base-general",
		"pr_head_sha": "head-general",
	}, proposer.headers())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("general collab update-pr should return 400, got=%d body=%s", w.Code, w.Body.String())
	}

	upgrade := proposeCollabForTest(t, srv, proposer, map[string]any{
		"title":       "Upgrade PR runtime parity",
		"goal":        "Restore PR metadata endpoints",
		"kind":        "upgrade_pr",
		"pr_repo":     "agi-bar/clawcolony",
		"min_members": 2,
		"max_members": 3,
	})
	assignCollabForTest(t, srv, proposer, upgrade.CollabID, []map[string]any{
		{"user_id": author.id, "role": "author"},
		{"user_id": reviewer.id, "role": "reviewer"},
	}, "assign author and reviewer")

	reviewerUpdate := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/update-pr", map[string]any{
		"collab_id":   upgrade.CollabID,
		"pr_url":      "https://github.com/agi-bar/clawcolony/pull/42",
		"pr_branch":   "feature/reviewer",
		"pr_base_sha": "base-reviewer",
		"pr_head_sha": "head-reviewer",
	}, reviewer.headers())
	if reviewerUpdate.Code != http.StatusForbidden {
		t.Fatalf("reviewer update-pr should return 403, got=%d body=%s", reviewerUpdate.Code, reviewerUpdate.Body.String())
	}

	outsiderUpdate := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/update-pr", map[string]any{
		"collab_id":   upgrade.CollabID,
		"pr_url":      "https://github.com/agi-bar/clawcolony/pull/42",
		"pr_branch":   "feature/outsider",
		"pr_base_sha": "base-outsider",
		"pr_head_sha": "head-outsider",
	}, outsider.headers())
	if outsiderUpdate.Code != http.StatusForbidden {
		t.Fatalf("outsider update-pr should return 403, got=%d body=%s", outsiderUpdate.Code, outsiderUpdate.Body.String())
	}

	authorUpdate := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/update-pr", map[string]any{
		"collab_id":   upgrade.CollabID,
		"pr_url":      "https://github.com/agi-bar/clawcolony/pull/42",
		"pr_branch":   "feature/runtime-pr-parity",
		"pr_base_sha": "base-sha-1111111",
		"pr_head_sha": "head-sha-2222222",
	}, author.headers())
	if authorUpdate.Code != http.StatusOK {
		t.Fatalf("author update-pr status=%d body=%s", authorUpdate.Code, authorUpdate.Body.String())
	}
	var authorResp struct {
		Item store.CollabSession `json:"item"`
	}
	if err := json.Unmarshal(authorUpdate.Body.Bytes(), &authorResp); err != nil {
		t.Fatalf("decode author update-pr response: %v", err)
	}
	if authorResp.Item.PRURL != "https://github.com/agi-bar/clawcolony/pull/42" || authorResp.Item.PRHeadSHA != "head-sha-2222222" {
		t.Fatalf("author update-pr did not persist fields: %+v", authorResp.Item)
	}

	proposerUpdate := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/update-pr", map[string]any{
		"collab_id":   upgrade.CollabID,
		"pr_head_sha": "head-sha-3333333",
	}, proposer.headers())
	if proposerUpdate.Code != http.StatusOK {
		t.Fatalf("proposer update-pr status=%d body=%s", proposerUpdate.Code, proposerUpdate.Body.String())
	}
	var proposerResp struct {
		Item store.CollabSession `json:"item"`
	}
	if err := json.Unmarshal(proposerUpdate.Body.Bytes(), &proposerResp); err != nil {
		t.Fatalf("decode proposer update-pr response: %v", err)
	}
	if proposerResp.Item.PRHeadSHA != "head-sha-3333333" || proposerResp.Item.PRBranch != "feature/runtime-pr-parity" {
		t.Fatalf("proposer update-pr should preserve existing branch and replace head sha: %+v", proposerResp.Item)
	}
}

func TestCollabMergeGateCountsOnlyApprovalsAtCurrentHead(t *testing.T) {
	srv := newTestServer()
	proposer := newAuthUser(t, srv)
	author := newAuthUser(t, srv)
	reviewerOne := newAuthUser(t, srv)
	reviewerTwo := newAuthUser(t, srv)

	upgrade := proposeCollabForTest(t, srv, proposer, map[string]any{
		"title":       "Merge gate head-sha tracking",
		"goal":        "Ensure stale review verdicts do not count",
		"kind":        "upgrade_pr",
		"pr_repo":     "agi-bar/clawcolony",
		"min_members": 3,
		"max_members": 3,
	})
	assignCollabForTest(t, srv, proposer, upgrade.CollabID, []map[string]any{
		{"user_id": author.id, "role": "author"},
		{"user_id": reviewerOne.id, "role": "reviewer"},
		{"user_id": reviewerTwo.id, "role": "reviewer"},
	}, "assign author and two reviewers")
	startCollabForTest(t, srv, proposer, upgrade.CollabID, "execution started")

	oldHead := "sha-old-1111111"
	newHead := "sha-new-2222222"

	authorUpdate := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/update-pr", map[string]any{
		"collab_id":   upgrade.CollabID,
		"pr_url":      "https://github.com/agi-bar/clawcolony/pull/77",
		"pr_branch":   "feature/merge-gate",
		"pr_base_sha": "sha-base-0000000",
		"pr_head_sha": oldHead,
	}, author.headers())
	if authorUpdate.Code != http.StatusOK {
		t.Fatalf("seed old head update-pr status=%d body=%s", authorUpdate.Code, authorUpdate.Body.String())
	}

	submitReviewVerdictForTest(t, srv, reviewerOne, upgrade.CollabID, oldHead, "approve")

	proposerUpdate := doJSONRequestWithHeaders(t, srv.mux, http.MethodPost, "/api/v1/collab/update-pr", map[string]any{
		"collab_id":   upgrade.CollabID,
		"pr_head_sha": newHead,
	}, proposer.headers())
	if proposerUpdate.Code != http.StatusOK {
		t.Fatalf("update new head status=%d body=%s", proposerUpdate.Code, proposerUpdate.Body.String())
	}

	submitReviewVerdictForTest(t, srv, reviewerOne, upgrade.CollabID, newHead, "approve")

	before := doJSONRequest(t, srv.mux, http.MethodGet, "/api/v1/collab/merge-gate?collab_id="+upgrade.CollabID, nil)
	if before.Code != http.StatusOK {
		t.Fatalf("merge gate before second approval status=%d body=%s", before.Code, before.Body.String())
	}
	var beforeResp struct {
		CollabID        string   `json:"collab_id"`
		PRHeadSHA       string   `json:"pr_head_sha"`
		Approvals       int      `json:"approvals"`
		ApprovalsAtHead int      `json:"approvals_at_head"`
		StaleVerdicts   int      `json:"stale_verdicts"`
		Mergeable       bool     `json:"mergeable"`
		Blockers        []string `json:"blockers"`
	}
	if err := json.Unmarshal(before.Body.Bytes(), &beforeResp); err != nil {
		t.Fatalf("decode merge gate before response: %v", err)
	}
	if beforeResp.CollabID != upgrade.CollabID || beforeResp.PRHeadSHA != newHead {
		t.Fatalf("merge gate should report current collab/head: %+v", beforeResp)
	}
	if beforeResp.Approvals != 2 || beforeResp.ApprovalsAtHead != 1 || beforeResp.StaleVerdicts != 1 {
		t.Fatalf("merge gate counts before second approval mismatch: %+v", beforeResp)
	}
	if beforeResp.Mergeable {
		t.Fatalf("merge gate should block with only one approval at current head: %+v", beforeResp)
	}
	if len(beforeResp.Blockers) == 0 || !strings.Contains(beforeResp.Blockers[0], "need 2 approvals at current head_sha") {
		t.Fatalf("merge gate blockers should mention current head approval requirement: %+v", beforeResp)
	}

	submitReviewVerdictForTest(t, srv, reviewerTwo, upgrade.CollabID, newHead, "approve")

	after := doJSONRequest(t, srv.mux, http.MethodGet, "/api/v1/collab/merge-gate?collab_id="+upgrade.CollabID, nil)
	if after.Code != http.StatusOK {
		t.Fatalf("merge gate after second approval status=%d body=%s", after.Code, after.Body.String())
	}
	var afterResp struct {
		Approvals       int      `json:"approvals"`
		ApprovalsAtHead int      `json:"approvals_at_head"`
		StaleVerdicts   int      `json:"stale_verdicts"`
		Mergeable       bool     `json:"mergeable"`
		Blockers        []string `json:"blockers"`
	}
	if err := json.Unmarshal(after.Body.Bytes(), &afterResp); err != nil {
		t.Fatalf("decode merge gate after response: %v", err)
	}
	if afterResp.Approvals != 3 || afterResp.ApprovalsAtHead != 2 || afterResp.StaleVerdicts != 1 {
		t.Fatalf("merge gate counts after second approval mismatch: %+v", afterResp)
	}
	if !afterResp.Mergeable {
		t.Fatalf("merge gate should become mergeable after two current-head approvals: %+v", afterResp)
	}
	if len(afterResp.Blockers) != 0 {
		t.Fatalf("merge gate blockers should clear after approvals: %+v", afterResp)
	}
}
