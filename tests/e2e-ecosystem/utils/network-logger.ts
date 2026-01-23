import { Page, Request, Response } from '@playwright/test';

/**
 * Network Logger Utility
 *
 * Captures network errors for forensic analysis when E2E tests fail.
 */

interface NetworkError {
  url: string;
  status: number;
  statusText: string;
  timestamp: Date;
  method: string;
}

export class NetworkLogger {
  private errors: NetworkError[] = [];
  private page: Page;

  constructor(page: Page) {
    this.page = page;
    this.setupListeners();
  }

  private setupListeners(): void {
    this.page.on('response', (response: Response) => {
      if (response.status() >= 400) {
        const request = response.request();
        this.errors.push({
          url: request.url(),
          status: response.status(),
          statusText: response.statusText(),
          timestamp: new Date(),
          method: request.method(),
        });
      }
    });

    this.page.on('requestfailed', (request: Request) => {
      this.errors.push({
        url: request.url(),
        status: 0,
        statusText: request.failure()?.errorText || 'Unknown error',
        timestamp: new Date(),
        method: request.method(),
      });
    });
  }

  getErrors(): NetworkError[] {
    return [...this.errors];
  }

  get502Errors(): NetworkError[] {
    return this.errors.filter((e) => e.status === 502);
  }

  get5xxErrors(): NetworkError[] {
    return this.errors.filter((e) => e.status >= 500 && e.status < 600);
  }

  clear(): void {
    this.errors = [];
  }

  printSummary(): void {
    if (this.errors.length === 0) {
      console.log('No network errors captured');
      return;
    }

    console.log(`\n=== Network Error Summary (${this.errors.length} errors) ===\n`);

    for (const error of this.errors) {
      console.log(`[${error.method}] ${error.url}`);
      console.log(`  Status: ${error.status} ${error.statusText}`);
      console.log(`  Time: ${error.timestamp.toISOString()}`);
      console.log('');
    }
  }
}
