// Package templates — Telegram MarkdownV2 escape helper.
//
// Telegram MarkdownV2 reserves a fixed set of special characters that MUST be
// backslash-escaped when embedding user-controlled data inside a message that
// declares ParseMode = "MarkdownV2". Failure to escape causes Telegram to
// reject the message with HTTP 400 ("Bad Request: can't parse entities").
//
// Reference: https://core.telegram.org/bots/api#markdownv2-style
//
// Usage:
//
//	safe := templates.EscapeMDV2(userInput)
//	msg.Text = "Hello, " + safe
//	msg.ParseMode = "MarkdownV2"
package templates

import "strings"

// mdV2Special lists every character Telegram MarkdownV2 reserves.
// The order is preserved deliberately — replacing in this order keeps the
// output deterministic across Go versions and platforms.
var mdV2Special = []string{
	`_`, `*`, `[`, `]`, `(`, `)`, `~`, "`",
	`>`, `#`, `+`, `-`, `=`, `|`,
	`{`, `}`, `.`, `!`,
}

// EscapeMDV2 returns s with every MarkdownV2 reserved character backslash-escaped.
// Safe to call on already-escaped strings (it will double-escape backslashes
// from prior runs — call once per render path).
func EscapeMDV2(s string) string {
	for _, c := range mdV2Special {
		s = strings.ReplaceAll(s, c, `\`+c)
	}
	return s
}
