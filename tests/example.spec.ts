import { expect, test } from '@playwright/test';
import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process';

const frontendPorts: Record<string, string> = {
  chromium: '41737',
  firefox: '41738',
  webkit: '41739',
};
let frontendURL = '';
let viteProcess: ChildProcessWithoutNullStreams | null = null;

test.beforeAll(async ({}, testInfo) => {
  const port = frontendPorts[testInfo.project.name] || '41737';
  frontendURL = `http://127.0.0.1:${port}`;
  viteProcess = spawn(process.execPath, ['node_modules/vite/bin/vite.js', '--host', '127.0.0.1', '--port', port, '--strictPort'], {
    cwd: 'frontend',
    env: { ...process.env, FORCE_COLOR: '0' },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  await new Promise<void>((resolve, reject) => {
    let settled = false;
    let output = '';
    const timeout = setTimeout(() => {
      if (settled) {
        return;
      }
      settled = true;
      reject(new Error(`Vite dev server did not become ready.\n${output}`));
    }, 15000);
    const finish = (error?: Error) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timeout);
      if (error) {
        reject(error);
        return;
      }
      resolve();
    };
    const handleOutput = (chunk: Buffer) => {
      output += chunk.toString();
      if (output.includes('Local:')) {
        finish();
      }
    };
    viteProcess?.stdout.on('data', handleOutput);
    viteProcess?.stderr.on('data', handleOutput);
    viteProcess?.on('exit', (code) => {
      finish(new Error(`Vite dev server exited early with code ${code}.\n${output}`));
    });
    viteProcess?.on('error', finish);
  });
});

test.afterAll(async () => {
  if (!viteProcess || viteProcess.killed) {
    return;
  }
  await new Promise<void>((resolve) => {
    const timeout = setTimeout(resolve, 2000);
    viteProcess?.once('exit', () => {
      clearTimeout(timeout);
      resolve();
    });
    viteProcess?.kill();
  });
  viteProcess = null;
});

test('keeps desktop UI alive when completed task result refresh fails', async ({ page }) => {
  const pageErrors: string[] = [];
  page.on('pageerror', (error) => {
    pageErrors.push(error.message);
  });

  await page.addInitScript(() => {
    type ProbeCallback = (payload: unknown) => void;

    const listeners = new Map<string, Set<ProbeCallback>>();
    const commandResult = (code: string, data: unknown = null, message = 'ok', ok = true) => ({
      code,
      data,
      message,
      ok,
      schema_version: 'cfst-gui-wails-v1',
      task_id: null,
      warnings: [],
    });
    const appBridge = {
      CheckForUpdates: async () => commandResult('UPDATE_CHECK_OK', { current_version: 'test', release_url: '' }),
      CheckStorageHealth: async () => commandResult('STORAGE_HEALTH_READY', { storage: { current_dir: '/tmp', runtime_dir: '/tmp', writable: true } }),
      GetAppInfo: async () => commandResult('APP_INFO_READY', { current_version: 'test', platform: 'desktop', release_url: '' }),
      ListResultFile: async () => {
        throw new Error('simulated result refresh failure');
      },
      LoadColoDictionaryStatus: async () => commandResult('COLO_DICTIONARY_STATUS_READY', null),
      LoadDesktopConfig: async () =>
        commandResult('CONFIG_READY', {
          config_snapshot: {},
          draft_status: { recoverable: false },
          source_profiles: { active_profile_id: '', items: [], schema_version: 'cfst-gui-source-profiles-v1' },
          storage: { current_dir: '/tmp', runtime_dir: '/tmp', writable: true },
        }),
      LoadDesktopDraft: async () => commandResult('DESKTOP_DRAFT_READY', { recoverable: false }),
      LoadSchedulerStatus: async () => commandResult('SCHEDULER_STATUS_READY', null),
      LoadSourceProfiles: async () => commandResult('SOURCE_PROFILE_LOAD_OK', { active_profile_id: '', items: [] }),
      LoadTaskSnapshot: async () => {
        throw new Error('simulated task refresh failure');
      },
      RecordFrontendRuntimeError: async (payload: Record<string, unknown>) => {
        (window as unknown as { __cfstRuntimeErrors: Record<string, unknown>[] }).__cfstRuntimeErrors.push(payload);
        return commandResult('FRONTEND_RUNTIME_ERROR_LOGGED', { log_path: '/tmp/error-log.txt' }, 'logged');
      },
      SaveDesktopDraft: async () => commandResult('DESKTOP_DRAFT_SAVE_OK', { recoverable: false }),
    };

    (window as unknown as { __cfstRuntimeErrors: Record<string, unknown>[] }).__cfstRuntimeErrors = [];
    (window as unknown as { __cfstListenerCount: (eventName: string) => number }).__cfstListenerCount = (eventName: string) => listeners.get(eventName)?.size ?? 0;
    (window as unknown as { __emitProbeEvent: (payload: unknown) => void }).__emitProbeEvent = (payload: unknown) => {
      listeners.get('desktop:probe')?.forEach((callback) => callback(payload));
    };
    (window as unknown as { go: unknown }).go = { app: { App: appBridge }, main: { App: appBridge } };
    (window as unknown as { runtime: unknown }).runtime = {
      EventsEmit: () => undefined,
      EventsOff: (eventName: string) => listeners.delete(eventName),
      EventsOffAll: () => listeners.clear(),
      EventsOnMultiple: (eventName: string, callback: ProbeCallback) => {
        const callbacks = listeners.get(eventName) ?? new Set<ProbeCallback>();
        callbacks.add(callback);
        listeners.set(eventName, callbacks);
        return () => callbacks.delete(callback);
      },
      LogDebug: () => undefined,
      LogError: () => undefined,
      LogFatal: () => undefined,
      LogInfo: () => undefined,
      LogPrint: () => undefined,
      LogTrace: () => undefined,
      LogWarning: () => undefined,
      WindowCenter: () => undefined,
      WindowGetSize: async () => ({ h: 760, w: 1180 }),
      WindowIsMaximised: async () => true,
      WindowMaximise: () => undefined,
      WindowSetSize: () => undefined,
      WindowUnfullscreen: () => undefined,
      WindowUnmaximise: () => undefined,
    };
  });

  await page.goto(frontendURL);
  await page.waitForFunction(() => (window as unknown as { __cfstListenerCount?: (eventName: string) => number }).__cfstListenerCount?.('desktop:probe'));

  await page.evaluate(() => {
    (window as unknown as { __emitProbeEvent: (payload: unknown) => void }).__emitProbeEvent({
      event: 'probe.completed',
      payload: {
        exported: 1,
        passed: 1,
        result_count: 1,
        target_path: '/tmp/result.csv',
      },
      schema_version: 'cfst-gui-wails-v1',
      seq: 1,
      task_id: 'desktop-completed-refresh-failure',
      ts: new Date().toISOString(),
    });
  });

  await expect(page.getByText('探测任务完成').first()).toBeVisible();
  await expect(page.getByText('结果刷新失败，可手动刷新').first()).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => (window as unknown as { __cfstRuntimeErrors: Record<string, unknown>[] }).__cfstRuntimeErrors.length))
    .toBe(1);
  expect(pageErrors).toEqual([]);
});
