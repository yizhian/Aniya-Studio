import { describe, it, expect } from 'vitest';
import { getToolDisplayName } from '../toolNameMap';
import { translations } from '../../i18n/translations';

const t = translations['zh-CN'];

describe('getToolDisplayName', () => {
  it('returns translation key for known legacy name', () => {
    const result = getToolDisplayName('todo_write', t);
    expect(result).toBe(t.toolNames.update_todo);
  });

  it('returns translation key for direct match', () => {
    const result = getToolDisplayName('read_file', t);
    expect(result).toBe(t.toolNames.read_file);
  });

  it('returns original name when no mapping exists', () => {
    const result = getToolDisplayName('unknown_tool_xyz', t);
    expect(result).toBe('unknown_tool_xyz');
  });

  it('handles empty string', () => {
    const result = getToolDisplayName('', t);
    expect(result).toBe('');
  });

  it('returns original for write_file tool', () => {
    const result = getToolDisplayName('write_file', t);
    expect(result).toBe(t.toolNames.write_file);
  });

  it('returns original for edit_file tool', () => {
    const result = getToolDisplayName('edit_file', t);
    expect(result).toBe(t.toolNames.edit_file);
  });
});
