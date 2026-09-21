package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tuzi/cdk-recharge-system/internal/cardplatform"
	"github.com/tuzi/cdk-recharge-system/internal/db"
)

// AdminCardPool exposes only masked, read-only selection diagnostics.
func AdminCardPool(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	plan := strings.TrimSpace(c.DefaultQuery("plan", "plus"))
	if plan != "plus" && plan != "pro_5x" && plan != "pro_20x" && plan != "go" {
		c.JSON(400, gin.H{"error": "unsupported diagnostic plan"})
		return
	}
	p, err := cardplatform.NewFromSettings().PreviewCardPool(c.Request.Context(), plan)
	if err != nil {
		writeCardErr(c, err)
		return
	}
	ids, err := db.ListActiveBlockedCardIDs()
	if err != nil {
		c.JSON(500, gin.H{"error": "读取本地排除卡片失败"})
		return
	}
	blocked := map[int64]bool{}
	for _, id := range ids {
		blocked[id] = true
	}
	rows := make([]gin.H, 0, len(p.Candidates))
	for _, row := range p.Candidates {
		number := row.CardNumber
		if len(number) > 4 {
			number = number[len(number)-4:]
		}
		rows = append(rows, gin.H{"card_id": row.CardID, "last_four": number, "issuer": row.Issuer, "picked": row.Picked, "skip": row.Skip, "skip_reason": row.SkipReason, "light_used": row.LightUsed, "light_remaining": row.LightRemain, "pro20_used": row.Pro20Used, "pro20_remaining": row.Pro20Remain, "available_usd": row.AvailableUSD.String(), "locally_excluded": blocked[row.CardID]})
	}
	c.JSON(http.StatusOK, gin.H{"plan": plan, "rule": p.Rule, "candidates": rows, "read_only": true})
}

func AdminCardUsage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"error": "invalid card id"})
		return
	}
	u, err := cardplatform.NewFromSettings().GetCardUsage(c.Request.Context(), id)
	if err != nil {
		writeCardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}
