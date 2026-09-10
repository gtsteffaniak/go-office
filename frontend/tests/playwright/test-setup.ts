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
    /dictionaries\/en_US/i.test(errorText)
  );
}

function isHarmlessFailedResponse(url: string, status: number): boolean {
  if (status !== 404) {
    return false;
  }
  return /\/themes\.json(?:\?|$)/.test(url) || /\/dictionaries\//.test(url);
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
      page.on("pageerror", (err) => {
        pageErrors.push(err.stack ?? err.message);
      });
      page.on("console", (msg) => {
        if (msg.type() !== "error" || isHarmlessConsoleError(msg.text())) {
          return;
        }
        consoleErrors.push(msg.text());
      });
      await use();
      if (testInfo.status !== testInfo.expectedStatus) {
        if (pageErrors.length > 0) {
          console.error("\n=== browser page errors ===");
          pageErrors.forEach((err, i) => {
            console.error(`\n[pageerror ${i + 1}]\n${err}\n---`);
          });
        }
        if (consoleErrors.length > 0) {
          console.error("\n=== browser console errors ===");
          consoleErrors.forEach((err, i) => {
            console.error(`\n[console.error ${i + 1}]\n${err}\n---`);
          });
        }
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
