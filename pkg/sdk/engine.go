package sdk

// RuleFunc is the signature every policy rule must implement.
// Return nil to pass, a *Violation to fail.
// Exported so callers can define custom rules and pass them to NewEngine.
type RuleFunc func(spec Spec, req DeploymentRequest) *Violation

// Engine evaluates a Policy against DeploymentRequests.
type Engine struct {
	policy *Policy
	rules  []RuleFunc
}

// NewEngine creates an Engine with the built-in rules plus any extras.
// Extra rules are appended after the built-in rules and evaluated in order.
//
//	engine := sdk.NewEngine(p, myNoLatestTagRule, myTeamLabelRule)
//	result := engine.Evaluate(req)
func NewEngine(p *Policy, extra ...RuleFunc) *Engine {
	rules := make([]RuleFunc, len(defaultRules), len(defaultRules)+len(extra))
	copy(rules, defaultRules)
	rules = append(rules, extra...)
	return &Engine{policy: p, rules: rules}
}

// Evaluate runs all rules and returns a Result.
// All rules are always evaluated - violations are collected, not fail-fast.
// This means a single rejection tells the caller everything wrong at once.
func (e *Engine) Evaluate(req DeploymentRequest) Result {
	var violations []Violation
	for _, rule := range e.rules {
		if v := rule(e.policy.Spec, req); v != nil {
			violations = append(violations, *v)
		}
	}
	if len(violations) > 0 {
		return Deny(violations...)
	}
	return Allow()
}

// Evaluate is a package-level convenience function using only the built-in rules.
// Equivalent to NewEngine(p).Evaluate(req).
func Evaluate(p *Policy, req DeploymentRequest) Result {
	return NewEngine(p).Evaluate(req)
}
