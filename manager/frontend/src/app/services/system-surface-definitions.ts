import type { SystemSurfaceDescriptor } from './system-surface-panel-source';

export const SYSTEM_SURFACES: readonly SystemSurfaceDescriptor[] = [
  {
    kind: 'tasks', title: 'Scheduled Tasks',
    columns: [
      { id: 'state', label: 'State', field: 'state', width: '100px' },
      { id: 'path', label: 'Task Path', field: 'path' },
      { id: 'nextRun', label: 'Next Run', field: 'nextRun', width: '170px' },
    ],
  },
  {
    kind: 'services', title: 'Services',
    columns: [
      { id: 'status', label: 'Status', field: 'status', width: '100px' },
      { id: 'startType', label: 'Startup', field: 'startType', width: '110px' },
      { id: 'displayName', label: 'Display Name', field: 'displayName' },
    ],
  },
  {
    kind: 'processes', title: 'Processes',
    columns: [
      { id: 'pid', label: 'PID', field: 'pid', width: '82px' },
      { id: 'cpu', label: 'CPU', field: 'cpu', width: '86px' },
      { id: 'workingSet', label: 'Working Set', field: 'workingSet', width: '120px' },
      { id: 'path', label: 'Executable', field: 'path' },
    ],
  },
  {
    kind: 'devices', title: 'Devices',
    columns: [
      { id: 'status', label: 'Status', field: 'status', width: '100px' },
      { id: 'class', label: 'Class', field: 'class', width: '140px' },
      { id: 'instanceId', label: 'Instance ID', field: 'instanceId' },
    ],
  },
  {
    kind: 'network', title: 'Network',
    columns: [
      { id: 'kind', label: 'Kind', field: 'kind', width: '90px' },
      { id: 'status', label: 'Status', field: 'status', width: '100px' },
      { id: 'local', label: 'Local', field: 'local' },
      { id: 'remote', label: 'Remote', field: 'remote' },
    ],
  },
  {
    kind: 'environment', title: 'Environment Variables',
    columns: [
      { id: 'scope', label: 'Scope', field: 'scope', width: '100px' },
      { id: 'value', label: 'Value', field: 'value' },
    ],
  },
  {
    kind: 'shell-extensions', title: 'Shell Extensions',
    columns: [
      { id: 'scope', label: 'Scope', field: 'scope', width: '100px' },
      { id: 'target', label: 'Target', field: 'target', width: '150px' },
      { id: 'clsid', label: 'CLSID', field: 'clsid' },
    ],
  },
] as const;