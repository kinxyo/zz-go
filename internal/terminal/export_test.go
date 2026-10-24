package terminal

// HelpText returns a help section's rows, one "left right" line each, for tests outside this
// package (the EXIT test needs cli, which imports terminal).
func HelpText(title string) string {
	var text string
	for _, s := range helpSections {
		if s.title != title {
			continue
		}
		for _, r := range s.rows {
			text += r.left + " " + r.right + "\n"
		}
	}
	return text
}
