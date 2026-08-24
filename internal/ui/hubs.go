package ui

import (
	"fmt"
	"strings"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
)

func hubsView(rows []mcp.Hub, selected, width, height int) string {
	var b strings.Builder
	fmt.Fprintln(&b, stColHead.Render("HUBS — centrality landmarks"))
	fmt.Fprintln(&b, stDim.Render("Highly connected symbols are useful cold-start points, not an execution order."))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stFainter.Render("symbol                 kind       location             in    out   degree   role"))
	for i, h := range rows {
		if i >= 40 {
			break
		}
		kind := h.Kind
		if kind == "" {
			kind = "symbol"
		}
		role := strings.Join(h.Flags, ",")
		line := fmt.Sprintf("%-23s %-10s %-20s %-5d %-5d %-8d %s", h.Name, kind, fmt.Sprintf("%s:%d", h.File, h.Line), h.FanIn, h.FanOut, h.Degree, role)
		fmt.Fprintln(&b, selRule(i == selected)+line)
	}
	if len(rows) == 0 {
		fmt.Fprintln(&b, stFainter.Render("no centrality data returned"))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stAmber.Render("Press enter on a landmark from Overview to open its symbol map."))
	return padView(b.String(), width, height)
}
