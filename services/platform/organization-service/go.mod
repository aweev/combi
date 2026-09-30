module github.com/combi/organization

go 1.25.0

require (
	github.com/combi/pkg v0.0.0
	github.com/google/uuid v1.6.0
	github.com/jmoiron/sqlx v1.4.0
	go.uber.org/fx v1.24.0
)

// Resolved locally while developing; the workspace (go.work) also covers
// this, the replace keeps standalone module builds working.
replace github.com/combi/pkg => ../../../pkg
