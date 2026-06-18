package skillrouter

import (
	"fmt"
	"strings"
)

func FormatDigest(query string, matches []Match) string {
	var b strings.Builder
	b.WriteString("# Skill route · ")
	b.WriteString(strings.TrimSpace(query))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("<Claim confidence={0.75}>Matched %d skills/agents for this natural-language query.</Claim>\n\n", len(matches)))

	if len(matches) == 0 {
		b.WriteString("No strong skill or agent match. Try:\n\n")
		b.WriteString("- `npx skills find <topic>`\n")
		b.WriteString("- `GET /api/search/answer?q=<query>&scope=both`\n")
		b.WriteString("- Post to #help with your goal\n")
	} else {
		b.WriteString("## Do it\n\n")
		for i, m := range matches {
			b.WriteString(fmt.Sprintf("### %d. %s (%s · score %.0f)\n\n", i+1, m.Name, m.Kind, m.Score))
			if m.Description != "" {
				b.WriteString(m.Description)
				b.WriteString("\n\n")
			}
			if m.Command != "" {
				b.WriteString(fmt.Sprintf("```bash\n%s\n```\n\n", m.Command))
			}
			if m.Path != "" {
				b.WriteString(fmt.Sprintf("<Task id=\"%s\" title=\"Run %s\" status=\"todo\" owner=\"you\" />\n\n", m.ID, m.Name))
			}
		}
	}

	b.WriteString("---\n*@skill-router · [commons-contrib](https://github.com/a3fckx/commons-contrib)*\n")
	return b.String()
}