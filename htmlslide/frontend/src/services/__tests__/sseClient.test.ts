import { describe, it, expect } from 'vitest';
import { parseSSEChunk } from '../sseClient';

describe('parseSSEChunk', () => {
  it('parses a thinking event', () => {
    const input = 'event: thinking\ndata: {"content":"analyzing..."}\n\n';
    const result = parseSSEChunk(input);
    expect(result).toEqual([
      { event: 'thinking', data: '{"content":"analyzing..."}' },
    ]);
  });

  it('parses a text event', () => {
    const input = 'event: text\ndata: {"content":"Hello, world!"}\n\n';
    const result = parseSSEChunk(input);
    expect(result).toEqual([
      { event: 'text', data: '{"content":"Hello, world!"}' },
    ]);
  });

  it('parses a tool event with phase "start"', () => {
    const input =
      'event: tool\ndata: {"phase":"start","name":"write_file","call_id":"call_01"}\n\n';
    const result = parseSSEChunk(input);
    expect(result).toEqual([
      {
        event: 'tool',
        data: '{"phase":"start","name":"write_file","call_id":"call_01"}',
      },
    ]);
  });

  it('parses a tool event with phase "result"', () => {
    const input =
      'event: tool\ndata: {"phase":"result","name":"write_file","call_id":"call_01","success":true,"summary":"ok","duration_ms":150}\n\n';
    const result = parseSSEChunk(input);
    expect(result).toEqual([
      {
        event: 'tool',
        data: '{"phase":"result","name":"write_file","call_id":"call_01","success":true,"summary":"ok","duration_ms":150}',
      },
    ]);
  });

  it('parses an error event', () => {
    const input =
      'event: error\ndata: {"code":"agent_error","message":"failed","recoverable":true}\n\n';
    const result = parseSSEChunk(input);
    expect(result).toEqual([
      {
        event: 'error',
        data: '{"code":"agent_error","message":"failed","recoverable":true}',
      },
    ]);
  });

  it('parses a done event', () => {
    const input =
      'event: done\ndata: {"project_id":"p1","version":"v001","total_rounds":3}\n\n';
    const result = parseSSEChunk(input);
    expect(result).toEqual([
      {
        event: 'done',
        data: '{"project_id":"p1","version":"v001","total_rounds":3}',
      },
    ]);
  });

  it('ignores keepalive comments and returns empty array', () => {
    const input = ': keepalive\n\n';
    const result = parseSSEChunk(input);
    expect(result).toEqual([]);
  });

  it('parses multiple events in one chunk', () => {
    const input =
      'event: thinking\ndata: {"content":"first"}\n\nevent: text\ndata: {"content":"second"}\n\n';
    const result = parseSSEChunk(input);
    expect(result).toEqual([
      { event: 'thinking', data: '{"content":"first"}' },
      { event: 'text', data: '{"content":"second"}' },
    ]);
  });

  it('parses a single event without trailing double-newline', () => {
    // parseSSEChunk splits by \n\n to separate multiple events within a chunk.
    // A single event without \n\n is treated as one complete part.
    const input = 'event: text\ndata: {"content":"partial"}';
    const result = parseSSEChunk(input);
    expect(result).toEqual([
      { event: 'text', data: '{"content":"partial"}' },
    ]);
  });

  it('returns empty array for empty or malformed input', () => {
    expect(parseSSEChunk('')).toEqual([]);
    expect(parseSSEChunk('garbage\nstring\nno\nproper\nformat\n\n')).toEqual([]);
  });
});
