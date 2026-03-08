import '@testing-library/jest-dom';
import { vi } from 'vitest';

// Mock matchMedia — not available in jsdom
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

// Mock window.URL.createObjectURL used by reportsAPI
Object.defineProperty(window.URL, 'createObjectURL', {
  writable: true,
  value: vi.fn().mockReturnValue('blob:mock-url'),
});

Object.defineProperty(window.URL, 'revokeObjectURL', {
  writable: true,
  value: vi.fn(),
});

// Suppress noisy console.error in tests (e.g. React act() warnings)
// Comment out if you want to see them during debugging.
const originalError = console.error;
beforeAll(() => {
  console.error = (...args: unknown[]) => {
    const msg = String(args[0]);
    if (
      msg.includes('Warning:') ||
      msg.includes('act(') ||
      msg.includes('ReactDOM.render')
    ) return;
    originalError(...args);
  };
});

afterAll(() => {
  console.error = originalError;
});
