// Package compliance holds statutory regulatory rules for Africa OS.
//
// These rules (KYC tier thresholds, payment caps, currency restrictions)
// differ by country and regulator. They are policy, not formatting.
//
// The embedded YAML files serve as safe compile-time defaults.
// Live applications should use the `loader` sub-package to fetch
// hot-reloaded rules from svc-compliance with in-process caching.
package compliance
