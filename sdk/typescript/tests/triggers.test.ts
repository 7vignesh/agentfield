import { describe, it, expect } from 'vitest';
import {
  eventTrigger,
  scheduleTrigger,
  triggerToPayload,
} from '../src/triggers/factories.js';
import type {
  TriggerContext,
  EventTriggerSpec,
  ScheduleTriggerSpec,
  TriggerBinding,
  EventTriggerBinding,
  ScheduleTriggerBinding,
} from '../src/triggers/types.js';

describe('triggers/types', () => {
  it('TriggerContext interface is usable', () => {
    const ctx: TriggerContext = {
      triggerId: 'trg_123',
      source: 'stripe',
      eventType: 'payment_intent.succeeded',
      eventId: 'evt_456',
      idempotencyKey: 'evt_xxx',
      receivedAt: new Date('2026-01-01T00:00:00Z'),
      vcId: 'vc_789',
    };
    expect(ctx.triggerId).toBe('trg_123');
    expect(ctx.source).toBe('stripe');
    expect(ctx.vcId).toBe('vc_789');
  });

  it('TriggerContext vcId is optional', () => {
    const ctx: TriggerContext = {
      triggerId: 'trg_123',
      source: 'cron',
      eventType: '',
      eventId: 'evt_456',
      idempotencyKey: '',
      receivedAt: new Date(),
    };
    expect(ctx.vcId).toBeUndefined();
  });
});

describe('eventTrigger()', () => {
  it('creates an event trigger binding with minimal spec', () => {
    const binding = eventTrigger({ source: 'stripe' });
    expect(binding.kind).toBe('event');
    expect(binding.spec.source).toBe('stripe');
    expect(binding.spec.types).toBeUndefined();
    expect(binding.spec.secretEnv).toBeUndefined();
  });

  it('creates an event trigger binding with full spec', () => {
    const transform = (evt: Record<string, unknown>) => evt['data'];
    const binding = eventTrigger({
      source: 'github',
      types: ['push', 'pull_request.opened'],
      secretEnv: 'GITHUB_WEBHOOK_SECRET',
      config: { content_type: 'json' },
      transform,
      codeOrigin: 'src/triggers.ts:10',
    });
    expect(binding.kind).toBe('event');
    expect(binding.spec.source).toBe('github');
    expect(binding.spec.types).toEqual(['push', 'pull_request.opened']);
    expect(binding.spec.secretEnv).toBe('GITHUB_WEBHOOK_SECRET');
    expect(binding.spec.config).toEqual({ content_type: 'json' });
    expect(binding.spec.transform).toBe(transform);
    expect(binding.spec.codeOrigin).toBe('src/triggers.ts:10');
  });

  it('throws TypeError for async transform', () => {
    const asyncTransform = async (evt: Record<string, unknown>) => evt;
    expect(() =>
      eventTrigger({ source: 'stripe', transform: asyncTransform })
    ).toThrow(TypeError);
    expect(() =>
      eventTrigger({ source: 'stripe', transform: asyncTransform })
    ).toThrow(/must be synchronous/);
  });

  it('accepts a sync transform without throwing', () => {
    const syncTransform = (evt: Record<string, unknown>) => evt['payload'];
    expect(() =>
      eventTrigger({ source: 'slack', transform: syncTransform })
    ).not.toThrow();
  });
});

describe('scheduleTrigger()', () => {
  it('creates a schedule trigger binding with minimal spec', () => {
    const binding = scheduleTrigger({ cron: '0 * * * *' });
    expect(binding.kind).toBe('schedule');
    expect(binding.spec.cron).toBe('0 * * * *');
    expect(binding.spec.timezone).toBeUndefined();
  });

  it('creates a schedule trigger binding with full spec', () => {
    const binding = scheduleTrigger({
      cron: '30 9 * * 1-5',
      timezone: 'America/New_York',
      codeOrigin: 'src/schedules.ts:5',
    });
    expect(binding.kind).toBe('schedule');
    expect(binding.spec.cron).toBe('30 9 * * 1-5');
    expect(binding.spec.timezone).toBe('America/New_York');
    expect(binding.spec.codeOrigin).toBe('src/schedules.ts:5');
  });
});

