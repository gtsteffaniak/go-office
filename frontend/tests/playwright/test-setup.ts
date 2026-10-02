import { test as base, expect, type Page } from "@playwright/test";

type Fixtures = {
  collectPageErrors: void;
  checkForErrors: (expectedConsoleErrors?: number, expectedApiErrors?: number) => void;
};

function isHarmlessConsoleError(errorText: string): boolean {
  return (
    // Firefox logs background lazy-load font failures; core icons still render.
    /downloadable font: download failed.*material-symbols\.woff2/i.test(errorText) ||
    // ONLYOFFICE may warn when optional spellcheck dictionaries are absent in demo.
    /dictionaries\/en_US/i.test(errorText) ||
    // Coauthoring falls back to long-polling when websocket upgrade is unavailable.
    /WebSocket connection to 'ws:.*\/doc\/.*\/c\/\?.*transport=websocket' failed.*501/i.test(
      errorText,
    ) ||
    /Error during WebSocket handshake: Unexpected response code: 501/i.test(errorText) ||
    // Generic browser network summary for expected 404s (themes, dictionaries, favicon).
    /^Failed to load resource: the server responded with a status of 404/i.test(errorText) ||
    // Slide themes bundle may be absent in trimmed asset trees; stub is served when present.
    /sdkjs\/slide\/themes\/themes\.js/i.test(errorText)
  );
}

function isHarmlessFailedResponse(url: string, status: number): boolean {
  // Redirects. These are not failures and carry no diagnostic value:
  //   301  editor shell directory requested without a trailing slash (".../main" -> ".../main/")
  //   307/308  cache paths that historically carried a doubled slash ("//cache/files/...")
  // The double-slash case is fixed server-side (see fileURL in internal/ws/opener.go); the
  // filter stays so a stale deployment does not flood the report with redirect noise.
  if (status === 301 || status === 302 || status === 307 || status === 308) {
    return true;
  }
  if (status === 404) {
    return (
      /\/themes\.json(?:\?|$)/.test(url) ||
      /\/dictionaries\//.test(url) ||
      /\/favicon\.ico(?:\?|$)/.test(url) ||
      /\/sdkjs\/slide\/themes\//.test(url)
    );
  }
  if (status === 501 && /\/doc\/.*\/c\/\?.*transport=websocket/.test(url)) {
    return true;
  }
  return false;
}

export function setupErrorTracking(page: Page) {
  const consoleErrors: string[] = [];
  const failedResponses: { url: string; status: number }[] = [];

  page.on("console", async (message) => {
    if (message.type() !== "error") {
      return;
    }

    const errorText = message.text();
    const args = message.args();
    let detailedError = errorText;

    if (args.length > 0) {
      try {
        const firstArg = await args[0].jsonValue().catch(() => null);
        if (firstArg && typeof firstArg === "object") {
          const err = firstArg as { stack?: string; message?: string; name?: string };
          if (err.stack) {
            detailedError = err.stack;
          } else if (err.message) {
            detailedError = `${err.name || "Error"}: ${err.message}`;
          }
        }
      } catch {
        detailedError = errorText;
      }
    }

    if (isHarmlessConsoleError(detailedError)) {
      return;
    }

    consoleErrors.push(detailedError);
  });

  page.on("response", (response) => {
    const status = response.status();
    const url = response.url();
    if (status === 304 || response.ok() || isHarmlessFailedResponse(url, status)) {
      return;
    }
    failedResponses.push({ url, status });
  });

  return {
    checkForErrors: (expectedConsoleErrors = 0, expectedApiErrors = 0) => {
      if (consoleErrors.length !== expectedConsoleErrors) {
        console.error(
          `\n=== Unexpected Console Errors (Expected: ${expectedConsoleErrors}, Got: ${consoleErrors.length}) ===`,
        );
        consoleErrors.forEach((error, index) => {
          console.error(`\nError ${index + 1}:`);
          console.error(error);
          console.error("---");
        });
        console.error("=== End Console Errors ===\n");
      }

      if (failedResponses.length !== expectedApiErrors) {
        console.error(
          `\n=== Unexpected Failed API Calls (Expected: ${expectedApiErrors}, Got: ${failedResponses.length}) ===`,
        );
        failedResponses.forEach((response, index) => {
          console.error(`\nFailed Request ${index + 1}: ${response.status} - ${response.url}`);
        });
        console.error("=== End Failed API Calls ===\n");
      }

      expect(consoleErrors).toHaveLength(expectedConsoleErrors);
      expect(failedResponses).toHaveLength(expectedApiErrors);
    },
  };
}

