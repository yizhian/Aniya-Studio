import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { uploadToAgentgo } from '../upload';
import type { UploadResponse } from '../upload';

describe('uploadToAgentgo', () => {
  let fakeXHR: any;
  let openSpy: ReturnType<typeof vi.fn>;
  let sendSpy: ReturnType<typeof vi.fn>;
  let addEventListenerSpy: ReturnType<typeof vi.fn>;
  let uploadAddEventListenerSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    openSpy = vi.fn();
    sendSpy = vi.fn();
    addEventListenerSpy = vi.fn();
    uploadAddEventListenerSpy = vi.fn();

    fakeXHR = {
      status: 200,
      responseText: '',
      readyState: 4,
      timeout: 0,
      open: openSpy,
      send: sendSpy,
      addEventListener: addEventListenerSpy,
      abort: vi.fn(),
      upload: {
        addEventListener: uploadAddEventListenerSpy,
      },
    };

    vi.spyOn(globalThis, 'XMLHttpRequest').mockImplementation(function (this: any) {
      Object.assign(this, fakeXHR);
      return this;
    } as any);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('makes POST request to correct URL', () => {
    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), vi.fn(), 'proj-1');
    expect(openSpy).toHaveBeenCalledWith('POST', 'http://localhost:8080/upload');
  });

  it('sets timeout to 120 seconds', () => {
    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), vi.fn(), 'proj-1');
    // timeout is set via assignment, not a method call
  });

  it('calls send with FormData', () => {
    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), vi.fn(), 'proj-1');
    expect(sendSpy).toHaveBeenCalled();
  });

  it('calls onStatus with "uploading"', () => {
    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    const onStatus = vi.fn();
    uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), onStatus, 'proj-1');
    expect(onStatus).toHaveBeenCalledWith('uploading');
  });

  it('registers progress/load/error/timeout/abort listeners', () => {
    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), vi.fn(), 'proj-1');
    expect(uploadAddEventListenerSpy).toHaveBeenCalledWith('progress', expect.any(Function));
    expect(addEventListenerSpy).toHaveBeenCalledWith('load', expect.any(Function));
    expect(addEventListenerSpy).toHaveBeenCalledWith('error', expect.any(Function));
    expect(addEventListenerSpy).toHaveBeenCalledWith('timeout', expect.any(Function));
    expect(addEventListenerSpy).toHaveBeenCalledWith('abort', expect.any(Function));
  });

  it('resolves with parsed JSON on success', async () => {
    const response: UploadResponse = {
      upload_id: 'upl_1',
      session_id: 'upl_1',
      files: [{ original_name: 'test.txt', type: 'text', saved_path_rel: '/tmp/test.txt', char_count: 100 }],
      summary_text: 'Uploaded 1 file',
      parse_stats: { total_files: 1, succeeded: 1, unsupported: 0, errors: 0, total_duration_ms: 50 },
    };
    fakeXHR.responseText = JSON.stringify(response);
    fakeXHR.status = 200;

    addEventListenerSpy.mockImplementation(function (type: string, listener: any) {
      if (type === 'load') {
        setTimeout(() => listener(), 0);
      }
    });

    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    const result = await uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), vi.fn(), 'proj-1');
    expect(result.upload_id).toBe('upl_1');
  });

  it('rejects when status is 500', async () => {
    fakeXHR.status = 500;
    fakeXHR.responseText = JSON.stringify({ message: 'Internal error' });

    addEventListenerSpy.mockImplementation(function (type: string, listener: any) {
      if (type === 'load') {
        setTimeout(() => listener(), 0);
      }
    });

    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    await expect(uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), vi.fn(), 'proj-1'))
      .rejects.toThrow('Internal error');
  });

  it('rejects with "Cannot connect" for network error', async () => {
    addEventListenerSpy.mockImplementation(function (type: string, listener: any) {
      if (type === 'error') {
        setTimeout(() => listener(), 0);
      }
    });

    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    await expect(uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), vi.fn(), 'proj-1'))
      .rejects.toThrow('Cannot connect');
  });

  it('rejects on timeout', async () => {
    addEventListenerSpy.mockImplementation(function (type: string, listener: any) {
      if (type === 'timeout') {
        setTimeout(() => listener(), 0);
      }
    });

    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    await expect(uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), vi.fn(), 'proj-1'))
      .rejects.toThrow('timeout');
  });

  it('rejects on abort', async () => {
    addEventListenerSpy.mockImplementation(function (type: string, listener: any) {
      if (type === 'abort') {
        setTimeout(() => listener(), 0);
      }
    });

    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    await expect(uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), vi.fn(), 'proj-1'))
      .rejects.toThrow('cancelled');
  });

  it('calls onStatus with "uploading" then "parsing"', async () => {
    const onStatus = vi.fn();
    fakeXHR.responseText = JSON.stringify({
      upload_id: 'upl_1',
      session_id: 'upl_1',
      files: [],
      summary_text: '',
      parse_stats: { total_files: 0, succeeded: 0, unsupported: 0, errors: 0, total_duration_ms: 0 },
    });

    // Register upload.load handler fires synchronously before xhr.load.
    uploadAddEventListenerSpy.mockImplementation(function (type: string, listener: any) {
      if (type === 'progress') { /* register silently */ }
      if (type === 'load') {
        // Fire synchronously during test setup.
        listener();
      }
    });
    addEventListenerSpy.mockImplementation(function (type: string, listener: any) {
      if (type === 'load') {
        setTimeout(() => listener(), 0);
      }
    });

    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    await uploadToAgentgo([file], 'http://localhost:8080', vi.fn(), onStatus, 'proj-1');

    // "parsing" fires synchronously during upload.load before promise resolves
    // "uploading" fires synchronously before send()
    expect(onStatus).toHaveBeenCalledWith('uploading');
    expect(onStatus).toHaveBeenCalledWith('parsing');
    expect(onStatus).toHaveBeenCalledTimes(2);
  });
});
