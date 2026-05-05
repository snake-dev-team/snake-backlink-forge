// payload.go — Phase 06: SePay webhook incoming payload struct + order-code regex.
package sepay

import "regexp"

// Payload is the JSON body SePay POSTs on every bank transaction event.
// Fields are named after the SePay API documentation (camelCase JSON).
// Unknown fields are silently ignored (json default) — forward-compat for SePay schema drift.
type Payload struct {
	ID              int64  `json:"id"`
	Gateway         string `json:"gateway"`
	TransactionDate string `json:"transactionDate"`
	AccountNumber   string `json:"accountNumber"`
	Code            string `json:"code"`
	Content         string `json:"content"`
	TransferType    string `json:"transferType"`
	TransferAmount  int64  `json:"transferAmount"`
	ReferenceCode   string `json:"referenceCode"`
	Description     string `json:"description"`
	Accumulated     int64  `json:"accumulated"`
	SubAccount      string `json:"subAccount"`
}

// OrderCodeRe matches the 12-hex-char order code embedded in the bank transfer memo.
// Pattern: case-insensitive "SBF TOPUP <12 hex chars>" (spaces flexible via \s+).
// [F2] callers MUST strings.ToUpper(match[1]) before any DB query — DB stores uppercase.
var OrderCodeRe = regexp.MustCompile(`(?i)SBF\s+TOPUP\s+([A-F0-9]{12})`)