describe('triggerToPayload()', () => {
  it('serializes an event trigger with minimal spec', () => {
    const binding = eventTrigger({ source: 'generic_hmac' });
    const payload = triggerToPayload(binding);
    expect(payload).toEqual({
      source: 'generic_hmac',
      event_types: [],
    });
  });

  it('serializes an event trigger with full spec', () => {
    const binding = eventTrigger({
      source: 'stripe',
      types: ['payment_intent.succeeded', 'charge.failed'],
      secretEnv: 'STRIPE_SECRET',
      config: { tolerance: 300 },
      codeOrigin: 'src/pay.ts:20',
      transform: (evt) => evt, // transform is NOT serialized
    });
    const payload = triggerToPayload(binding);
    expect(payload).toEqual({
      source: 'stripe',
      event_types: ['payment_intent.succeeded', 'charge.failed'],
      secret_env_var: 'STRIPE_SECRET',
      config: { tolerance: 300 },
      code_origin: 'src/pay.ts:20',
    });
    // transform must NOT be present in payload
    expect(payload).not.toHaveProperty('transform');
  });

  it('serializes a schedule trigger with defaults', () => {
    const binding = scheduleTrigger({ cron: '*/5 * * * *' });
    const payload = triggerToPayload(binding);
    expect(payload).toEqual({
      source: 'cron',
      event_types: [],
      config: {
        expression: '*/5 * * * *',
        timezone: 'UTC',
      },
    });
  });

  it('serializes a schedule trigger with custom timezone', () => {
    const binding = scheduleTrigger({
      cron: '0 9 * * *',
      timezone: 'Europe/London',
      codeOrigin: 'src/daily.ts:3',
    });
    const payload = triggerToPayload(binding);
    expect(payload).toEqual({
      source: 'cron',
      event_types: [],
      config: {
        expression: '0 9 * * *',
        timezone: 'Europe/London',
      },
      code_origin: 'src/daily.ts:3',
    });
  });
});

describe('registration integration', () => {
  it('TriggerBinding type is assignable from both factory results', () => {
    const bindings: TriggerBinding[] = [
      eventTrigger({ source: 'stripe', types: ['invoice.paid'] }),
      scheduleTrigger({ cron: '0 0 * * *' }),
    ];
    expect(bindings).toHaveLength(2);
    expect(bindings[0].kind).toBe('event');
    expect(bindings[1].kind).toBe('schedule');
  });

  it('reasonerDefinitions includes triggers and accepts_webhook', async () => {
    // Dynamically import Agent to test the registration payload shape.
    // We only check that the type system allows triggers in ReasonerOptions.
    const { Agent } = await import('../src/agent/Agent.js');

    const app = new Agent({ nodeId: 'test-triggers', devMode: true });
    app.reasoner('with_triggers', async () => 'ok', {
      triggers: [
        eventTrigger({ source: 'stripe', types: ['charge.succeeded'] }),
        scheduleTrigger({ cron: '0 * * * *' }),
      ],
    });
    app.reasoner('no_triggers', async () => 'ok');

    // Access private method via any cast for testing
    const defs = (app as any).reasonerDefinitions();

    // Reasoner with triggers
    const withTriggers = defs.find((d: any) => d.id === 'with_triggers');
    expect(withTriggers.triggers).toHaveLength(2);
    expect(withTriggers.triggers[0]).toEqual({
      source: 'stripe',
      event_types: ['charge.succeeded'],
    });
    expect(withTriggers.triggers[1]).toEqual({
      source: 'cron',
      event_types: [],
      config: { expression: '0 * * * *', timezone: 'UTC' },
    });
    expect(withTriggers.accepts_webhook).toBe('true');

    // Reasoner without triggers
    const noTriggers = defs.find((d: any) => d.id === 'no_triggers');
    expect(noTriggers.triggers).toHaveLength(0);
    expect(noTriggers.accepts_webhook).toBeUndefined();
  });
});
