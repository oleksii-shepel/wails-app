import { spawn } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const managerDir = path.join(__dirname, '..', 'manager');
const userArgs = process.argv.slice(2);

const args = ['dev', ...userArgs];
if (process.platform === 'win32' && !includesFlag(args, '-ldflags')) {
  args.push('-ldflags=-H=windowsgui');
}

const child = spawn('wails', args, {
  cwd: managerDir,
  stdio: 'inherit',
  shell: false,
});

child.on('exit', code => {
  process.exit(code ?? 0);
});

child.on('error', error => {
  console.error(`Failed to start Wails dev: ${error.message}`);
  process.exit(1);
});

function includesFlag(args, flag) {
  return args.some(arg => arg === flag || arg.startsWith(`${flag}=`));
}
