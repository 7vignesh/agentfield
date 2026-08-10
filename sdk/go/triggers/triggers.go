// Package triggers provides types and factories for declaring inbound webhook
// and cron-schedule bindings on Go SDK reasoners.
//
// A reasoner declares external event sources via WithTriggers on
// RegisterReasoner. The canonical form passes typed Binding values
// created by Event() / Schedule() factories.
//
// The control plane registers a code-managed Trigger row per binding when
// the agent registers, so the agent never has to provision webhooks itself.
//
// Field-for-field equivalent of sdk/python/agentfield/triggers.py and
// sdk/typescript/src/triggers/types.ts.
package triggers

import (
	"encoding/json"
	"time"
)

// Context is the webhook-trigger metadata exposed to reasoners at runtime.
//
// Available as the *triggers.Context parameter in the handler (nil when the
// reasoner was invoked directly via app.Call instead of by an inbound event).
type Context struct {
	// AgentField trigger row ID; stable, equals the public URL slug.
	TriggerID string
	// Provider source ("stripe", "github", "slack", "cron", "generic_hmac", "generic_bearer").
	Source string
	// Provider's event type (or "" for cron tick).
	EventType string
	// AgentField inbound_event ID (replay key).
	EventID string
	// Provider's idempotency key (e.g. evt_xxx).
	IdempotencyKey string
	// When the control plane received the inbound event.
	ReceivedAt time.Time
	// Trigger event VC ID, if DID enabled.
	VCID string
}

// Transform is an optional sync function to convert a raw provider event
// into the reasoner's input. When set, the SDK runs Transform(rawEvent)
// before invoking the reasoner; the handler's input parameter receives
// the return value rather than the raw event. Must be synchronous.
type Transform func(rawEvent map[string]any) any

// EventOpts configures an event trigger binding.
type EventOpts struct {
	// Registered Source name (e.g. "stripe", "github", "slack",
	// "generic_hmac", "generic_bearer").
	Source string
	// Event types the reasoner cares about. Empty means "all".
	// Supports prefix-match: "pull_request" matches "pull_request.opened".
	Types []string
	// Name of the env var on the control plane that holds the provider's
	// webhook secret. Required for Sources whose secret_required is true.
	SecretEnv string
	// Source-specific JSON config (timestamp tolerance, custom header names, etc).
	Config json.RawMessage
	// Optional sync transform to convert raw provider event to reasoner input.
	Transform Transform
}

// ScheduleOpts configures a cron schedule trigger binding.
type ScheduleOpts struct {
	// 5-field cron expression (minute hour dom month dow).
	Expression string
	// IANA timezone name. Defaults to "UTC".
	Timezone string
	// Optional source-specific config.
	Config json.RawMessage
}

// Binding is a typed trigger binding — either an event trigger or a schedule
// trigger. Created via Event() or Schedule() factory functions. Carries both
// the wire-serialisable payload and the non-serialisable Transform.
type Binding struct {
	// Source is the provider name (for quick filtering without inspecting Wire).
	Source string
	// EventTypes is the set of subscribed event types (empty = all).
	EventTypes []string
	// SecretEnv is the env var name for the provider secret.
	SecretEnv string
	// Config is source-specific JSON config.
	Config json.RawMessage
	// Transform is the optional sync transform (not serialised to wire).
	TransformFn Transform
	// CodeOrigin is the source file:line where the binding was declared.
	CodeOrigin string
	// Kind distinguishes event from schedule bindings.
	Kind BindingKind
}

// BindingKind enumerates the types of trigger bindings.
type BindingKind int

const (
	// EventBinding represents a webhook event trigger.
	EventBinding BindingKind = iota
	// ScheduleBinding represents a cron schedule trigger.
	ScheduleBinding
)

// Event creates an event trigger binding from the given options.
func Event(opts EventOpts) Binding {
	return Binding{
		Source:      opts.Source,
		EventTypes:  opts.Types,
		SecretEnv:   opts.SecretEnv,
		Config:      opts.Config,
		TransformFn: opts.Transform,
		Kind:        EventBinding,
	}
}

// Schedule creates a schedule (cron) trigger binding from the given options.
func Schedule(opts ScheduleOpts) Binding {
	tz := opts.Timezone
	if tz == "" {
		tz = "UTC"
	}
	cfg := opts.Config
	if cfg == nil {
		raw, _ := json.Marshal(map[string]any{
			"expression": opts.Expression,
			"timezone":   tz,
		})
		cfg = raw
	}
	return Binding{
		Source: "cron",
		Config: cfg,
		Kind:   ScheduleBinding,
	}
}
