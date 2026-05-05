// Vendor shim — re-exports `ignore` npm package installed in ../../node_modules.
// Restored after missing from ClaudeKit bundle — root cause of PreToolUse/PostToolUse
// hook spam "Cannot find module './vendor/ignore.cjs'" node:internal/modules/cjs/loader:1459.
module.exports = require('../../node_modules/ignore');
