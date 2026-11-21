import { filenameProblem } from './filename-validation';

describe('filenameProblem', () => {
  it('rejects empty names, traversal segments, and separators', () => {
    for (const name of ['', ' ', '.', '..', 'sub/file', 'sub\\file', 'name\u0000']) {
      expect(filenameProblem(name)).withContext(name).not.toBeNull();
    }
  });

  it('rejects names and suffixes reserved on Windows', () => {
    for (const name of ['CON', 'NUL.zip', 'lpt1.doc', 'COM¹.log', 'file.', 'file ', 'question?']) {
      expect(filenameProblem(name, true)).withContext(name).not.toBeNull();
    }
  });

  it('leaves valid platform-specific names available', () => {
    expect(filenameProblem('report.zip', true)).toBeNull();
    expect(filenameProblem('NUL.zip')).toBeNull();
    expect(filenameProblem('notes: questions?')).toBeNull();
  });
});
