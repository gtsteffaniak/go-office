import { test as base, expect } from "@playwright/test";

type Fixtures = {
  collectPageErrors: void;
};

export const test = base.extend<Fixtures>({
  collectPageErrors: [
    async ({ page }, use) => {
      const errors: string[] = [];
      page.on("pageerror", (err) => errors.push(err.message));
      page.on("console", (msg) => {
        if (msg.type() === "error") {
          errors.push(msg.text());
        }
      });
      await use();
      const fatal = errors.filter((e) => /fonts are not loaded/i.test(e));
      expect(fatal, `browser errors: ${fatal.join("; ")}`).toHaveLength(0);
    },
    { auto: true },
  ],
});

export { expect };
