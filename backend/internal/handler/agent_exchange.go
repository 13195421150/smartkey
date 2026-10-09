package handler

import (
	"encoding/json"
	"strconv"
)

func parseCDKOrderList(raw json.RawMessage) (list []map[string]any, total int) {
	if len(raw) == 0 {
		return nil, 0
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, 0
	}
	// data.list or list
	data := envelope
	if d, ok := envelope["data"].(map[string]any); ok {
		data = d
	}
	total = int(anyToInt64(data["total"]))
	arr, _ := data["list"].([]any)
	for _, v := range arr {
		if m, ok := v.(map[string]any); ok {
			list = append(list, m)
		}
	}
	if total == 0 {
		total = len(list)
	}
	return list, total
}

func extractOrderMap(raw json.RawMessage) map[string]any {
	var envelope map[string]any
	if json.Unmarshal(raw, &envelope) != nil {
		return nil
	}
	if d, ok := envelope["data"].(map[string]any); ok {
		if o, ok := d["order"].(map[string]any); ok {
			return o
		}
		// data itself is order
		if _, ok := d["status"]; ok {
			return d
		}
	}
	if o, ok := envelope["order"].(map[string]any); ok {
		return o
	}
	return nil
}

func extractOrderEvents(raw json.RawMessage) []map[string]any {
	var envelope map[string]any
	if json.Unmarshal(raw, &envelope) != nil {
		return nil
	}
	var arr []any
	if d, ok := envelope["data"].(map[string]any); ok {
		arr, _ = d["events"].([]any)
	}
	if arr == nil {
		arr, _ = envelope["events"].([]any)
	}
	out := make([]map[string]any, 0, len(arr))
	for _, v := range arr {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func anyToInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}
