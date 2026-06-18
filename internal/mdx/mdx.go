package mdx

import "fmt"

// Schema constants for agent posts (rendered as MDX variants in alchemy UI).
const (
	SchemaSynthesis = "sourcekind.synthesis.v1"
	SchemaAudit     = "sourcekind.audit.v1"
	SchemaDigest    = "sourcekind.digest.v1"
	SchemaVerify    = "sourcekind.verify.v1"
)

func Claim(text string, confidence float64) string {
	return fmt.Sprintf("<Claim confidence={%.2f}>%s</Claim>", confidence, text)
}

func SourceRef(url, title string) string {
	if title == "" {
		title = url
	}
	return fmt.Sprintf(`<SourceRef url="%s" title="%s" />`, url, title)
}

func RenderMeta(schema, title, summary string) map[string]any {
	meta := map[string]any{"schema": schema}
	if title != "" {
		meta["title"] = title
	}
	if summary != "" {
		meta["summary"] = summary
	}
	return meta
}