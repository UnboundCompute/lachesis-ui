package ui

import (
	"encoding/json"
	"fmt"
	"strings"
)

// scanView renders the investigation queue and its coverage census. It keeps
// the server's neutral wording visible: a scan produces questions, not verdicts.
func scanView(data map[string]any, width, height int) string {
	var b strings.Builder
	fmt.Fprintln(&b, stColHead.Render("SCAN — questions to investigate"))
	fmt.Fprintln(&b, stDim.Render("Ranked graph observations. A row is not a proof of a defect."))
	fmt.Fprintln(&b)
	if len(data) == 0 {
		fmt.Fprintln(&b, stAmber.Render("scan returned no data"))
		return padView(b.String(), width, height)
	}
	if census, ok := data["census"]; ok {
		fmt.Fprintf(&b, "%s %v\n", stCyan.Render("coverage"), census)
	}
	if page, ok := data["page"]; ok {
		fmt.Fprintf(&b, "%s %v\n", stDim.Render("page"), page)
	}
	if queue, ok := data["queue"]; ok {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, stColHead.Render("INVESTIGATION QUEUE"))
		if rows, ok := queue.([]any); ok {
			for i, row := range rows {
				raw, _ := json.Marshal(row)
				var m map[string]any
				_ = json.Unmarshal(raw, &m)
				fmt.Fprintf(&b, "%s %s\n", selRule(i == 0), stFg.Render(scanRow(m)))
			}
		} else {
			fmt.Fprintln(&b, stFg.Render(fmt.Sprint(queue)))
		}
	}
	if _, ok := data["queue"]; !ok {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, stAmber.Render("no investigation rows were returned"))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stFainter.Render("Use Findings to inspect a candidate capsule; unknown coverage stays visible."))
	return padView(b.String(), width, height)
}

func scanRow(m map[string]any) string {
	for _, key := range []string{"entrypoint", "sink", "site", "callee", "id"} {
		if v, ok := m[key]; ok {
			return fmt.Sprintf("%-16s %v", key, v)
		}
	}
	return fmt.Sprint(m)
}
