import { Injectable, Optional } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import {
  PluginLifecycleCoordinatorService,
  type ManagedPluginRuntimeSnapshot,
  type PluginActivationEventResult,
} from './plugin-lifecycle-coordinator.service';
import { BackendPluginCommandBridgeService } from './backend-plugin-command-bridge.service';
import { PluginInstallationStateService } from './plugin-installation-state.service';

export type PluginBootstrapStatus =
  | 'idle'
  | 'initializing'
  | 'ready'
  | 'degraded'
  | 'shutting-down'
  | 'stopped';

export interface PluginBootstrapSnapshot {
  status: PluginBootstrapStatus;
  runtimes: readonly ManagedPluginRuntimeSnapshot[];
  startupActivations: readonly PluginActivationEventResult[];
  errors: readonly string[];
}

const INITIAL_SNAPSHOT: PluginBootstrapSnapshot = {
  status: 'idle',
  runtimes: [],
  startupActivations: [],
  errors: [],
};

@Injectable({ providedIn: 'root' })
export class PluginBootstrapService {
  readonly state$ = createBehaviorSubject<PluginBootstrapSnapshot>(INITIAL_SNAPSHOT);

  private current: PluginBootstrapSnapshot = INITIAL_SNAPSHOT;
  private initializePromise: Promise<PluginBootstrapSnapshot> | null = null;
  private shutdownPromise: Promise<void> | null = null;

  constructor(
    private readonly lifecycle: PluginLifecycleCoordinatorService,
    private readonly installation: PluginInstallationStateService,
    @Optional() private readonly backendCommands?: BackendPluginCommandBridgeService,
  ) {}

  get snapshot(): PluginBootstrapSnapshot {
    return this.current;
  }

  /**
   * Discovers and reconciles plugins once, then activates packages that opted
   * into onStartup. Individual plugin activation failures degrade plugin
   * health but never prevent the file manager itself from starting.
   */
  initialize(selectionProvider: () => string[] = () => []): Promise<PluginBootstrapSnapshot> {
    if (this.current.status === 'ready' || this.current.status === 'degraded') {
      return Promise.resolve(this.current);
    }
    if (this.initializePromise) return this.initializePromise;

    this.publish({ ...INITIAL_SNAPSHOT, status: 'initializing' });
    this.initializePromise = this.runInitialize(selectionProvider).finally(() => {
      this.initializePromise = null;
    });
    return this.initializePromise;
  }

  /**
   * Application teardown is best-effort. We wait for an in-flight bootstrap
   * before deactivating so activation and cleanup cannot race each other.
   */
  shutdown(): Promise<void> {
    if (this.current.status === 'stopped') return Promise.resolve();
    if (this.shutdownPromise) return this.shutdownPromise;

    this.shutdownPromise = this.runShutdown().finally(() => {
      this.shutdownPromise = null;
    });
    return this.shutdownPromise;
  }

  private async runInitialize(selectionProvider: () => string[]): Promise<PluginBootstrapSnapshot> {
    const bootstrapErrors: string[] = [];
    try {
      const relaunched = await this.installation.ensureStartupHostPrivilege();
      if (relaunched) {
        const snapshot: PluginBootstrapSnapshot = {
          status: 'initializing',
          runtimes: [],
          startupActivations: [],
          errors: [],
        };
        this.publish(snapshot);
        return snapshot;
      }
    } catch (error) {
      bootstrapErrors.push(`startup privilege: ${errorMessage(error)}`);
    }

    if (this.backendCommands) {
      try {
        await this.backendCommands.start(selectionProvider);
      } catch (error) {
        bootstrapErrors.push(`backend commands: ${errorMessage(error)}`);
      }
    }

    try {
      const runtimes = await this.lifecycle.sync();
      const startupActivations = await this.lifecycle.activateStartup();
      const errors = [
        ...bootstrapErrors,
        ...startupActivations
          .filter(result => !result.activated && result.error)
          .map(result => `${result.pluginId}: ${result.error}`),
      ];

      const status: PluginBootstrapStatus = errors.length ? 'degraded' : 'ready';
      const snapshot: PluginBootstrapSnapshot = {
        status,
        runtimes,
        startupActivations,
        errors,
      };
      this.publish(snapshot);
      return snapshot;
    } catch (error) {
      const message = errorMessage(error);
      const snapshot: PluginBootstrapSnapshot = {
        status: 'degraded',
        runtimes: [],
        startupActivations: [],
        errors: [...bootstrapErrors, message],
      };
      this.publish(snapshot);
      return snapshot;
    }
  }

  private async runShutdown(): Promise<void> {
    if (this.initializePromise) {
      await this.initializePromise.catch(() => undefined);
    }

    this.publish({ ...this.current, status: 'shutting-down' });
    let results: Awaited<ReturnType<PluginLifecycleCoordinatorService['shutdown']>> = [];
    try {
      results = await this.lifecycle.shutdown();
    } finally {
      this.backendCommands?.stop();
    }
    const shutdownErrors = results
      .filter(result => result.error)
      .map(result => `${result.pluginId}: ${result.error}`);

    this.publish({
      ...this.current,
      status: 'stopped',
      errors: [...this.current.errors, ...shutdownErrors],
    });
  }

  private publish(snapshot: PluginBootstrapSnapshot): void {
    this.current = snapshot;
    this.state$.next(snapshot);
  }
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
