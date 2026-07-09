import type { ReasonerContext } from '../context/ReasonerContext.js';
import type { TriggerBinding } from '../triggers/types.js';

export interface ReasonerDefinition<TInput = any, TOutput = any> {
  name: string;
  handler: ReasonerHandler<TInput, TOutput>;
  options?: ReasonerOptions;
}

export type ReasonerHandler<TInput = any, TOutput = any> = (
  ctx: ReasonerContext<TInput>
) => Promise<TOutput> | TOutput;

export interface ReasonerOptions {
  tags?: string[];
  description?: string;
  inputSchema?: any;
  outputSchema?: any;
  trackWorkflow?: boolean;
  memoryConfig?: any;
  /** Force control-plane verification instead of local verification for this reasoner. */
  requireRealtimeValidation?: boolean;
  /**
   * Trigger bindings for this reasoner. When present, the control plane
   * registers inbound webhook / cron triggers so events are routed to
   * this reasoner automatically.
   *
   * Use `eventTrigger()` or `scheduleTrigger()` factories to create bindings.
   */
  triggers?: TriggerBinding[];
}
