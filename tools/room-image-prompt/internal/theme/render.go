package theme

import "strings"

// RenderFinal joins template and theme block per plan: trim trailing newlines from template,
// then "\n\n", then themeBlock (which must already end with a final newline).
func RenderFinal(template string, themeBlock string) string {
	template = strings.TrimRight(template, "\n")
	var b strings.Builder
	b.WriteString(template)
	b.WriteString("\n\n")
	b.WriteString(themeBlock)
	return b.String()
}