export const test = base.extend<Fixtures>({
  collectPageErrors: [
    async ({ page }, use, testInfo) => {
      const pageErrors: string[] = [];
      const consoleErrors: string[] = [];
      // Warnings matter for diagnosis: sdkjs reports "apiReady=false" style failures and
      // resource problems as console.warn, and Playwright reports those with type
      // "warning" (not "warn"), so an errors-only filter silently drops them.
      const consoleWarnings: string[] = [];
      const failedRequests: string[] = [];

      page.on("pageerror", (err) => {
        pageErrors.push(err.stack ?? err.message);
      });
      page.on("console", (msg) => {
        const type = msg.type();
        const text = msg.text();
        if (type === "error") {
          if (!isHarmlessConsoleError(text)) {
            consoleErrors.push(text);
          }
          return;
        }
        if (type === "warning") {
          // Route through the same noise filter so known-benign warnings stay quiet.
          if (!isHarmlessConsoleError(text)) {
            consoleWarnings.push(text);
          }
        }
      });
      // Non-OK responses are often the actual cause when the editor never becomes ready
      // (a missing asset does not always produce a console error).
      page.on("response", (res) => {
        const status = res.status();
        if (status === 304 || res.ok() || isHarmlessFailedResponse(res.url(), status)) {
          return;
        }
        failedRequests.push(`${status} ${res.url()}`);
      });

      await use();

      const failed = testInfo.status !== testInfo.expectedStatus;
      const report = () => {
        if (pageErrors.length > 0) {
          console.error("\n=== browser page errors ===");
          pageErrors.forEach((err, i) => console.error(`\n[pageerror ${i + 1}]\n${err}\n---`));
        }
        if (consoleErrors.length > 0) {
          console.error("\n=== browser console errors ===");
          consoleErrors.forEach((err, i) => console.error(`\n[console.error ${i + 1}]\n${err}\n---`));
        }
        if (consoleWarnings.length > 0) {
          console.error("\n=== browser console warnings ===");
          consoleWarnings.forEach((w, i) => console.error(`\n[console.warning ${i + 1}]\n${w}\n---`));
        }
        if (failedRequests.length > 0) {
          console.error("\n=== failed responses ===");
          failedRequests.forEach((r, i) => console.error(`\n[response ${i + 1}] ${r}\n---`));
        }
      };

      if (failed) {
        report();
      }

      // Attach to the test report as well as stdout: CI keeps the artifact even when the
      // log is truncated, and the summary stays readable without scrolling the whole run.
      const summary = [
        pageErrors.length > 0 ? `pageErrors: ${pageErrors.length}` : "",
        consoleErrors.length > 0 ? `consoleErrors: ${consoleErrors.length}` : "",
        consoleWarnings.length > 0 ? `consoleWarnings: ${consoleWarnings.length}` : "",
        failedRequests.length > 0 ? `failedResponses: ${failedRequests.length}` : "",
      ]
        .filter(Boolean)
        .join(", ");
      if (summary) {
        const body = [
          "=== page errors ===",
          ...pageErrors,
          "=== console errors ===",
          ...consoleErrors,
          "=== console warnings ===",
          ...consoleWarnings,
          "=== failed responses ===",
          ...failedRequests,
        ].join("\n");
        await testInfo.attach("browser-console", { body, contentType: "text/plain" });
      }

      const fatal = [...pageErrors, ...consoleErrors].filter((e) =>
        /fonts are not loaded/i.test(e),
      );
      expect(fatal, `browser errors: ${fatal.join("; ")}`).toHaveLength(0);
    },
    { auto: true },
  ],
  checkForErrors: async ({ page }, use) => {
    const { checkForErrors } = setupErrorTracking(page);
    await use(checkForErrors);
  },
});

export { expect };
