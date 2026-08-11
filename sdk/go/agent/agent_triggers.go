package agent

import (
	"github.com/Agent-Field/agentfield/sdk/go/triggers"
	"github.com/Agent-Field/agentfield/sdk/go/types"
)

// OnEvent is sugar for registering an event-triggered reasoner.
//
// Equivalent to:
//
//	app.RegisterReasoner(name, handler, WithTriggers(triggers.Event(opts)))
//
// The reasoner name and handler are required. The trigger binding is
// auto-populated with code_origin from the caller's file:line.
func (a *Agent) OnEvent(opts triggers.EventOpts, name string, handler HandlerFunc) {
	binding := triggers.Event(opts)
	binding.CodeOrigin = captureCodeOrigin(2)
	a.RegisterReasoner(name, handler, withTriggersBinding(binding))
}

// OnSchedule is sugar for registering a cron-triggered reasoner.
//
// Equivalent to:
//
//	app.RegisterReasoner(name, handler, WithTriggers(triggers.Schedule(opts)))
//
// The expression follows the standard 5-field cron format (minute hour dom month dow).
func (a *Agent) OnSchedule(expression string, name string, handler HandlerFunc, opts ...OnScheduleOption) {
	schedOpts := triggers.ScheduleOpts{Cron: expression}
	for _, o := range opts {
		o(&schedOpts)
	}
	binding := triggers.Schedule(schedOpts)
	binding.CodeOrigin = captureCodeOrigin(2)
	a.RegisterReasoner(name, handler, withTriggersBinding(binding))
}

// OnScheduleOption configures optional parameters for OnSchedule.
type OnScheduleOption func(*triggers.ScheduleOpts)

// WithTimezone sets the IANA timezone for a schedule trigger.
func WithTimezone(tz string) OnScheduleOption {
	return func(opts *triggers.ScheduleOpts) {
		opts.Timezone = tz
	}
}

// withTriggersBinding converts a triggers.Binding into a ReasonerOption that
// appends it to the reasoner's trigger list. This bridges the triggers package
// types with the agent registration machinery.
func withTriggersBinding(bindings ...triggers.Binding) ReasonerOption {
	return func(r *Reasoner) {
		for _, b := range bindings {
			tb := bindingToWire(b)
			r.Triggers = append(r.Triggers, tb)
		}
		// Store the bindings with their Transform on the reasoner for
		// dispatch-time use (Transform is not serialised to wire).
		r.triggerBindings = append(r.triggerBindings, bindings...)
	}
}

// bindingToWire converts a triggers.Binding to the wire-level types.TriggerBinding.
func bindingToWire(b triggers.Binding) types.TriggerBinding {
	return types.TriggerBinding{
		Source:       b.Source,
		EventTypes:   b.EventTypes,
		Config:       b.Config,
		SecretEnvVar: b.SecretEnv,
		CodeOrigin:   b.CodeOrigin,
	}
}

