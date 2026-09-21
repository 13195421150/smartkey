package handler

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/tuzi/cdk-recharge-system/internal/cardplatform"
)

type lookupEvent struct {
	CreatedAt string `json:"created_at,omitempty"`
	Step      string `json:"step,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Status    string `json:"status,omitempty"`
}

func addLookupDetail(ctx context.Context, cli *cardplatform.Client, resp *cdkLookupResult, cdkID int64) {
	if resp.OrderID <= 0 {
		return
	}
	raw, err := cli.GetCDKOrder(ctx, strconv.FormatInt(resp.OrderID, 10))
	if err != nil {
		return
	}
	order := publicLookupOrder(raw)
	if order == nil || lookupOrderID(order) != resp.OrderID || anyToInt64(order["cdk_id"]) != cdkID {
		return
	}
	if strAny(order["status"]) != "" {
		applyLookupOrder(resp, order)
	}
	applyLookupTimeline(resp, raw)
}

func applyLookupTimeline(resp *cdkLookupResult, raw json.RawMessage) {
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return
	}
	if d, ok := body["data"].(map[string]any); ok {
		body = d
	}
	resp.DetailsAvailable = true
	events, _ := body["events"].([]any)
	for _, value := range events {
		event, ok := value.(map[string]any)
		if !ok {
			continue
		}
		item := lookupEvent{CreatedAt: strAny(event["created_at"]), Step: publicMessage(event, "step"), Code: publicMessage(event, "public_code"), Message: publicMessage(event, "public_message"), Status: publicMessage(event, "to_status")}
		if item.Message == "" && item.Code == "" && item.Status == "" {
			continue
		}
		resp.Events = append(resp.Events, item)
	}
	sort.SliceStable(resp.Events, func(i, j int) bool { return resp.Events[i].CreatedAt < resp.Events[j].CreatedAt })
	if len(resp.Events) > 50 {
		resp.Events = resp.Events[len(resp.Events)-50:]
	}
	if resp.Status == "failed" {
		for i := len(resp.Events) - 1; i >= 0; i-- {
			e := resp.Events[i]
			if e.Status != "declined" && e.Status != "failed_precharge" && e.Status != "cancelled" && e.Status != "failed" {
				continue
			}
			if resp.FailureReason == "" && e.Message != "" {
				resp.FailureReason = e.Message
			}
			if resp.ErrorCode == "" {
				resp.ErrorCode = e.Code
			}
			if resp.FailureReason != "" {
				break
			}
		}
	}
}
