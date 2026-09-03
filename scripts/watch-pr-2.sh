#!/usr/bin/env bash
set -euo pipefail

REPO="buihongduc132/gomuks"
PR_NUMBER=2
STATE_FILE="${HOME}/.cache/gomuks-pr2-watch.json"

mkdir -p "$(dirname "$STATE_FILE")"

# Fetch latest PR details
CURRENT_DATA=$(gh pr view "$PR_NUMBER" --repo "$REPO" --json headRefOid,updatedAt,comments,statusCheckRollup,state,reviews,latestReviews,reviewDecision 2>/dev/null || true)

if [ -z "$CURRENT_DATA" ]; then
    echo "Failed to fetch PR #$PR_NUMBER from $REPO"
    exit 1
fi

HEAD_OID=$(echo "$CURRENT_DATA" | jq -r '.headRefOid')
UPDATED_AT=$(echo "$CURRENT_DATA" | jq -r '.updatedAt')
PR_STATE=$(echo "$CURRENT_DATA" | jq -r '.state')
COMMENT_COUNT=$(echo "$CURRENT_DATA" | jq -r '.comments | length')
REVIEW_DECISION=$(echo "$CURRENT_DATA" | jq -r '.reviewDecision // ""')
LATEST_REVIEWS=$(echo "$CURRENT_DATA" | jq -r '.latestReviews // []')
APPROVED_COUNT=$(echo "$CURRENT_DATA" | jq -r '[.latestReviews[]? | select(.state == "APPROVED")] | length')

echo "=== PR #$PR_NUMBER Status ==="
echo "State: $PR_STATE | Review Decision: $REVIEW_DECISION | Approved Reviews: $APPROVED_COUNT"
echo "Head: ${HEAD_OID:0:8} | Comments: $COMMENT_COUNT | Updated: $UPDATED_AT"

if [ "$REVIEW_DECISION" = "APPROVED" ] || [ "$APPROVED_COUNT" -gt 0 ]; then
    echo "VERDICT: APPROVED"
elif [ "$PR_STATE" != "OPEN" ]; then
    echo "VERDICT: CLOSED_OR_MERGED ($PR_STATE)"
else
    echo "VERDICT: PENDING_APPROVAL"
fi

if [ -f "$STATE_FILE" ]; then
    PREV_COMMENTS=$(jq -r '.comments // 0' "$STATE_FILE" 2>/dev/null || true)
    if [ "$COMMENT_COUNT" -gt "$PREV_COMMENTS" ]; then
        echo "NEW_COMMENTS: $(($COMMENT_COUNT - $PREV_COMMENTS)) new comment(s) received since last check."
    fi
fi

# Save current state
jq -n \
  --arg headRefOid "$HEAD_OID" \
  --arg updatedAt "$UPDATED_AT" \
  --arg state "$PR_STATE" \
  --arg reviewDecision "$REVIEW_DECISION" \
  --argjson comments "$COMMENT_COUNT" \
  --argjson approvedCount "$APPROVED_COUNT" \
  '{headRefOid: $headRefOid, updatedAt: $updatedAt, state: $state, reviewDecision: $reviewDecision, comments: $comments, approvedCount: $approvedCount}' > "$STATE_FILE"
